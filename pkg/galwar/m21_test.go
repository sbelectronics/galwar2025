package galwar

import (
	"strings"
	"testing"
	"time"
)

// M21: faction strikes route through sectors, and placed defenses on the path
// blunt or stop them. These tests use a hand-built linear topology so the
// route stronghold -> target is known exactly.

// linearUniverse builds sectors 1..n in a chain (1<->2<->...<->n), plus the
// off-map sector 0. ShortestPathTo(a,b) is then the obvious run of integers.
func linearUniverse(t *testing.T, n int) *UniverseType {
	t.Helper()
	u := NewUniverse()
	u.SeedDefaultConfig()
	for i := 0; i <= n; i++ {
		u.Sectors = append(u.Sectors, Sector{Number: i, Warps: []int{}})
	}
	for i := 1; i < n; i++ {
		u.Sectors[i].AddWarp(i + 1)
		u.Sectors[i+1].AddWarp(i)
	}
	return u
}

// strikeSetup returns a Cabal with a stronghold at sector 11 (fighters set by
// caller) and a fresh active target at targetSector.
func strikeSetup(t *testing.T, u *UniverseType, targetSector, strongholdFighters, targetFighters int) (*Player, *Planet, *Player) {
	t.Helper()
	cabal := u.EnsureNPC("cabal")
	stronghold := u.NewPlanet(cabal.Id, 11, cabalStrongholdName)
	stronghold.SetQuantity(FIGHTERS, strongholdFighters)

	target, err := u.RegisterPlayer("Warlord", "w@x.com", "")
	if err != nil {
		t.Fatalf("register target: %v", err)
	}
	target.EverMoved = true
	target.MoveTo(targetSector)
	target.SetQuantity(FIGHTERS, targetFighters)
	target.SetQuantity(EMWARP, 0) // no lucky escape in these tests
	u.TouchLastSeen(target, time.Now().Unix())
	return cabal, stronghold, target
}

func TestFactionStrikeBluntedByChokepointGarrison(t *testing.T) {
	u := linearUniverse(t, 20)
	cabal, stronghold, target := strikeSetup(t, u, 15, 5000, 100)

	// a defender parks a huge garrison in sector 13, on the route 11->15
	defender, _ := u.RegisterPlayer("Gatekeeper", "g@x.com", "")
	garrison := u.NewBattlegroup(defender.Id, 13)
	garrison.SetQuantity(FIGHTERS, 10000)

	killed := u.factionStrike(cabal, stronghold, target, 5000, time.Now().Unix())

	if killed || target.IsDead() {
		t.Errorf("strike reached the target through a wall of 10,000 fighters")
	}
	if target.GetQuantity(FIGHTERS) != 100 {
		t.Errorf("target took losses despite the fleet being stopped en route: %d", target.GetQuantity(FIGHTERS))
	}
	if garrison.GetQuantity(FIGHTERS) >= 10000 {
		t.Errorf("chokepoint garrison did not engage the fleet: %d", garrison.GetQuantity(FIGHTERS))
	}
	if stronghold.GetQuantity(FIGHTERS) != 0 {
		t.Errorf("fleet should have been wiped at the chokepoint; stronghold has %d back", stronghold.GetQuantity(FIGHTERS))
	}
	if news := strings.Join(u.TakeNews(defender.Id), "\n"); !strings.Contains(news, "sector 13") {
		t.Errorf("defender not told their garrison engaged the Cabal: %q", news)
	}
}

func TestFactionStrikeBluntedByHomeMinefield(t *testing.T) {
	u := linearUniverse(t, 20)
	cabal, stronghold, target := strikeSetup(t, u, 15, 3000, 100)

	// the target mines their own home sector (the last hop of the route)
	minefield := u.NewBattlegroup(target.Id, 15)
	minefield.SetQuantity(MINES, 12)

	killed := u.factionStrike(cabal, stronghold, target, 1000, time.Now().Unix())

	if killed || target.IsDead() {
		t.Errorf("strike killed the target through their own minefield")
	}
	if target.GetQuantity(FIGHTERS) != 100 {
		t.Errorf("target ship took losses though the minefield stopped the fleet: %d", target.GetQuantity(FIGHTERS))
	}
	if minefield.GetQuantity(MINES) >= 12 {
		t.Errorf("home minefield did not detonate against the fleet: %d mines left", minefield.GetQuantity(MINES))
	}
}

func TestFactionStrikeUnreachableTargetSpared(t *testing.T) {
	u := linearUniverse(t, 20)
	// sector 25 exists but is an island (no warps): unreachable from 11
	u.Sectors = append(u.Sectors, Sector{Number: 25, Warps: []int{}})
	cabal, stronghold, target := strikeSetup(t, u, 25, 5000, 100)

	killed := u.factionStrike(cabal, stronghold, target, 5000, time.Now().Unix())

	if killed || target.IsDead() {
		t.Errorf("struck an unreachable target")
	}
	if stronghold.GetQuantity(FIGHTERS) != 5000 {
		t.Errorf("fleet launched at an unreachable target: stronghold has %d", stronghold.GetQuantity(FIGHTERS))
	}
}

func TestFactionStrikeSparesTransitBystander(t *testing.T) {
	u := linearUniverse(t, 20)
	cabal, stronghold, target := strikeSetup(t, u, 15, 100000, 100)

	// a bystander's MOBILE SHIP sits in sector 13, on the route. It is not a
	// placed defense (no garrison, no mines), so the fleet must pass it by.
	bystander, _ := u.RegisterPlayer("Bystander", "b@x.com", "")
	bystander.EverMoved = true
	bystander.MoveTo(13)
	bystander.SetQuantity(FIGHTERS, 50)
	u.TouchLastSeen(bystander, time.Now().Unix())

	killed := u.factionStrike(cabal, stronghold, target, 100000, time.Now().Unix())

	if bystander.IsDead() || bystander.GetQuantity(FIGHTERS) != 50 {
		t.Errorf("a mobile ship in a transit sector was attacked: dead=%v fighters=%d", bystander.IsDead(), bystander.GetQuantity(FIGHTERS))
	}
	// the overwhelming fleet still crushes the intended target at the destination
	if !killed || !target.IsDead() {
		t.Errorf("overwhelming strike did not kill the intended target")
	}
}

func TestFactionStrikeReachesUndefendedTarget(t *testing.T) {
	// sanity: with a clear route and no defenses, the strike lands as before
	u := linearUniverse(t, 20)
	cabal, stronghold, target := strikeSetup(t, u, 15, 5000, 100)

	if !u.factionStrike(cabal, stronghold, target, 5000, time.Now().Unix()) {
		t.Errorf("overwhelming strike down an undefended route did not kill the target")
	}
	if !target.IsDead() {
		t.Errorf("target not dead after an unopposed strike")
	}
}
