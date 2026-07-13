package galwar

import (
	"testing"
)

// clearWarps strips every warp touching a sector (both directions), so plasma
// tests don't depend on the random generator's warp counts.
func clearWarps(u *UniverseType, sec int) {
	for _, w := range append([]int{}, u.Sectors[sec].Warps...) {
		u.Sectors[sec].RemoveWarp(w)
		u.Sectors[w].RemoveWarp(sec)
	}
}

func specialsUniverse(t *testing.T) (*UniverseType, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Specials", "s@example.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	p.MoveTo(50)
	return u, p
}

func TestSolSellsSpecials(t *testing.T) {
	u, _ := specialsUniverse(t)
	sol := u.Ports.Ports[0]
	if sol.Goods != Sol {
		t.Fatalf("port 0 is not Sol")
	}
	for _, name := range []string{PLASMA, PULSAR, EMWARP} {
		cm := sol.GetCommodity(name)
		if cm == nil || !cm.Sell {
			t.Errorf("Sol does not sell %s", name)
		}
	}
	// starting players carry none
	p := u.Players.Players[0]
	for _, name := range []string{PLASMA, PULSAR, EMWARP} {
		if p.GetQuantity(name) != 0 {
			t.Errorf("new player starts with %s", name)
		}
	}
}

func TestPlasmaAddRemove(t *testing.T) {
	u, p := specialsUniverse(t)
	p.SetQuantity(PLASMA, 5)

	// federation space is off-limits
	p.MoveTo(5)
	if _, err := u.UsePlasma(p, PlasmaAdd, 60); err == nil {
		t.Errorf("plasma allowed in fed space")
	}
	p.MoveTo(50)
	if _, err := u.UsePlasma(p, PlasmaAdd, 5); err == nil {
		t.Errorf("plasma allowed to link fed-space target")
	}
	if _, err := u.UsePlasma(p, PlasmaAdd, 50); err == nil {
		t.Errorf("plasma allowed to link a sector to itself")
	}

	// add a two-way warp 50<->60 where none existed (clear both first so the
	// generator's warp state doesn't interfere)
	clearWarps(u, 50)
	clearWarps(u, 60)
	before := p.GetQuantity(PLASMA)
	turns := p.GetQuantity(TURNS)
	if _, err := u.UsePlasma(p, PlasmaAdd, 60); err != nil {
		t.Fatalf("plasma add: %v", err)
	}
	if !u.Sectors[50].HasWarp(60) || !u.Sectors[60].HasWarp(50) {
		t.Errorf("plasma add did not create a two-way link")
	}
	if p.GetQuantity(PLASMA) != before-1 {
		t.Errorf("plasma not consumed")
	}
	if p.GetQuantity(TURNS) != turns-1 {
		t.Errorf("plasma add did not cost a turn")
	}

	// adding the same link again is refused
	if _, err := u.UsePlasma(p, PlasmaAdd, 60); err == nil {
		t.Errorf("duplicate plasma link allowed")
	}

	// remove it (two-way)
	if _, err := u.UsePlasma(p, PlasmaRemove, 60); err != nil {
		t.Fatalf("plasma remove: %v", err)
	}
	if u.Sectors[50].HasWarp(60) || u.Sectors[60].HasWarp(50) {
		t.Errorf("plasma remove did not sever the two-way link")
	}

	// no plasma left check
	p.SetQuantity(PLASMA, 0)
	if _, err := u.UsePlasma(p, PlasmaAdd, 61); err == nil {
		t.Errorf("plasma used with none in stock")
	}
}

func TestPlasmaMaxWarps(t *testing.T) {
	u, p := specialsUniverse(t)
	p.SetQuantity(PLASMA, 20)
	p.MoveTo(50)

	// fill sector 50 up to max_warps with links to high sectors
	max := u.ConfigInt("max_warps", 8)
	clearWarps(u, 50)
	target := 60
	for len(u.Sectors[50].Warps) < max {
		clearWarps(u, target) // make sure the target has room too
		if _, err := u.UsePlasma(p, PlasmaAdd, target); err != nil {
			t.Fatalf("filling warps: %v", err)
		}
		target++
	}
	// one more must be refused (sector full)
	if _, err := u.UsePlasma(p, PlasmaAdd, target); err == nil {
		t.Errorf("plasma exceeded the max-warps cap")
	}
}

func TestPulsarBombsOwnPlanet(t *testing.T) {
	u, p := specialsUniverse(t)
	p.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(p, 50, "Homestead"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]
	// grow production so it survives a single bomb: set prod high
	planet.GetCommodity(ORE).Prod = 3000
	planet.GetCommodity(ORGANICS).Prod = 3000
	planet.GetCommodity(EQUIPMENT).Prod = 3000

	p.SetQuantity(PULSAR, 10)

	// can't bomb someone else's planet (ownership enforced via GetPlanet)
	other, _ := u.RegisterPlayer("Other", "o@example.com", "")
	other.MoveTo(50)
	other.SetQuantity(PULSAR, 5)
	if _, err := u.UsePulsar(other, 1); err == nil {
		t.Errorf("bombed a planet the player doesn't own")
	}

	// one bomb destroys up to 1000 production per commodity
	turns := p.GetQuantity(TURNS)
	if _, err := u.UsePulsar(p, 1); err != nil {
		t.Fatalf("pulsar: %v", err)
	}
	if got := planet.GetCommodity(ORE).Prod; got != 2000 {
		t.Errorf("ore prod after 1 bomb = %d; want 2000", got)
	}
	if p.GetQuantity(PULSAR) != 9 {
		t.Errorf("pulsar not consumed")
	}
	if p.GetQuantity(TURNS) != turns-1 {
		t.Errorf("pulsar did not cost a turn")
	}

	// enough bombs to zero all production destroys the planet
	if _, err := u.UsePulsar(p, 2); err != nil { // 2 bombs = 2000 each, zeroes the remaining 2000
		t.Fatalf("pulsar: %v", err)
	}
	if len(u.Planets.Planets) != 0 {
		t.Errorf("planet not destroyed when all production hit zero")
	}
}

func TestEmWarpSavesFromDeath(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	attacker, _ := u.RegisterPlayer("Attacker", "a@example.com", "sa")
	victim, _ := u.RegisterPlayer("Victim", "v@example.com", "sv")
	attacker.MoveTo(50)
	victim.MoveTo(50)

	// certain-death setup, but the victim carries an Emergency Warp
	attacker.SetQuantity(FIGHTERS, 100000)
	victim.SetQuantity(FIGHTERS, 5)
	victim.SetQuantity(MINES, 0)
	victim.SetQuantity(EMWARP, 1)

	report, err := u.AttackPlayer(attacker, victim.Id, 100000)
	if err != nil {
		t.Fatalf("attack: %v", err)
	}
	if victim.IsDead() {
		t.Fatalf("victim died despite Emergency Warp; report: %v", report)
	}
	if victim.GetQuantity(EMWARP) != 0 {
		t.Errorf("Emergency Warp not consumed")
	}
	if victim.Sector == 50 || victim.Sector < 1 {
		t.Errorf("victim not teleported to a valid random sector: %d", victim.Sector)
	}
	// the victim has news about the escape
	news := u.TakeNews(victim.Id)
	found := false
	for _, n := range news {
		if len(n) > 0 && (n[:1] == "Y") { // "Your Emergency Warp device energized..."
			found = true
		}
	}
	if !found {
		t.Errorf("victim got no Emergency Warp news: %v", news)
	}

	// without an Emergency Warp, the same attack kills (reset the attacker's
	// fighters, which the first exchange depleted, and their turns)
	attacker.SetQuantity(FIGHTERS, 100000)
	attacker.SetQuantity(TURNS, 250)
	victim2, _ := u.RegisterPlayer("Victim Two", "v2@example.com", "sv2")
	victim2.MoveTo(50)
	victim2.SetQuantity(FIGHTERS, 5)
	if _, err := u.AttackPlayer(attacker, victim2.Id, 100000); err != nil {
		t.Fatalf("attack2: %v", err)
	}
	if !victim2.IsDead() {
		t.Errorf("victim without Emergency Warp survived")
	}
}

func TestUpgradeAddsSpecials(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	old, _ := u.RegisterPlayer("Old", "old@example.com", "")

	// simulate a pre-M7 save: strip the new commodities from the player and Sol
	for _, name := range []string{PLASMA, PULSAR, EMWARP} {
		for i, c := range old.Inventory {
			if c.Name == name {
				old.Inventory = append(old.Inventory[:i], old.Inventory[i+1:]...)
				break
			}
		}
	}
	sol := u.Ports.Ports[0]
	for _, name := range []string{PLASMA, PULSAR, EMWARP} {
		for i, c := range sol.Inventory {
			if c.Name == name {
				sol.Inventory = append(sol.Inventory[:i], sol.Inventory[i+1:]...)
				break
			}
		}
	}

	u.upgrade()

	for _, name := range []string{PLASMA, PULSAR, EMWARP} {
		if old.GetCommodity(name) == nil {
			t.Errorf("legacy player did not gain %s", name)
		}
		if cm := sol.GetCommodity(name); cm == nil || !cm.Sell {
			t.Errorf("legacy Sol did not learn to sell %s", name)
		}
	}
}
