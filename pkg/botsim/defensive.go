package botsim

import (
	"math/rand"
	"strconv"
)

// DefensiveTrader trades like a Trader but garrisons its two favorite port
// sectors (its trade pair) with fighters, topping up sector battlegroups as its
// net worth grows. It exercises battlegroup upkeep and the toll/hostile
// interactions other bots have with parked forces (PLAN-BOTS.md 3.3).
type DefensiveTrader struct {
	*Trader
	garrisonBase int // baseline garrison per sector, before the net-worth bonus
	garrisonCap  int
}

// NewDefensiveTrader builds a DefensiveTrader brain.
func NewDefensiveTrader(name string, rng *rand.Rand) *DefensiveTrader {
	t := NewTrader(name, "defensive", rng)
	t.holdPct = 40 // divert more reinvestment into fighters for defense
	return &DefensiveTrader{Trader: t, garrisonBase: 100, garrisonCap: 2000}
}

// Plan garrisons a pair port when it can, otherwise trades as the base Trader.
func (d *DefensiveTrader) Plan(v *View) Action {
	if !v.Self.HasTurns() {
		return pass()
	}
	s := v.Self.Sector
	// only at one of the two home ports, only outside fed space (galactic law
	// forbids leaving forces in sectors 1-10), and only when no foreign force
	// is present (which would make the [F] command bail before our answer)
	if (s == d.pairA || s == d.pairB) && s > 10 && !foreignForce(v.Here) {
		if a, ok := d.garrison(v); ok {
			return a
		}
	}
	return d.Trader.Plan(v)
}

// garrison deposits fighters into this sector's defense force up to a target
// that grows with net worth, always keeping the fighter floor aboard.
func (d *DefensiveTrader) garrison(v *View) (Action, bool) {
	if v.Self.Fighters <= d.fighterFloor {
		return Action{}, false
	}
	target := d.garrisonBase + v.Self.Value/2000
	if target > d.garrisonCap {
		target = d.garrisonCap
	}
	current := ownForceFighters(v.Here)
	total := v.Self.Fighters + current
	desired := target
	if cap := total - d.fighterFloor; desired > cap {
		desired = cap
	}
	if desired <= current {
		return Action{}, false // already garrisoned enough, or no spare fighters
	}
	return act("garrison", "f", strconv.Itoa(desired)).
		withDetail(map[string]any{"sector": v.Self.Sector, "target": desired, "was": current}), true
}

// foreignForce reports a battlegroup in the sector owned by someone else.
func foreignForce(sv SectorView) bool {
	for _, bg := range sv.Battlegroups {
		if !bg.Mine && bg.Fighters+bg.Mines > 0 {
			return true
		}
	}
	return false
}

// ownForceFighters totals the bot's own battlegroup fighters in a sector.
func ownForceFighters(sv SectorView) int {
	n := 0
	for _, bg := range sv.Battlegroups {
		if bg.Mine {
			n += bg.Fighters
		}
	}
	return n
}
