package galwar

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestTurnEconomy(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	player := u.NewPlayer("Test", "t@example.com")

	if got := player.GetQuantity(TURNS); got != 250 {
		t.Fatalf("starting turns = %d; want 250", got)
	}

	dest := u.Sectors[1].GetWarps()[0]
	if _, err := u.MovePlayer(player, dest); err != nil {
		t.Fatalf("move: %v", err)
	}
	if got := player.GetQuantity(TURNS); got != 249 {
		t.Errorf("turns after move = %d; want 249", got)
	}

	// docking a trading port costs a turn; Sol is free
	port := testPort(u, player.Sector, "TurnPort")
	if err := u.Dock(player, port); err != nil {
		t.Fatalf("dock: %v", err)
	}
	if got := player.GetQuantity(TURNS); got != 248 {
		t.Errorf("turns after dock = %d; want 248", got)
	}
	sol := u.Ports.Ports[0]
	if sol.Goods != Sol {
		t.Fatalf("port 0 is not Sol")
	}
	if err := u.Dock(player, sol); err != nil {
		t.Fatalf("dock sol: %v", err)
	}
	if got := player.GetQuantity(TURNS); got != 248 {
		t.Errorf("Sol docking charged a turn: %d", got)
	}

	// at zero turns, movement and docking are refused
	player.SetQuantity(TURNS, 0)
	if _, err := u.MovePlayer(player, dest); err == nil {
		t.Errorf("move allowed with no turns")
	}
	if err := u.Dock(player, port); err == nil {
		t.Errorf("dock allowed with no turns")
	}

	// Sol sells turns
	if sol.GetCommodity(TURNS) == nil {
		t.Errorf("Sol does not sell turns")
	}
}

func TestScarcityPricing(t *testing.T) {
	c := &Commodity{Name: ORE, Prod: 100, Quantity: 1000, BuyPrice: 8, SellPrice: 5, Sell: true}

	// full stock: flat prices
	if got := c.EffectiveSellPrice(); got != 5 {
		t.Errorf("full-stock sell price = %v; want 5", got)
	}
	if got := c.EffectiveBuyPrice(); got != 8 {
		t.Errorf("full-stock buy price = %v; want 8", got)
	}

	// half depleted: sell +2.5%, buy -2.5%
	c.Quantity = 500
	if got := c.EffectiveSellPrice(); math.Abs(got-5*1.025) > 1e-9 {
		t.Errorf("half-depleted sell price = %v; want %v", got, 5*1.025)
	}
	if got := c.EffectiveBuyPrice(); math.Abs(got-8*0.975) > 1e-9 {
		t.Errorf("half-depleted buy price = %v; want %v", got, 8*0.975)
	}

	// empty: full 5% swing
	c.Quantity = 0
	if got := c.EffectiveSellPrice(); math.Abs(got-5*1.05) > 1e-9 {
		t.Errorf("empty sell price = %v; want %v", got, 5*1.05)
	}

	// no production rate (ship equipment, Sol goods): never swings
	flat := &Commodity{Name: FIGHTERS, SellPrice: 98, Sell: true}
	if got := flat.EffectiveSellPrice(); got != 98 {
		t.Errorf("prod-less commodity price swung: %v", got)
	}
}

func TestVolumeScaling(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com") // 25 holds -> factor 1

	if got := ScaleFactor(player); got != 1 {
		t.Errorf("factor at 25 holds = %v; want 1", got)
	}
	player.SetQuantity(HOLDS, 500) // -> factor 10
	if got := ScaleFactor(player); got != 10 {
		t.Errorf("factor at 500 holds = %v; want 10", got)
	}
	player.SetQuantity(HOLDS, 50000) // clamped at 275
	if got := ScaleFactor(player); got != 275 {
		t.Errorf("factor at 50000 holds = %v; want 275", got)
	}

	// a 500-hold ship sees 10x the stock and consumes port units at 1/10
	player.SetQuantity(HOLDS, 500)
	player.Money = 1000000
	port := testPort(u, 50, "ScalePort")
	ore := port.GetCommodity(ORE) // 100 units, factor 10 -> 1000 quoted

	if got := ScaleUp(player, ore.Quantity); got != 1000 {
		t.Errorf("quoted stock = %d; want 1000", got)
	}
	if err := TradeBuy(ORE, port, player, 500); err != nil {
		t.Fatalf("scaled buy: %v", err)
	}
	if got := ore.Quantity; got != 50 { // 500 player-units / 10
		t.Errorf("port stock after scaled buy = %d; want 50", got)
	}
	if got := player.GetQuantity(ORE); got != 500 {
		t.Errorf("player ore = %d; want 500", got)
	}

	// tiny trades still consume at least one port unit (anti-exploit)
	player.SetQuantity(ORE, 0) // free the holds again
	if err := TradeBuy(ORE, port, player, 0); err != nil {
		t.Fatalf("zero buy: %v", err)
	}
	if got := ore.Quantity; got != 50 {
		t.Errorf("zero-quantity buy consumed stock: %d", got)
	}
	if err := TradeBuy(ORE, port, player, 1); err != nil {
		t.Fatalf("tiny buy: %v", err)
	}
	if got := ore.Quantity; got != 49 {
		t.Errorf("tiny buy consumed %d units; want 1", 50-got)
	}
}

func TestScaleDownExactInverse(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com")

	// fractional factors must not under-consume: scaleDown returns the
	// smallest port delta whose ScaleUp covers the traded amount
	for _, holds := range []int{25, 50, 60, 75, 500, 1234, 50000} {
		player.SetQuantity(HOLDS, holds)
		for _, w := range []int{1, 2, 3, 7, 50, 499, 1000} {
			d := scaleDown(player, w)
			if ScaleUp(player, d) < w {
				t.Errorf("holds=%d w=%d: scaleDown=%d under-covers (ScaleUp=%d)", holds, w, d, ScaleUp(player, d))
			}
			if d > 1 && ScaleUp(player, d-1) >= w {
				t.Errorf("holds=%d w=%d: scaleDown=%d not minimal", holds, w, d)
			}
		}
		if got := scaleDown(player, 0); got != 0 {
			t.Errorf("holds=%d: scaleDown(0) = %d; want 0", holds, got)
		}
	}
}

func TestDaemonLifecycle(t *testing.T) {
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()

	// Stop without Start must return immediately; double Start and double
	// Stop must be harmless
	m := NewMaintenanceDaemon(u)
	m.Stop()
	m.Start()
	m.Start()
	m.Stop()
	m.Stop()

	store, err := OpenStore(filepath.Join(t.TempDir(), "galwar.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	p := NewPersister(u, store)
	p.Stop()
	p.Start()
	p.Start()
	p.Stop()
	p.Stop()
}

func TestRestock(t *testing.T) {
	now := time.Now().Unix()
	c := &Commodity{Name: ORE, Prod: 100, Quantity: 0, BuyPrice: 8, SellPrice: 5}

	// first call initializes the clock, credits nothing
	c.Restock(now)
	if c.Quantity != 0 {
		t.Errorf("initialization credited stock: %d", c.Quantity)
	}

	// half a day at +2*prod/day = +prod
	c.Restock(now + 43200)
	if c.Quantity != 100 {
		t.Errorf("half-day restock = %d; want 100", c.Quantity)
	}

	// a week later: capped at 10*prod
	c.Restock(now + 7*86400)
	if c.Quantity != 1000 {
		t.Errorf("capped restock = %d; want 1000", c.Quantity)
	}

	// rapid tiny polls must not starve the accrual: 200 units/day is one
	// unit per 432s; polling every 100s must still credit ~200/day
	c2 := &Commodity{Name: ORE, Prod: 100, Quantity: 0}
	c2.Restock(now)
	credited := 0
	for tick := int64(100); tick <= 86400; tick += 100 {
		before := c2.Quantity
		c2.Restock(now + tick)
		credited += c2.Quantity - before
	}
	if credited < 199 || credited > 200 {
		t.Errorf("day of rapid polling credited %d units; want ~200", credited)
	}

	// no production rate: never restocks
	flat := &Commodity{Name: FIGHTERS, Quantity: 5}
	flat.Restock(now)
	flat.Restock(now + 86400)
	if flat.Quantity != 5 {
		t.Errorf("prod-less commodity restocked: %d", flat.Quantity)
	}
}

func TestPlanetProduction(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	player := u.NewPlayer("Test", "t@example.com")
	player.MoveTo(50)
	player.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(player, 50, "Farm"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]

	// genesis seed: 10 stock, prod 1 for ore/org/eqp (PLANET.PAS:221-234)
	if got := planet.GetCommodity(ORE).Prod; got != 1 {
		t.Fatalf("seed ore prod = %d; want 1", got)
	}

	// day 1: stock 10+1=11 > prod*10=10, so prod ramps by (11-10)/10 = 0
	growPlanet(planet, 1000, false)
	if got := planet.GetQuantity(ORE); got != 11 {
		t.Errorf("day-1 ore = %d; want 11", got)
	}

	// hoard cargo on the planet and production compounds:
	// prod 1, stock 210 -> prod += (210-10)/10 = 20 -> prod 21
	planet.SetQuantity(ORE, 209) // becomes 210 after +prod
	growPlanet(planet, 1000, false)
	if got := planet.GetCommodity(ORE).Prod; got != 21 {
		t.Errorf("ramped ore prod = %d; want 21", got)
	}

	// derived weapon production: t = ore/4 + org/2 + eqp
	ore := planet.GetCommodity(ORE).Prod
	org := planet.GetCommodity(ORGANICS).Prod
	eqp := planet.GetCommodity(EQUIPMENT).Prod
	want := int(math.Round(float64(ore)/4+float64(org)/2+float64(eqp))) / 5
	if got := planet.GetCommodity(FIGHTERS).Prod; got != want {
		t.Errorf("fighter prod = %d; want %d", got, want)
	}
}

func TestDailyMaintenance(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	player := u.NewPlayer("Test", "t@example.com")
	player.SetQuantity(TURNS, 3)

	now := time.Now()
	if !u.RunDailyMaintenance(now) {
		t.Fatalf("maintenance did not run on a fresh universe")
	}
	if got := player.GetQuantity(TURNS); got != 250 {
		t.Errorf("turns after maintenance = %d; want 250", got)
	}

	// second run the same day is a no-op
	player.SetQuantity(TURNS, 7)
	if u.RunDailyMaintenance(now) {
		t.Errorf("maintenance ran twice in one day")
	}
	if got := player.GetQuantity(TURNS); got != 7 {
		t.Errorf("no-op maintenance changed turns: %d", got)
	}

	// next day it runs again
	if !u.RunDailyMaintenance(now.Add(24 * time.Hour)) {
		t.Errorf("maintenance did not run the next day")
	}
	if got := player.GetQuantity(TURNS); got != 250 {
		t.Errorf("turns after next-day maintenance = %d; want 250", got)
	}
}

func TestUpgradeLegacyData(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	player := u.NewPlayer("Old", "old@example.com")

	// simulate a pre-M3 save: no Turns anywhere, no planet prod
	for i, c := range player.Inventory {
		if c.Name == TURNS {
			player.Inventory = append(player.Inventory[:i], player.Inventory[i+1:]...)
			break
		}
	}
	sol := u.Ports.Ports[0]
	for i, c := range sol.Inventory {
		if c.Name == TURNS {
			sol.Inventory = append(sol.Inventory[:i], sol.Inventory[i+1:]...)
			break
		}
	}
	player.SetQuantity(GENESIS, 1)
	player.MoveTo(50)
	if err := u.UseGenesisDevice(player, 50, "OldWorld"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]
	planet.GetCommodity(ORE).Prod = 0

	u.upgrade()

	if got := player.GetQuantity(TURNS); got != 250 {
		t.Errorf("legacy player turns = %d; want 250", got)
	}
	solTurns := sol.GetCommodity(TURNS)
	if solTurns == nil || !solTurns.Sell {
		t.Errorf("legacy Sol did not learn to sell turns")
	}
	if got := planet.GetCommodity(ORE).Prod; got != 1 {
		t.Errorf("legacy planet ore prod = %d; want 1", got)
	}
}

func TestStoreMigrationV1toV2(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "galwar.db")

	// build a current-schema database with content, then regress it to v1
	u := makeTestUniverse(t)
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.SaveUniverse(u.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}
	store.Close()

	// regress to a true v1 schema: every column added since then must go,
	// or replaying the migration chain would hit duplicate columns
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	for _, ddl := range []string{
		`ALTER TABLE commodities DROP COLUMN last_restock`,
		`ALTER TABLE players DROP COLUMN google_sub`,
		`ALTER TABLE players DROP COLUMN pass_hash`,
		`ALTER TABLE players DROP COLUMN last_seen`,
		`ALTER TABLE players DROP COLUMN times_died`,
		`ALTER TABLE players DROP COLUMN died_at`,
		`ALTER TABLE players DROP COLUMN systems`,
		`ALTER TABLE players DROP COLUMN banned`,
		`ALTER TABLE players DROP COLUMN expired`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("regress %q: %v", ddl, err)
		}
	}
	if _, err := db.Exec(`UPDATE meta SET value='1' WHERE key='schema_version'`); err != nil {
		t.Fatalf("regress version: %v", err)
	}
	db.Close()

	// reopening must migrate and load cleanly
	store2, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open v1 db: %v", err)
	}
	defer store2.Close()
	u2 := NewUniverse()
	loaded, err := store2.LoadUniverse(u2)
	if err != nil || !loaded {
		t.Fatalf("load after migration: loaded=%v err=%v", loaded, err)
	}
	if u2.Players.GetByEmail("t@example.com") == nil {
		t.Errorf("player lost in migration")
	}

	// a future schema version must be refused
	db, _ = sql.Open("sqlite", "file:"+dbPath)
	db.Exec(`UPDATE meta SET value='99' WHERE key='schema_version'`)
	db.Close()
	if _, err := OpenStore(dbPath); err == nil {
		t.Errorf("future schema version accepted")
	}
}
