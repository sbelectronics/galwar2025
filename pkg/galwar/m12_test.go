package galwar

import (
	"path/filepath"
	"testing"
	"time"
)

// M12: never-moved visibility, grandfathering, and persistence.

func m12Universe(t *testing.T) *UniverseType {
	t.Helper()
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()
	return u
}

func containsPlayer(objs []ObjectInterface, p *Player) bool {
	for _, obj := range objs {
		if obj == ObjectInterface(p) {
			return true
		}
	}
	return false
}

func TestNeverMovedHiddenFromOthers(t *testing.T) {
	u := m12Universe(t)
	now := time.Now()

	newbie, err := u.RegisterPlayer("Fresh One", "f1@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	observer, err := u.RegisterPlayer("Old Hand", "oh@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	observer.EverMoved = true // a veteran

	// the newbie is invisible to the observer, but sees themselves
	objs := u.GetVisibleObjectsInSector(1, TYPE_PLAYER, observer, now)
	if containsPlayer(objs, newbie) {
		t.Errorf("never-moved player visible to another player")
	}
	if !containsPlayer(u.GetVisibleObjectsInSector(1, TYPE_PLAYER, newbie, now), newbie) {
		t.Errorf("never-moved player cannot see themselves")
	}
	// the veteran is visible to the newbie
	if !containsPlayer(u.GetVisibleObjectsInSector(1, TYPE_PLAYER, newbie, now), observer) {
		t.Errorf("veteran not visible to the newbie")
	}

	// one committed move makes the newbie permanently visible
	dest := u.Sectors[1].Warps[0]
	if _, err := u.MovePlayer(newbie, dest); err != nil {
		t.Fatalf("move: %v", err)
	}
	if !newbie.EverMoved {
		t.Fatalf("EverMoved not set by MovePlayer")
	}
	if !containsPlayer(u.GetVisibleObjectsInSector(dest, TYPE_PLAYER, observer, now), newbie) {
		t.Errorf("player still hidden after their first move")
	}
}

func TestEverMovedGrandfathering(t *testing.T) {
	// an old world (no schema marker): everyone is grandfathered as moved
	u := NewUniverse()
	u.Generate(30)
	vet, err := u.RegisterPlayer("Veteran", "v@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	u.upgrade()
	if !vet.EverMoved {
		t.Errorf("pre-existing player not grandfathered as moved")
	}
	if u.ConfigString("schema_ever_moved", "") != "1" {
		t.Errorf("grandfathering marker not set")
	}

	// a fresh world (marker seeded): a new never-moved player stays hidden
	u2 := m12Universe(t)
	newbie, err := u2.RegisterPlayer("Fresh Two", "f2@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	u2.upgrade()
	if newbie.EverMoved {
		t.Errorf("upgrade() wrongly grandfathered a new player in a fresh world")
	}
}

// TestSchemaV5MigrationGrandfathers builds a v5-era database by hand (no
// ever_moved / bank_balance columns) and reopens it with the current build:
// the migration must add the columns and backfill every existing player as
// moved, with an empty bank account.
func TestSchemaV5MigrationGrandfathers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.db")

	// build the v5 database with the historical schema chain (base tables as
	// they existed, then a v5 version marker and one player row)
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	u := m12Universe(t)
	p, _ := u.RegisterPlayer("Legacy Player", "lp@x.com", "")
	p.EverMoved = false
	if err := store.SaveUniverse(u.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}
	// rewind the database to v5: drop the new columns' data path by rebuilding
	// the players table the way v5 had it, and set the version marker back
	for _, ddl := range []string{
		`CREATE TABLE players_v5 AS SELECT id, email, name, sector, money, google_sub, pass_hash, last_seen, times_died, died_at, systems, banned, expired FROM players`,
		`DROP TABLE players`,
		`ALTER TABLE players_v5 RENAME TO players`,
		`UPDATE meta SET value='5' WHERE key='schema_version'`,
		`DELETE FROM config WHERE key='schema_ever_moved'`,
	} {
		if _, err := store.db.Exec(ddl); err != nil {
			t.Fatalf("rewinding to v5 (%s): %v", ddl, err)
		}
	}
	store.Close()

	// reopen: the v5->v6 migration runs, then LoadUniverse
	store2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen (migration): %v", err)
	}
	defer store2.Close()
	u2 := NewUniverse()
	if ok, err := store2.LoadUniverse(u2); err != nil || !ok {
		t.Fatalf("load after migration: ok=%v err=%v", ok, err)
	}
	p2 := u2.Players.GetById(p.Id)
	if p2 == nil {
		t.Fatalf("player lost in migration")
	}
	if !p2.EverMoved {
		t.Errorf("migrated player not grandfathered as moved")
	}
	if p2.BankBalance != 0 {
		t.Errorf("migrated player has nonzero bank balance: %d", p2.BankBalance)
	}
}

func TestEverMovedAndBankPersist(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "m12.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	u := m12Universe(t)
	mover, _ := u.RegisterPlayer("Mover", "m@x.com", "")
	mover.EverMoved = true
	mover.BankBalance = 12345
	parked, _ := u.RegisterPlayer("Parked", "p@x.com", "")

	if err := store.SaveUniverse(u.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}
	u2 := NewUniverse()
	if ok, err := store.LoadUniverse(u2); err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}

	m2 := u2.Players.GetById(mover.Id)
	p2 := u2.Players.GetById(parked.Id)
	if m2 == nil || p2 == nil {
		t.Fatalf("players did not survive the round trip")
	}
	if !m2.EverMoved || m2.BankBalance != 12345 {
		t.Errorf("mover round trip: EverMoved=%v BankBalance=%d", m2.EverMoved, m2.BankBalance)
	}
	if p2.EverMoved {
		t.Errorf("never-moved flag wrongly true after round trip")
	}
}
