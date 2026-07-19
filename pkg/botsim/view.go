// Package botsim is the scripted-bot simulation harness (PLAN-BOTS.md). It
// drives computer-controlled players through the real consoleui.ConsoleUI so a
// run exercises the whole UI stack, records a structured log of everything that
// happens, and can be replayed deterministically from a seed. Bots decide by
// perceiving engine state directly (a View) and act by emitting ordinary player
// command lines; they never screen-scrape.
package botsim

import (
	"math/rand"
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// The three trade goods, in the engine's canonical inventory order. Bots reason
// about ports in terms of which of these a port sells (exactly one) versus buys
// (the other two) - see PLAN-BOTS.md section on the economy.
var tradeGoods = []string{galwar.ORE, galwar.ORGANICS, galwar.EQUIPMENT}

// PortView is what a bot knows about a port right now. From a mere sighting
// (sector display or scan) only the sells-headline is known - which goods the
// port sells - so Full is false and Buys/Stock are empty. Standing in the
// port's own sector reveals the full commerce picture (Full true): what it
// buys and its current stock, the ground truth a trade needs.
type PortView struct {
	Sector int
	Name   string
	Kind   galwar.PortType
	Sells  map[string]bool // good -> the port sells it to players (headline knowledge)
	Buys   map[string]bool // good -> the port buys it from players (Full only)
	Stock  map[string]int  // good -> player-scaled units available (Full only)
	Menu   []string        // commodity names in the port's inventory order (Full only)
	Full   bool
}

// MenuIndex returns the 1-based position of a commodity in the port's buy menu
// (the number the service-port dialog expects), or 0 if absent.
func (p PortView) MenuIndex(name string) int {
	for i, n := range p.Menu {
		if n == name {
			return i + 1
		}
	}
	return 0
}

// SellsGood returns the single trade good this port sells, or "" if none/unknown.
func (p PortView) SellsGood() string {
	for _, g := range tradeGoods {
		if p.Sells[g] {
			return g
		}
	}
	return ""
}

// IsTrading reports a regular commodity port (not Sol, the bank, or devices).
func (p PortView) IsTrading() bool { return p.Kind == galwar.TradingPort }

// PlanetView is what a bot can see of a planet in a sector. Defenders are only
// populated (KnownDefense) when the bot owns the planet or carries a Planetary
// Scanner - otherwise a would-be invader is guessing.
type PlanetView struct {
	Sector       int
	Name         string
	Owner        galwar.PlayerId
	OwnerName    string
	Mine         bool
	Fighters     int
	Mines        int
	KnownDefense bool
}

// ShipView is another player's ship seen in a sector. Fighters are not public,
// so a bot estimates strength from the rankings (Value), never exact fighters.
type ShipView struct {
	Id    galwar.PlayerId
	Name  string
	Value int
}

// BattlegroupView is a sector defense force. Own groups report exact contents;
// a hostile group's fighters/mines are what the sector display shows.
type BattlegroupView struct {
	Sector    int
	Owner     galwar.PlayerId
	OwnerName string
	Mine      bool
	Fighters  int
	Mines     int
}

// SectorView is the contents of one sector as this bot may legitimately see it
// (GetVisibleObjectsInSector - cloak and dormancy hiding respected).
type SectorView struct {
	Sector       int
	Warps        []int
	Ports        []PortView
	Planets      []PlanetView
	Ships        []ShipView
	Battlegroups []BattlegroupView
}

// Port returns the trading port in this sector, if any.
func (s SectorView) Port() (PortView, bool) {
	for _, p := range s.Ports {
		return p, true
	}
	return PortView{}, false
}

// SelfView is the bot's own ship and account: everything on the [I]nfo screen
// plus a few derived counts a Brain needs to size its actions.
type SelfView struct {
	Name        string
	Sector      int
	Money       int
	Bank        int
	Turns       int // daily turns remaining
	BankedTurns int // Fusion Cell reserve, drawn after Turns hits zero
	Holds       int
	FreeHolds   int
	Fighters    int
	Mines       int
	Cargo       map[string]int // ore/organics/equipment in holds
	Devices     map[string]int // device name -> count owned
	SensorsOK   bool           // sensors undamaged (scans allowed)
	EnginesOK   bool
	Dead        bool
	Value       int
}

// HasTurns reports whether the bot can still take a turn-spending action.
func (s SelfView) HasTurns() bool { return s.Turns > 0 || s.BankedTurns > 0 }

// View is the complete, legitimately-knowable snapshot handed to a Brain at
// each decision point. The harness builds it under one Universe.Do, folds its
// perception into the bot's Memory, then calls Plan. Brains are pure functions
// of a View: no engine access, no screen text - which makes them unit-testable
// in isolation.
type View struct {
	Self       SelfView
	Here       SectorView         // current sector: always fully visible
	Scan       map[int]SectorView // adjacent sectors, populated only if sensors work
	OwnPlanets []PlanetView
	OwnForces  []galwar.Force
	Rankings   []galwar.Ranking

	Day        int
	NumSectors int // universe size (public: shown on the [U] stats screen)
	SimNow     time.Time
	Mem        *Memory    // the bot's accumulated knowledge (routing, port catalog, threats)
	Rand       *rand.Rand // the bot's own deterministic RNG
	Prices     Prices     // snapshotted cost constants (Brains never touch the universe)
}

// Prices is a snapshot of the tunable cost constants a Brain needs to size its
// spending. Copied out of config under Do so Brains never reach into the
// universe. Values mirror the SeedDefaultConfig keys.
type Prices struct {
	Hold        int
	Fighter     int
	Mine        int
	Genesis     int
	Scanner     int
	TurnsPerDay int
	TargetFloor int // faction_target_floor: no faction targets a player below this
	StartingKit int // approximate value of a fresh starter ship
}
