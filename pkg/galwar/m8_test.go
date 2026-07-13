package galwar

import (
	"strings"
	"testing"
)

// invasionSetup makes an attacker and a defender-owned planet in the same
// sector, with the given garrison and mines on the planet.
func invasionSetup(t *testing.T, garrison, mines int) (*UniverseType, *Player, *Player, *Planet) {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	attacker, _ := u.RegisterPlayer("Invader", "i@example.com", "sa")
	defender, _ := u.RegisterPlayer("Holder", "h@example.com", "sd")
	defender.MoveTo(50)
	defender.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(defender, 50, "Fortress"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]
	planet.SetQuantity(FIGHTERS, garrison)
	planet.SetQuantity(MINES, mines)
	attacker.MoveTo(50)
	return u, attacker, defender, planet
}

func TestInvadeUndefended(t *testing.T) {
	u, attacker, _, planet := invasionSetup(t, 0, 0)
	turns := attacker.GetQuantity(TURNS)

	report, err := u.InvadePlanet(attacker, 100)
	if err != nil {
		t.Fatalf("invade: %v", err)
	}
	if planet.Owner != attacker.Id {
		t.Errorf("undefended planet not captured")
	}
	if attacker.GetQuantity(TURNS) != turns-1 {
		t.Errorf("invasion did not cost a turn")
	}
	joined := strings.Join(report, "\n")
	if !strings.Contains(joined, "captured Fortress") {
		t.Errorf("no capture message: %v", report)
	}
}

func TestInvadeDefendedWin(t *testing.T) {
	u, attacker, defender, planet := invasionSetup(t, 50, 0)
	attacker.SetQuantity(FIGHTERS, 100000)

	if _, err := u.InvadePlanet(attacker, 100000); err != nil {
		t.Fatalf("invade: %v", err)
	}
	if planet.Owner != attacker.Id {
		t.Errorf("planet not captured after overwhelming assault")
	}
	if planet.GetQuantity(FIGHTERS) != 0 {
		t.Errorf("garrison not wiped on capture: %d", planet.GetQuantity(FIGHTERS))
	}
	// the former owner has news of the loss
	news := u.TakeNews(defender.Id)
	if !strings.Contains(strings.Join(news, "\n"), "captured by Invader") {
		t.Errorf("defender not notified of capture: %v", news)
	}
}

func TestInvadeRepelled(t *testing.T) {
	// a huge garrison, a token assault: the attacker loses its commitment and
	// is repelled without capturing (survives, since it committed few)
	u, attacker, defender, planet := invasionSetup(t, 100000, 0)
	attacker.SetQuantity(FIGHTERS, 5000)

	report, err := u.InvadePlanet(attacker, 100)
	if err != nil {
		t.Fatalf("invade: %v", err)
	}
	if planet.Owner == attacker.Id {
		t.Errorf("token assault captured a 100000-fighter garrison")
	}
	if attacker.IsDead() {
		t.Errorf("attacker died despite committing only 100 of 5000 fighters")
	}
	if planet.Owner != defender.Id {
		t.Errorf("planet changed hands on a repelled assault")
	}
	if !strings.Contains(strings.Join(report, "\n"), "hold") {
		t.Errorf("no repelled message: %v", report)
	}
}

func TestInvadeDeathAndEmWarp(t *testing.T) {
	// commit everything into an unwinnable garrison -> attacker dies
	u, attacker, _, _ := invasionSetup(t, 100000, 0)
	attacker.SetQuantity(FIGHTERS, 100)
	if _, err := u.InvadePlanet(attacker, 100); err != nil {
		t.Fatalf("invade: %v", err)
	}
	if !attacker.IsDead() {
		t.Errorf("attacker survived committing all 100 fighters into a 100000 garrison")
	}

	// same, but with an Emergency Warp: survives and escapes
	u2, atk2, _, _ := invasionSetup(t, 100000, 0)
	atk2.SetQuantity(FIGHTERS, 100)
	atk2.SetQuantity(EMWARP, 1)
	if _, err := u2.InvadePlanet(atk2, 100); err != nil {
		t.Fatalf("invade2: %v", err)
	}
	if atk2.IsDead() {
		t.Errorf("Emergency Warp did not save the invader")
	}
	if atk2.GetQuantity(EMWARP) != 0 {
		t.Errorf("Emergency Warp not consumed")
	}
}

func TestInvadeMinefield(t *testing.T) {
	// no garrison, but a heavy minefield: enough mines destroy the invader
	u, attacker, _, planet := invasionSetup(t, 0, 100)
	attacker.SetQuantity(FIGHTERS, 400) // 100 mines * ~350 each >> 400
	if _, err := u.InvadePlanet(attacker, 400); err != nil {
		t.Fatalf("invade: %v", err)
	}
	if !attacker.IsDead() {
		t.Errorf("invader survived a 100-mine minefield with 400 fighters")
	}
	_ = planet
}

func TestInvadeDamagesPlanet(t *testing.T) {
	// a garrison big enough to cost the attacker >2000 fighters (dam >= 100)
	// wrecks the planet's production on capture
	u, attacker, _, planet := invasionSetup(t, 50000, 0)
	planet.GetCommodity(ORE).Prod = 5000
	planet.GetCommodity(ORGANICS).Prod = 5000
	planet.GetCommodity(EQUIPMENT).Prod = 5000
	attacker.SetQuantity(FIGHTERS, 200000)

	oreBefore := planet.GetCommodity(ORE).Prod
	if _, err := u.InvadePlanet(attacker, 200000); err != nil {
		t.Fatalf("invade: %v", err)
	}
	if planet.Owner != attacker.Id {
		t.Fatalf("did not capture")
	}
	if planet.GetCommodity(ORE).Prod >= oreBefore {
		t.Errorf("bloody assault did not damage planet production: %d -> %d", oreBefore, planet.GetCommodity(ORE).Prod)
	}
}

func TestInvadeValidation(t *testing.T) {
	u, attacker, _, _ := invasionSetup(t, 10, 0)

	// can't invade a planet you already own (defender owns the one at 50;
	// give the attacker their own planet at 60 and try to "invade" it)
	attacker.MoveTo(60)
	attacker.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(attacker, 60, "Home"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	if _, err := u.InvadePlanet(attacker, 100); err == nil {
		t.Errorf("allowed invading a planet the attacker owns")
	}

	// no planet in the sector
	attacker.MoveTo(70)
	if _, err := u.InvadePlanet(attacker, 100); err == nil {
		t.Errorf("invaded an empty sector")
	}

	// too few / too many fighters
	attacker.MoveTo(50)
	if _, err := u.InvadePlanet(attacker, 0); err == nil {
		t.Errorf("invaded with zero fighters")
	}
	if _, err := u.InvadePlanet(attacker, 1<<30); err == nil {
		t.Errorf("invaded with more fighters than owned")
	}

	// damaged thrusters block invasion
	attacker.DamageSystem(SysThrusters, 5)
	if _, err := u.InvadePlanet(attacker, 5); err == nil {
		t.Errorf("invaded with damaged thrusters")
	}
}
