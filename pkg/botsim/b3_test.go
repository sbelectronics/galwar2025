package botsim

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

func aggressorView(sector int) *View {
	v := baseView(sector)
	v.Prices.TargetFloor = 200000
	return v
}

func TestAggressorAttacksWeakerShip(t *testing.T) {
	a := NewAggressor("A", rand.New(rand.NewSource(1)))
	v := aggressorView(30) // outside fed space
	v.Self.Fighters = 800
	// a beatable, above-floor enemy: value ~ 300000 -> est ~1020 fighters... too
	// strong. Use a softer one: value 250000 -> est ~850; 800 < 1.5*850, declines.
	// Give it a genuinely weak target: value 210000 -> est ~714; still 800<1071.
	// Make the Aggressor strong enough to bite.
	v.Self.Fighters = 2000
	v.Here.Ships = []ShipView{{Id: "e1", Name: "Weakling", Value: 210000}}

	act := a.Plan(v)
	if act.Kind != "attack" || act.Tokens[0] != "a" {
		t.Fatalf("Aggressor should attack a weaker above-floor ship, got %+v", act)
	}
}

func TestAggressorDeclinesStrongerShip(t *testing.T) {
	a := NewAggressor("A", rand.New(rand.NewSource(1)))
	v := aggressorView(30)
	v.Self.Fighters = 300
	v.Here.Ships = []ShipView{{Id: "e1", Name: "Titan", Value: 900000}} // way stronger
	v.Mem.Observe(SectorView{Sector: 30, Warps: []int{31, 32}}, "self", 1, false)
	v.Mem.MarkVisited(30)

	act := a.Plan(v)
	if act.Kind == "attack" {
		t.Errorf("Aggressor should not attack a much stronger ship, got %+v", act)
	}
}

func TestAggressorSparesNewbies(t *testing.T) {
	a := NewAggressor("A", rand.New(rand.NewSource(1)))
	v := aggressorView(30)
	v.Self.Fighters = 5000 // could crush anyone
	v.Here.Ships = []ShipView{{Id: "n1", Name: "Newbie", Value: 67000}}
	v.Mem.Observe(SectorView{Sector: 30, Warps: []int{31}}, "self", 1, false)
	v.Mem.MarkVisited(30)

	act := a.Plan(v)
	if act.Kind == "attack" {
		t.Errorf("Aggressor must not farm sub-floor newbies, got %+v", act)
	}
}

func TestAggressorInvadesWeakPlanet(t *testing.T) {
	a := NewAggressor("A", rand.New(rand.NewSource(1)))
	v := aggressorView(30)
	v.Self.Fighters = 2000
	v.Here.Planets = []PlanetView{{Sector: 30, Name: "SoftWorld", Owner: "e1", OwnerName: "Rival",
		Fighters: 100, KnownDefense: true}}

	act := a.Plan(v)
	if act.Kind != "invade" || act.Tokens[0] != "l" {
		t.Fatalf("Aggressor should invade a weakly-held foreign planet, got %+v", act)
	}
}

func TestAggressorLeavesFactionStronghold(t *testing.T) {
	a := NewAggressor("A", rand.New(rand.NewSource(1)))
	v := aggressorView(30)
	v.Self.Fighters = 50000
	v.Here.Planets = []PlanetView{{Sector: 30, Name: "Cabal HQ", Owner: "npc", OwnerName: "The Cabal",
		Fighters: 100, KnownDefense: true}}
	v.Mem.Observe(SectorView{Sector: 30, Warps: []int{31}}, "self", 1, false)
	v.Mem.MarkVisited(30)

	act := a.Plan(v)
	if act.Kind == "invade" {
		t.Errorf("Aggressor should not invade a faction stronghold, got %+v", act)
	}
}

func TestUnexpectedErrorClassification(t *testing.T) {
	expected := []galwar.GameErrorCode{
		galwar.ErrNotEnoughMoney, galwar.ErrNotEnoughQuantity,
		galwar.ErrNotEnoughHolds, galwar.ErrNoTurns, galwar.ErrDead,
	}
	for _, c := range expected {
		if unexpectedError(galwar.NewGameError(c, "x")) {
			t.Errorf("code %v should be an expected (routine) error", c)
		}
	}
	unexpected := []galwar.GameErrorCode{
		galwar.ErrFedRestricted, galwar.ErrNotFound, galwar.ErrInvalidName,
		galwar.ErrNegativeQuantity, galwar.ErrNotOwner,
	}
	for _, c := range unexpected {
		if !unexpectedError(galwar.NewGameError(c, "x")) {
			t.Errorf("code %v should be flagged as a finding", c)
		}
	}
	if !unexpectedError(errors.New("plain error")) {
		t.Error("a non-game error should be flagged")
	}
}
