package botsim

import (
	"math/rand"
	"strconv"
)

// Aggressor trades like a Trader but diverts to fight when the odds look good:
// it invades weak planets, attacks weaker ships outside Federation space, and
// charges known-weaker sector forces. It only knows what it can see, so its
// strength estimates are sometimes wrong - and those mispredicted fights are
// the interesting ones (PLAN-BOTS.md 3.4).
type Aggressor struct {
	*Trader
	shipMargin   float64 // required fighter advantage over a ship (1.5 = 50% more)
	forceMargin  float64 // required advantage over a sector battlegroup
	planetMargin float64 // required advantage over a planet garrison
	unknownGuess int     // assumed garrison of a planet it can't scan
	retreatFrac  int     // retreat to repair below this % of a healthy fighter count
	healthyFtr   int
}

// NewAggressor builds an Aggressor brain.
func NewAggressor(name string, rng *rand.Rand) *Aggressor {
	t := NewTrader(name, "aggressor", rng)
	t.holdPct = 25       // pour reinvestment into fighters - it fights for a living
	t.fighterFloor = 100 // keeps more fighters aboard than a peaceful trader
	return &Aggressor{
		Trader: t, shipMargin: 1.5, forceMargin: 1.3, planetMargin: 1.5,
		unknownGuess: 500, healthyFtr: 200, retreatFrac: 40,
	}
}

// Plan looks for a favorable fight before falling back to trading.
func (a *Aggressor) Plan(v *View) Action {
	if !v.Self.HasTurns() {
		return pass()
	}
	// too weak to fight: fall back to the Trader (which restocks/retreats/trades)
	if v.Self.Fighters < a.healthyFtr*a.retreatFrac/100 {
		return a.Trader.Plan(v)
	}

	// 1. a weaker enemy ship in this sector, outside Federation space
	if v.Self.Sector > 10 {
		if act, ok := a.attackShip(v); ok {
			return act
		}
	}
	// 2. a foreign planet here we can take
	if act, ok := a.invade(v); ok {
		return act
	}
	// 3. a known-weaker sector force in an adjacent sector - charge in
	if act, ok := a.chargeForce(v); ok {
		return act
	}
	return a.Trader.Plan(v)
}

// attackShip attacks the weakest beatable enemy ship in the current sector.
func (a *Aggressor) attackShip(v *View) (Action, bool) {
	best, bestIdx := -1, -1
	for i, sh := range v.Here.Ships {
		if sh.Value < v.Prices.TargetFloor { // don't farm newbies (mirrors the faction floor)
			continue
		}
		est := estFighters(sh.Value, v.Prices.Fighter)
		if float64(v.Self.Fighters) < a.shipMargin*float64(est) {
			continue
		}
		if best == -1 || sh.Value < best { // prefer the softest qualifying target
			best, bestIdx = sh.Value, i
		}
	}
	if bestIdx < 0 {
		return Action{}, false
	}
	// [A] dialog: pick target (1-based, same visible order), confirm, commit all
	return act("attack", "a", strconv.Itoa(bestIdx+1), "y", strconv.Itoa(v.Self.Fighters)).
		withDetail(map[string]any{"target": v.Here.Ships[bestIdx].Name, "my_fighters": v.Self.Fighters,
			"est_enemy": estFighters(best, v.Prices.Fighter)}), true
}

// invade attacks a foreign planet in the current sector when it looks takeable.
func (a *Aggressor) invade(v *View) (Action, bool) {
	for _, pl := range v.Here.Planets {
		if pl.Mine || pl.Sector <= 10 {
			continue
		}
		if isFactionOwner(pl.OwnerName) { // faction strongholds are out of our league
			continue
		}
		est := a.unknownGuess
		if pl.KnownDefense {
			est = pl.Fighters
		}
		if float64(v.Self.Fighters) < a.planetMargin*float64(est) {
			continue
		}
		commit := v.Self.Fighters * 8 / 10
		return act("invade", "l", "y", strconv.Itoa(commit)).
			withDetail(map[string]any{"planet": pl.Name, "owner": pl.OwnerName,
				"my_fighters": v.Self.Fighters, "est_garrison": est}), true
	}
	return Action{}, false
}

// chargeForce moves into an adjacent sector holding a known, weaker hostile
// battlegroup - the move itself triggers the fight (MovePlayer's conflict).
func (a *Aggressor) chargeForce(v *View) (Action, bool) {
	for _, w := range v.Here.Warps {
		sv, ok := v.Scan[w]
		if !ok || w <= 10 {
			continue
		}
		for _, bg := range sv.Battlegroups {
			if bg.Mine || bg.Fighters <= 0 {
				continue
			}
			if float64(v.Self.Fighters) < a.forceMargin*float64(bg.Fighters) {
				continue
			}
			a.job, a.dest = "", 0 // abandon the trade goal; we're hunting
			return act("attack", "m", strconv.Itoa(w)).
				withDetail(map[string]any{"charge_sector": w, "enemy_fighters": bg.Fighters,
					"my_fighters": v.Self.Fighters}), true
		}
	}
	return Action{}, false
}

// estFighters converts a public net-worth estimate into a guess of a ship's
// fighter strength. Fighters are only a fraction of a player's value (which also
// counts cargo, holds, and bank), so this deliberately underestimates - the
// Aggressor will sometimes bite off more than it can chew, which is the point.
func estFighters(value, costFighter int) int {
	if costFighter <= 0 {
		costFighter = 98
	}
	return value / costFighter / 3
}

// isFactionOwner reports whether a name looks like one of the NPC factions,
// whose strongholds an Aggressor should leave alone.
func isFactionOwner(name string) bool {
	switch name {
	case "The Federation", "The Cabal", "The Renegades", "Federation", "Cabal", "Renegades":
		return true
	}
	return false
}
