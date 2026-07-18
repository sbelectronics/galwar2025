package galwar

import (
	"strings"
	"testing"
	"time"
)

// M15: Fusion Cell, Mine Deflector, Planetary Scanner.

func deviceUniverse(t *testing.T) (*UniverseType, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(60)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Gadgeteer", "g@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	u.TouchLastSeen(p, time.Now().Unix())
	return u, p
}

func TestFusionCellBanksAndDrawsTurns(t *testing.T) {
	u, p := deviceUniverse(t)
	now := time.Now()
	p.SetQuantity(FUSIONCELL, 1)

	// spend the whole daily allowance down to 40 left, then run maintenance:
	// the 40 unused turns bank
	p.SetQuantity(TURNS, 40)
	if ran := u.RunDailyMaintenance(now); !ran {
		t.Fatalf("maintenance did not run")
	}
	if p.BankedTurns != 40 {
		t.Errorf("banked %d turns, want 40", p.BankedTurns)
	}
	if p.GetQuantity(TURNS) != 250 {
		t.Errorf("daily allowance not reset: %d", p.GetQuantity(TURNS))
	}

	// spend all 250 daily turns, then one more: it comes from the bank
	p.SetQuantity(TURNS, 0)
	if err := u.spendTurn(p); err != nil {
		t.Errorf("spendTurn refused despite banked turns: %v", err)
	}
	if p.BankedTurns != 39 {
		t.Errorf("banked turn not drawn: %d", p.BankedTurns)
	}
}

func TestFusionCellCapAndNoCell(t *testing.T) {
	u, p := deviceUniverse(t)
	now := time.Now()

	// with a cell, banking is capped at a full day's allowance
	p.SetQuantity(FUSIONCELL, 1)
	p.BankedTurns = 240
	p.SetQuantity(TURNS, 100) // would push to 340
	u.RunDailyMaintenance(now)
	if p.BankedTurns != 250 {
		t.Errorf("banked turns not capped at 250: %d", p.BankedTurns)
	}

	// without a cell, the reserve is wiped at maintenance
	u2, p2 := deviceUniverse(t)
	p2.BankedTurns = 100 // e.g. sold/lost the cell
	p2.SetQuantity(TURNS, 50)
	u2.RunDailyMaintenance(now)
	if p2.BankedTurns != 0 {
		t.Errorf("banked turns survived without a Fusion Cell: %d", p2.BankedTurns)
	}
}

func TestFusionCellResetOnDeath(t *testing.T) {
	u, p := deviceUniverse(t)
	now := time.Now()
	p.SetQuantity(FUSIONCELL, 1)
	p.BankedTurns = 100
	u.KillPlayer(p, now.Unix())
	u.ReconstructIfDue(p, now.Add(48*time.Hour))
	if p.BankedTurns != 0 {
		t.Errorf("banked turns survived reconstruction: %d", p.BankedTurns)
	}
	if p.GetQuantity(FUSIONCELL) != 0 {
		t.Errorf("Fusion Cell survived reconstruction: %d", p.GetQuantity(FUSIONCELL))
	}
}

func TestMineDeflectorAbsorbsBlast(t *testing.T) {
	u, p := deviceUniverse(t)
	p.SetQuantity(FIGHTERS, 500)
	p.SetQuantity(HOLDS, 25)
	p.SetQuantity(MINEDEFLECTOR, 1)

	// one deflector absorbs one blast: no losses, deflector consumed
	if !u.absorbMine(p) {
		t.Fatalf("deflector did not absorb")
	}
	if p.GetQuantity(FIGHTERS) != 500 || p.GetQuantity(HOLDS) != 25 {
		t.Errorf("absorbed blast still caused losses")
	}
	if p.GetQuantity(MINEDEFLECTOR) != 0 {
		t.Errorf("deflector not consumed: %d", p.GetQuantity(MINEDEFLECTOR))
	}
	// no more deflectors: next blast lands
	if u.absorbMine(p) {
		t.Errorf("absorbed a blast with no deflector left")
	}
}

func TestMineDeflectorInInvasion(t *testing.T) {
	u, attacker := deviceUniverse(t)
	defender, _ := u.RegisterPlayer("Owner", "o@x.com", "")
	defender.MoveTo(40)
	defender.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(defender, 40, "Fort"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]
	planet.SetQuantity(FIGHTERS, 0) // no garrison: straight to the minefield
	planet.SetQuantity(MINES, 3)

	attacker.MoveTo(40)
	attacker.SetQuantity(FIGHTERS, 100000)
	attacker.SetQuantity(HOLDS, 1000)
	attacker.SetQuantity(MINEDEFLECTOR, 5) // enough to absorb all 3 mines

	report, err := u.InvadePlanet(attacker, 100000)
	if err != nil {
		t.Fatalf("invade: %v", err)
	}
	joined := strings.Join(report, "\n")
	if !strings.Contains(joined, "Mine Deflector absorbs") {
		t.Errorf("deflector message not in invasion report:\n%s", joined)
	}
	if attacker.GetQuantity(MINEDEFLECTOR) != 2 { // 3 absorbed, 2 left
		t.Errorf("deflectors consumed wrong: %d left, want 2", attacker.GetQuantity(MINEDEFLECTOR))
	}
	if planet.Owner != attacker.Id {
		t.Errorf("invasion did not capture the planet")
	}
}

func TestPlanetScannerHelper(t *testing.T) {
	_, p := deviceUniverse(t)
	if p.HasPlanetScanner() {
		t.Errorf("no scanner but HasPlanetScanner true")
	}
	p.SetQuantity(PLANETSCANNER, 1)
	if !p.HasPlanetScanner() {
		t.Errorf("scanner owned but HasPlanetScanner false")
	}
}

func TestNewDevicesValued(t *testing.T) {
	u, p := deviceUniverse(t)
	before := u.PlayerValue(p)
	p.SetQuantity(FUSIONCELL, 1)
	p.SetQuantity(PLANETSCANNER, 1)
	p.SetQuantity(MINEDEFLECTOR, 2)
	got := u.PlayerValue(p)
	want := before + 45000 + 40000 + 2*6000
	if got != want {
		t.Errorf("PlayerValue with new devices = %d, want %d", got, want)
	}
}

func TestNewDevicesSoldAtAmazingDevices(t *testing.T) {
	u, _ := deviceUniverse(t)
	var shop *Port
	for _, port := range u.Ports.Ports {
		if port.Goods == AmazingDevices {
			shop = port
		}
	}
	if shop == nil {
		t.Fatalf("no Amazing Devices port")
	}
	for _, name := range []string{FUSIONCELL, PLANETSCANNER, MINEDEFLECTOR} {
		if c := shop.GetCommodity(name); c == nil || !c.Sell {
			t.Errorf("Amazing Devices does not sell %s", name)
		}
	}
}

func TestBankedTurnsPersist(t *testing.T) {
	u, p := deviceUniverse(t)
	p.BankedTurns = 77
	snap := u.Snapshot()
	if len(snap.players) == 0 || snap.players[0].bankedTurns != 77 {
		t.Errorf("banked turns not in snapshot")
	}
	_ = p
}
