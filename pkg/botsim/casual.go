package botsim

import (
	"math/rand"
)

// Casual plays only some days, and only a handful of turns when it does, then
// goes quiet. It is the one bot that exercises the time-based paths - dormancy
// hiding, expiry warnings, grandfathering, and reconstruction-after-absence -
// that no always-on bot ever touches (PLAN-BOTS.md 3.7). It is expected to
// finish last; that the engine handles its neglect correctly is the test.
type Casual struct {
	*Trader
	playProb   float64
	lastDay    int
	playToday  bool
	turnBudget int
	startTurns int
}

// NewCasual builds a Casual brain.
func NewCasual(name string, rng *rand.Rand) *Casual {
	return &Casual{Trader: NewTrader(name, "casual", rng), playProb: 0.4, lastDay: -1}
}

// Plan skips whole days (accruing absence, since a day with no action never
// "logs in") and, on a play day, quits after a short burst of trading.
func (c *Casual) Plan(v *View) Action {
	if v.Day != c.lastDay {
		c.lastDay = v.Day
		c.playToday = c.rng.Float64() < c.playProb
		c.turnBudget = 10 + c.rng.Intn(21) // 10-30 turns
		c.startTurns = v.Self.Turns
	}
	if !c.playToday {
		return pass() // a skipped day: no login, absence accrues
	}
	if c.startTurns-v.Self.Turns >= c.turnBudget {
		return pass() // done for today, log off early
	}
	return c.Trader.Plan(v)
}
