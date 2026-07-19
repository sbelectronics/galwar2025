package botsim

import (
	"fmt"
	"math/rand"
	"strconv"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// planetNames are clean, moderation-passing names for a bot's colony.
var planetNames = []string{
	"New Hope", "Terra Nova", "Haven", "Providence", "New Eden",
	"Prosperity", "Arcadia", "Homestead", "Concord", "Elysium",
}

// PlanetBuilder trades like a Trader but earmarks profit toward founding and
// developing a planet: it buys a Genesis Device, plants a colony on its trade
// route, and feeds it cargo and fighters as it passes (PLAN-BOTS.md 3.2).
type PlanetBuilder struct {
	*Trader
	minTradeReserve int // credits kept for trading before buying a Genesis Device
	planetFighters  int // target garrison to leave on the colony
	feedEvery       int // trips between colony feedings
	lastFed         int // trips value at the last feeding
}

// NewPlanetBuilder builds a PlanetBuilder brain.
func NewPlanetBuilder(name string, rng *rand.Rand) *PlanetBuilder {
	t := NewTrader(name, "planet", rng)
	return &PlanetBuilder{Trader: t, minTradeReserve: 8000, planetFighters: 300, feedEvery: 4}
}

// Plan layers the planet program on top of trading: found a colony when it has
// a device and stands somewhere legal, buy a device when it can afford one,
// feed the colony when passing it, else trade.
func (p *PlanetBuilder) Plan(v *View) Action {
	if !v.Self.HasTurns() {
		return pass()
	}

	haveDevice := v.Self.Devices[galwar.GENESIS] > 0
	ownPlanet := len(v.OwnPlanets) > 0

	// 1. found a colony where it stands, if it can
	if haveDevice && !ownPlanet && p.plantableHere(v) {
		name := fmt.Sprintf("%s %d", planetNames[p.rng.Intn(len(planetNames))], 1+p.rng.Intn(99))
		return act("genesis", "j", name).withDetail(map[string]any{"sector": v.Self.Sector, "name": name})
	}

	// 2. buy a Genesis Device at Sol once it can spare the credits
	if !haveDevice && !ownPlanet && v.Self.Money >= v.Prices.Genesis+p.minTradeReserve {
		if sol := v.Mem.NearestServicePort(v.Self.Sector, galwar.Sol); sol != 0 {
			if v.Self.Sector == sol {
				if a, ok := p.buyGenesis(v); ok {
					return a
				}
			} else {
				return p.moveToward(v, sol)
			}
		}
	}

	// 3. feed the colony when passing through its sector - but only when there
	// is something to give and not more than once per few trips (feeding costs
	// no turn, so an unconditional feed at the planet sector is a free loop)
	if ownPlanet {
		pl := v.OwnPlanets[0]
		if v.Self.Sector == pl.Sector && p.trips-p.lastFed >= p.feedEvery {
			if a, ok := p.feed(v, pl); ok {
				p.lastFed = p.trips
				return a
			}
		}
	}

	return p.Trader.Plan(v)
}

// plantableHere reports whether the current sector can take a new colony:
// outside fed space and with no planet already present.
func (p *PlanetBuilder) plantableHere(v *View) bool {
	if v.Self.Sector <= 10 {
		return false
	}
	return len(v.Here.Planets) == 0
}

// buyGenesis buys one Genesis Device from Sol's fixed-price menu.
func (p *PlanetBuilder) buyGenesis(v *View) (Action, bool) {
	port, ok := v.Here.Port()
	if !ok || port.Kind != galwar.Sol {
		return Action{}, false
	}
	idx := port.MenuIndex(galwar.GENESIS)
	if idx == 0 {
		return Action{}, false
	}
	p.lastDockMoney = -1 // a 10k purchase is not a trading loss: recalibrate
	return act("buy_device", "p", strconv.Itoa(idx), "1", "q").
		withDetail(map[string]any{"device": galwar.GENESIS}), true
}

// feed develops the colony: leave fighters for defense when the bot has spare,
// otherwise dump its cargo to fuel production. Each is a self-contained land -
// act - leave dialog. ok=false means there is nothing worth transferring, so
// the caller falls through to trading instead of a no-op land/leave loop.
func (p *PlanetBuilder) feed(v *View, pl PlanetView) (Action, bool) {
	spare := v.Self.Fighters - p.fighterFloor
	if pl.KnownDefense && pl.Fighters < p.planetFighters && spare > 0 {
		leave := pl.Fighters + spare
		if leave > p.planetFighters {
			leave = p.planetFighters
		}
		return act("planet_transfer", "l", "f", strconv.Itoa(leave), "l").
			withDetail(map[string]any{"sector": pl.Sector, "leave_fighters": leave}), true
	}
	// dump cargo to the colony (grows production), then leave - but only if we
	// are actually carrying something
	if v.Self.Holds-v.Self.FreeHolds > 0 {
		return act("planet_transfer", "l", "t", "y", "l").
			withDetail(map[string]any{"sector": pl.Sector, "cargo": true}), true
	}
	return Action{}, false
}
