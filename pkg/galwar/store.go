package galwar

import (
	"database/sql"
	"fmt"
	"strconv"

	_ "modernc.org/sqlite" // pure Go, no cgo
)

// Store is the SQLite persistence layer. The in-memory universe is
// authoritative; the store is written behind it (see persister.go) and read
// once at startup. Every SaveUniverse call rewrites the world in a single
// transaction, so the database is always a consistent snapshot.
//
// A full rewrite is a few tens of thousands of rows - fine at the current
// scale and flush rate. If it ever becomes a bottleneck, the upgrade path is
// per-entity dirty tracking, not a schema change.

const schemaVersion = "1"

const storeSchema = `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sectors (number INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS warps (
	from_sector INTEGER NOT NULL,
	to_sector   INTEGER NOT NULL,
	PRIMARY KEY (from_sector, to_sector)
);
CREATE TABLE IF NOT EXISTS players (
	id     TEXT PRIMARY KEY,
	email  TEXT NOT NULL,
	name   TEXT NOT NULL,
	sector INTEGER NOT NULL,
	money  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS ports (
	idx    INTEGER PRIMARY KEY,
	name   TEXT NOT NULL,
	sector INTEGER NOT NULL,
	goods  INTEGER NOT NULL,
	money  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS planets (
	idx    INTEGER PRIMARY KEY,
	name   TEXT NOT NULL,
	sector INTEGER NOT NULL,
	owner  TEXT NOT NULL,
	money  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS battlegroups (
	idx    INTEGER PRIMARY KEY,
	name   TEXT NOT NULL,
	sector INTEGER NOT NULL,
	owner  TEXT NOT NULL,
	money  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS commodities (
	owner_type TEXT NOT NULL,
	owner_id   TEXT NOT NULL,
	pos        INTEGER NOT NULL,
	name       TEXT NOT NULL,
	prod       INTEGER NOT NULL,
	quantity   INTEGER NOT NULL,
	buy_price  REAL NOT NULL,
	sell_price REAL NOT NULL,
	sell       INTEGER NOT NULL,
	PRIMARY KEY (owner_type, owner_id, pos)
);
`

type Store struct {
	db *sql.DB
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func OpenStore(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// one process, one writer: a single connection avoids SQLITE_BUSY games
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(storeSchema); err != nil {
		db.Close()
		return nil, err
	}

	var v string
	err = db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&v)
	switch {
	case err == sql.ErrNoRows:
		if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('schema_version', ?)`, schemaVersion); err != nil {
			db.Close()
			return nil, err
		}
	case err != nil:
		db.Close()
		return nil, err
	case v != schemaVersion:
		db.Close()
		return nil, fmt.Errorf("database %s has schema version %s; this build supports %s", path, v, schemaVersion)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// SaveUniverse rewrites the entire universe in one transaction.
func (s *Store) SaveUniverse(snap *Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range []string{"sectors", "warps", "players", "ports", "planets", "battlegroups", "commodities", "config"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}

	insSector, err := tx.Prepare(`INSERT INTO sectors (number) VALUES (?)`)
	if err != nil {
		return err
	}
	for _, n := range snap.sectors {
		if _, err := insSector.Exec(n); err != nil {
			return err
		}
	}

	insWarp, err := tx.Prepare(`INSERT INTO warps (from_sector, to_sector) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	for _, w := range snap.warps {
		if _, err := insWarp.Exec(w[0], w[1]); err != nil {
			return err
		}
	}

	insPlayer, err := tx.Prepare(`INSERT INTO players (id, email, name, sector, money) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	for _, p := range snap.players {
		if _, err := insPlayer.Exec(p.id, p.email, p.name, p.sector, p.money); err != nil {
			return err
		}
	}

	insPort, err := tx.Prepare(`INSERT INTO ports (idx, name, sector, goods, money) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	for _, p := range snap.ports {
		if _, err := insPort.Exec(p.idx, p.name, p.sector, p.goods, p.money); err != nil {
			return err
		}
	}

	insPlanet, err := tx.Prepare(`INSERT INTO planets (idx, name, sector, owner, money) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	for _, p := range snap.planets {
		if _, err := insPlanet.Exec(p.idx, p.name, p.sector, p.owner, p.money); err != nil {
			return err
		}
	}

	insBg, err := tx.Prepare(`INSERT INTO battlegroups (idx, name, sector, owner, money) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	for _, b := range snap.battlegroups {
		if _, err := insBg.Exec(b.idx, b.name, b.sector, b.owner, b.money); err != nil {
			return err
		}
	}

	insCommodity, err := tx.Prepare(`INSERT INTO commodities (owner_type, owner_id, pos, name, prod, quantity, buy_price, sell_price, sell) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	for _, c := range snap.commodities {
		sell := 0
		if c.sell {
			sell = 1
		}
		if _, err := insCommodity.Exec(c.ownerType, c.ownerID, c.pos, c.name, c.prod, c.quantity, c.buyPrice, c.sellPrice, sell); err != nil {
			return err
		}
	}

	insConfig, err := tx.Prepare(`INSERT INTO config (key, value) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	for k, v := range snap.config {
		if _, err := insConfig.Exec(k, v); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadUniverse populates u from the store. Returns false if the store is
// empty (fresh database). Call before Start.
func (s *Store) LoadUniverse(u *UniverseType) (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sectors`).Scan(&n); err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}

	// commodities, grouped by owner
	type invKey struct{ ownerType, ownerID string }
	inventories := map[invKey][]*Commodity{}
	rows, err := s.db.Query(`SELECT owner_type, owner_id, name, prod, quantity, buy_price, sell_price, sell FROM commodities ORDER BY owner_type, owner_id, pos`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var k invKey
		var c Commodity
		var sell int
		if err := rows.Scan(&k.ownerType, &k.ownerID, &c.Name, &c.Prod, &c.Quantity, &c.BuyPrice, &c.SellPrice, &sell); err != nil {
			rows.Close()
			return false, err
		}
		c.Sell = sell != 0
		cm := c
		inventories[k] = append(inventories[k], &cm)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}
	inv := func(ownerType, ownerID string) []*Commodity {
		return inventories[invKey{ownerType, ownerID}]
	}

	// sectors + warps
	rows, err = s.db.Query(`SELECT number FROM sectors ORDER BY number`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var num int
		if err := rows.Scan(&num); err != nil {
			rows.Close()
			return false, err
		}
		u.Sectors = append(u.Sectors, Sector{Number: num, Warps: []int{}})
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	rows, err = s.db.Query(`SELECT from_sector, to_sector FROM warps ORDER BY from_sector, to_sector`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var from, to int
		if err := rows.Scan(&from, &to); err != nil {
			rows.Close()
			return false, err
		}
		if from < 0 || from >= len(u.Sectors) {
			rows.Close()
			return false, fmt.Errorf("warp from invalid sector %d", from)
		}
		u.Sectors[from].Warps = append(u.Sectors[from].Warps, to)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	// players
	rows, err = s.db.Query(`SELECT id, email, name, sector, money FROM players ORDER BY rowid`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		p := &Player{}
		var id string
		if err := rows.Scan(&id, &p.Email, &p.Name, &p.Sector, &p.Money); err != nil {
			rows.Close()
			return false, err
		}
		p.Id = PlayerId(id)
		p.Inventory = inv("player", id)
		u.Players.Players = append(u.Players.Players, p)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	// ports
	rows, err = s.db.Query(`SELECT idx, name, sector, goods, money FROM ports ORDER BY idx`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		p := &Port{}
		var idx, goods int
		if err := rows.Scan(&idx, &p.Name, &p.Sector, &goods, &p.Money); err != nil {
			rows.Close()
			return false, err
		}
		p.Goods = PortType(goods)
		p.Inventory = inv("port", itoa(idx))
		u.Ports.Ports = append(u.Ports.Ports, p)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	// planets
	rows, err = s.db.Query(`SELECT idx, name, sector, owner, money FROM planets ORDER BY idx`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		p := &Planet{}
		var idx int
		var owner string
		if err := rows.Scan(&idx, &p.Name, &p.Sector, &owner, &p.Money); err != nil {
			rows.Close()
			return false, err
		}
		p.Owner = PlayerId(owner)
		p.Inventory = inv("planet", itoa(idx))
		u.Planets.Planets = append(u.Planets.Planets, p)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	// battlegroups
	rows, err = s.db.Query(`SELECT idx, name, sector, owner, money FROM battlegroups ORDER BY idx`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		b := &Battlegroup{}
		var idx int
		var owner string
		if err := rows.Scan(&idx, &b.Name, &b.Sector, &owner, &b.Money); err != nil {
			rows.Close()
			return false, err
		}
		b.Owner = PlayerId(owner)
		b.Inventory = inv("battlegroup", itoa(idx))
		u.Battlegroups.Battlegroups = append(u.Battlegroups.Battlegroups, b)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	// config
	rows, err = s.db.Query(`SELECT key, value FROM config`)
	if err != nil {
		return false, err
	}
	u.Config = map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return false, err
		}
		u.Config[k] = v
	}
	if err := rows.Close(); err != nil {
		return false, err
	}

	u.wire()
	if err := u.validate(); err != nil {
		return false, err
	}
	return true, nil
}

// Backup writes a consistent copy of the database to path (VACUUM INTO).
// The destination must not already exist.
func (s *Store) Backup(path string) error {
	_, err := s.db.Exec(`VACUUM INTO ?`, path)
	return err
}
