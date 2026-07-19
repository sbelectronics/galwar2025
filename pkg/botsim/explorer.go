package botsim

import (
	"math/rand"
	"strconv"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// Explorer tours the whole map, never settling into a pair. It peddles at every
// trading port it passes to stay solvent, and buys a Planetary Scanner when it
// can. It stresses pathfinding, long autopilot chains, the device shop, and the
// sector-visibility code from every angle (PLAN-BOTS.md 3.6).
type Explorer struct {
	*Trader
	lastStop int // last sector peddled at, so it trades once per stop
}

// NewExplorer builds an Explorer brain.
func NewExplorer(name string, rng *rand.Rand) *Explorer {
	t := NewTrader(name, "explorer", rng)
	return &Explorer{Trader: t, lastStop: -1}
}

// Plan buys a device at a service port, peddles at a trading port, or presses on
// to the next unexplored sector.
func (e *Explorer) Plan(v *View) Action {
	if !v.Self.HasTurns() {
		return pass()
	}
	if port, ok := v.Here.Port(); ok {
		switch {
		case port.Kind == galwar.AmazingDevices || port.Kind == galwar.Sol:
			if a, ok := e.buyScanner(v, port); ok {
				return a
			}
		case port.IsTrading() && v.Self.Sector != e.lastStop:
			e.lastStop = v.Self.Sector
			return e.dockTrade(v, port) // peddle: a bet that a port ahead buys this
		}
	}
	// press on to the next unexplored sector, one careful hop at a time (travel
	// walks the frontier over safe warps; blind jumps only to escape a pocket)
	e.dest, e.job = 0, "explore"
	return e.travel(v)
}

// buyScanner buys one Planetary Scanner when the bot lacks one and can afford
// it (it is sold at the Amazing Devices port). Returns false otherwise so the
// bot moves on rather than looping on a free service-port visit.
func (e *Explorer) buyScanner(v *View, port PortView) (Action, bool) {
	if v.Self.Devices[galwar.PLANETSCANNER] > 0 {
		return Action{}, false
	}
	idx := port.MenuIndex(galwar.PLANETSCANNER)
	if idx == 0 || v.Self.Money < v.Prices.Scanner+5000 {
		return Action{}, false
	}
	e.lastDockMoney = -1 // a device purchase is not a trading loss: recalibrate
	return act("buy_device", "p", strconv.Itoa(idx), "1", "q").
		withDetail(map[string]any{"device": galwar.PLANETSCANNER}), true
}
