package galwar

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// testPort builds a deterministic trading port in the given sector.
func testPort(u *UniverseType, sector int, name string) *Port {
	p := &Port{
		ObjectBase: ObjectBase{Name: name, Sector: sector},
	}
	p.Inventory = append(p.Inventory,
		&Commodity{Name: ORE, Quantity: 100, BuyPrice: 8, SellPrice: 5, Sell: true},
		&Commodity{Name: ORGANICS, Quantity: 100, BuyPrice: 14, SellPrice: 10, Sell: false},
	)
	u.Ports.Ports = append(u.Ports.Ports, p)
	return p
}

func TestGameErrorMessage(t *testing.T) {
	err := NewGameError(ErrNotEnoughMoney, "You don't have enough credits.")
	if !strings.Contains(err.Error(), "You don't have enough credits.") {
		t.Errorf("Error() = %q; want it to contain the message", err.Error())
	}
	if strings.Contains(err.Error(), "%!s") {
		t.Errorf("Error() = %q; formats a func value instead of the message", err.Error())
	}
}

func TestSectorGetName(t *testing.T) {
	s := Sector{Number: 65}
	if got := s.GetName(); got != "Sector 65" {
		t.Errorf("GetName() = %q; want %q", got, "Sector 65")
	}
}

func TestTradeBuyValidatesBeforeMutating(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com")
	port := testPort(u, 50, "TestPort")

	player.Money = 10 // cannot afford 100 ore at 5 each
	before := port.GetQuantity(ORE)
	if err := TradeBuy(ORE, port, player, 100); err == nil {
		t.Fatalf("expected error buying beyond means")
	}
	if port.GetQuantity(ORE) != before {
		t.Errorf("port stock changed on failed buy: %d -> %d", before, port.GetQuantity(ORE))
	}
	if player.GetQuantity(ORE) != 0 {
		t.Errorf("player received goods on failed buy")
	}
	if player.Money != 10 {
		t.Errorf("player charged on failed buy")
	}
}

func TestTradeBuyEnforcesHolds(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com") // 25 holds
	port := testPort(u, 50, "TestPort")

	before := port.GetQuantity(ORE)
	err := TradeBuy(ORE, port, player, 30) // 30 > 25 holds, money is plenty
	if err == nil {
		t.Fatalf("expected ErrNotEnoughHolds")
	}
	if ge, ok := err.(*GameError); !ok || ge.code != ErrNotEnoughHolds {
		t.Errorf("got %v; want ErrNotEnoughHolds", err)
	}
	if port.GetQuantity(ORE) != before {
		t.Errorf("port stock changed on failed buy")
	}

	if err := TradeBuy(ORE, port, player, 25); err != nil {
		t.Fatalf("buy within holds failed: %v", err)
	}
	if player.GetFreeHolds() != 0 {
		t.Errorf("free holds = %d; want 0", player.GetFreeHolds())
	}
}

func TestTradeSellValidatesBeforeMutating(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com")
	port := testPort(u, 50, "TestPort")

	before := port.GetQuantity(ORGANICS)
	if err := TradeSell(ORGANICS, port, player, 10); err == nil {
		t.Fatalf("expected error selling cargo the player doesn't have")
	}
	if port.GetQuantity(ORGANICS) != before {
		t.Errorf("port buy-capacity changed on failed sell: %d -> %d", before, port.GetQuantity(ORGANICS))
	}
	if player.Money != 35000 {
		t.Errorf("player money changed on failed sell")
	}
}

func TestNegativeQuantityGuards(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	player := u.NewPlayer("Test", "t@example.com")
	port := testPort(u, 50, "TestPort")
	player.MoveTo(50)

	if err := TradeBuy(ORE, port, player, -5); err == nil {
		t.Errorf("TradeBuy accepted negative quantity")
	}
	if err := TradeSell(ORGANICS, port, player, -5); err == nil {
		t.Errorf("TradeSell accepted negative quantity")
	}

	moneyBefore := player.Money
	fighters := &Commodity{Name: FIGHTERS, SellPrice: 98, Sell: true}
	if err := TradeBuyNoLimit(fighters, player, -1000); err == nil {
		t.Errorf("TradeBuyNoLimit accepted negative quantity")
	}
	if player.Money != moneyBefore {
		t.Errorf("negative purchase changed money: %d -> %d", moneyBefore, player.Money)
	}

	fightersBefore := player.GetQuantity(FIGHTERS)
	if err := u.AdjustBattlegroup(player, 50, FIGHTERS, -5); err == nil {
		t.Errorf("AdjustBattlegroup accepted negative amount")
	}
	if player.GetQuantity(FIGHTERS) != fightersBefore {
		t.Errorf("negative battlegroup adjust changed player fighters")
	}
}

func TestBattlegroupPlaceAndTake(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	player := u.NewPlayer("Test", "t@example.com")
	player.MoveTo(50)

	if err := u.AdjustBattlegroup(player, 5, FIGHTERS, 10); err == nil {
		t.Errorf("expected fed restriction in sector 5")
	}

	if err := u.AdjustBattlegroup(player, 50, FIGHTERS, 150); err != nil {
		t.Fatalf("place fighters: %v", err)
	}
	if player.GetQuantity(FIGHTERS) != 50 {
		t.Errorf("player fighters = %d; want 50", player.GetQuantity(FIGHTERS))
	}
	bg, err := u.GetBattlegroup(player, 50, false)
	if err != nil || bg == nil {
		t.Fatalf("battlegroup not found: %v", err)
	}
	if bg.GetQuantity(FIGHTERS) != 150 {
		t.Errorf("bg fighters = %d; want 150", bg.GetQuantity(FIGHTERS))
	}

	// take them all back; empty battlegroup should be removed
	if err := u.AdjustBattlegroup(player, 50, FIGHTERS, 0); err != nil {
		t.Fatalf("take fighters: %v", err)
	}
	if player.GetQuantity(FIGHTERS) != 200 {
		t.Errorf("player fighters = %d; want 200", player.GetQuantity(FIGHTERS))
	}
	if len(u.Battlegroups.Battlegroups) != 0 {
		t.Errorf("empty battlegroup not removed")
	}
}

func TestGenesisAndPlanetTransfers(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	player := u.NewPlayer("Test", "t@example.com")
	player.MoveTo(50)

	if err := u.UseGenesisDevice(player, 50, "Testworld"); err == nil {
		t.Errorf("genesis without a device should fail")
	}
	player.SetQuantity(GENESIS, 2)

	if err := u.UseGenesisDevice(player, 5, "FedWorld"); err == nil {
		t.Errorf("genesis in sector 5 should be fed-restricted")
	}
	if err := u.UseGenesisDevice(player, 50, "Testworld"); err != nil {
		t.Fatalf("genesis failed: %v", err)
	}
	if err := u.UseGenesisDevice(player, 50, "Second"); err == nil {
		t.Errorf("second planet in one sector should be refused")
	}

	if err := u.TransferSet(player, 50, FIGHTERS, -5); err == nil {
		t.Errorf("TransferSet accepted negative amount")
	}
	if err := u.TransferOut(player, 50, ORE, -5); err == nil {
		t.Errorf("TransferOut accepted negative amount")
	}

	// planet starts with 10 ore; take 5
	if err := u.TransferOut(player, 50, ORE, 5); err != nil {
		t.Fatalf("TransferOut failed: %v", err)
	}
	if player.GetQuantity(ORE) != 5 {
		t.Errorf("player ore = %d; want 5", player.GetQuantity(ORE))
	}
}

func TestMovePlayer(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	player := u.NewPlayer("Test", "t@example.com")

	if _, err := u.MovePlayer(player, 99999); err == nil {
		t.Errorf("move to nonexistent sector allowed")
	}

	warps := u.Sectors[1].GetWarps()
	if len(warps) == 0 {
		t.Fatalf("sector 1 has no warps")
	}
	dest := warps[0]
	if _, err := u.MovePlayer(player, dest); err != nil {
		t.Fatalf("move 1->%d failed: %v", dest, err)
	}
	if player.Sector != dest {
		t.Errorf("player sector = %d; want %d", player.Sector, dest)
	}

	// find a sector the new location does not link to and confirm it's refused
	for bad := 1; bad <= 100; bad++ {
		if bad != dest && !u.Sectors[dest].HasWarp(bad) {
			if _, err := u.MovePlayer(player, bad); err == nil {
				t.Errorf("move to non-adjacent sector %d allowed", bad)
			}
			break
		}
	}
}

func TestGenerateConnectivity(t *testing.T) {
	u := NewUniverse()
	u.Generate(200)
	for s := 1; s <= 200; s++ {
		if path := u.ShortestPathTo(1, s); path == nil {
			t.Errorf("sector %d unreachable from sector 1", s)
		}
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	player := u.NewPlayer("Test", "t@example.com")
	player.MoveTo(50)
	player.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(player, 50, "Testworld"); err != nil {
		t.Fatalf("genesis failed: %v", err)
	}

	fn := filepath.Join(t.TempDir(), "universe.yaml")
	u.SetFilename(fn)
	if err := u.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	u2 := NewUniverse()
	u2.SetFilename(fn)
	if err := u2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(u2.Sectors) != len(u.Sectors) {
		t.Errorf("sectors: %d != %d", len(u2.Sectors), len(u.Sectors))
	}
	if len(u2.Ports.Ports) != len(u.Ports.Ports) {
		t.Errorf("ports: %d != %d", len(u2.Ports.Ports), len(u.Ports.Ports))
	}
	p2 := u2.Players.GetByEmail("t@example.com")
	if p2 == nil {
		t.Fatalf("player missing after load")
	}
	if p2.Sector != 50 {
		t.Errorf("player sector = %d; want 50", p2.Sector)
	}
	if len(u2.Planets.Planets) != 1 {
		t.Fatalf("planet missing after load")
	}
	// wire() check: the planet must resolve its owner through the new universe
	if got := u2.Planets.Planets[0].GetOwnerPlayer().GetName(); got != "Test" {
		t.Errorf("planet owner = %q; want %q (wire() not applied?)", got, "Test")
	}
}

func TestActorPropagatesPanics(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com")
	u.Start()

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("panic in a command was not re-raised on the caller")
			}
		}()
		u.Do(func() {
			panic("boom")
		})
	}()

	// the actor must survive a panicking command
	var money int
	u.Do(func() { money = player.Money })
	if money != 35000 {
		t.Errorf("actor dead or state wrong after panic: money = %d", money)
	}

	// DoErr must not return nil on a panic; it must panic too
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("DoErr swallowed a panic and would have returned nil")
			}
		}()
		_ = u.DoErr(func() error {
			panic("boom")
		})
	}()
}

func TestValidateRejectsBadData(t *testing.T) {
	// out-of-range warp
	u := NewUniverse()
	u.Generate(50)
	u.Sectors[10].Warps = append(u.Sectors[10].Warps, 9999)
	if err := u.validate(); err == nil {
		t.Errorf("validate accepted a warp to a nonexistent sector")
	}

	// nil commodity entry
	u2 := NewUniverse()
	u2.Generate(50)
	player := u2.NewPlayer("Test", "t@example.com")
	player.Inventory = append(player.Inventory, nil)
	if err := u2.validate(); err == nil {
		t.Errorf("validate accepted a nil commodity entry")
	}

	// unknown commodity name
	u3 := NewUniverse()
	u3.Generate(50)
	player3 := u3.NewPlayer("Test", "t@example.com")
	player3.Inventory = append(player3.Inventory, &Commodity{Name: "Flux Capacitors", Quantity: 1})
	if err := u3.validate(); err == nil {
		t.Errorf("validate accepted an unknown commodity")
	}
}

func TestActorSerializesCommands(t *testing.T) {
	u := NewUniverse()
	player := u.NewPlayer("Test", "t@example.com")
	u.Start()

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				u.Do(func() {
					player.Money++
				})
			}
		}()
	}
	wg.Wait()

	var money int
	u.Do(func() { money = player.Money })
	if money != 35000+2000 {
		t.Errorf("money = %d; want %d (commands not serialized?)", money, 35000+2000)
	}
}
