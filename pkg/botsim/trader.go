package botsim

import (
	"math/rand"
	"strconv"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// Trader is the base bot: it finds a pair of complementary ports and shuttles
// goods between them, reinvesting profits into holds and fighters, exploring
// when it has no route yet, and dodging obvious threats. Every other class
// embeds it and layers extra behavior on top (PLAN-BOTS.md 3.1).
type Trader struct {
	name  string
	class string
	rng   *rand.Rand

	// trade pair (sector numbers of the two ports), 0 when none chosen yet
	pairA, pairB int

	// current goal: travel to dest and do job there
	dest int
	job  string // "dock", "sol", "explore", "" (needs a new goal)

	// bookkeeping
	trips           int         // completed dockings
	lastReinvest    int         // trips value at the last Sol reinvestment
	lastDockMoney   int         // money at the previous dock (-1 = recalibrate, don't judge)
	poorTrips       int         // consecutive low-profit docks (depletion detector)
	coolUntil       map[int]int // port sector -> day it becomes worth trading again
	lastScan        int         // sector last scanned, so we scan new ground once
	fighterFloor    int
	reinvestEvery   int
	reinvestReserve int
	holdPct         int // share of reinvestment spent on holds vs fighters (rest -> fighters)
	blindJumpPct    int // % chance of a blind autopilot jump when the frontier is walled off
}

// portCooldownDays is how long a drained pair rests before findPair will pick
// its ports again. Restock is 2*Prod/day toward a 10*Prod cap, so a few days
// rebuilds enough stock to be worth a return visit.
const portCooldownDays = 3

// poorTripsBeforeRepair is how many consecutive low-profit docks (both ends of
// a depleted pair count) trigger abandoning the pair for a fresh one.
const poorTripsBeforeRepair = 3

// NewTrader builds a Trader brain.
func NewTrader(name, class string, rng *rand.Rand) *Trader {
	return &Trader{
		name:            name,
		class:           class,
		rng:             rng,
		coolUntil:       map[int]int{},
		fighterFloor:    50,
		reinvestEvery:   4,
		reinvestReserve: 15000,
		holdPct:         80,
		blindJumpPct:    12,
	}
}

func (t *Trader) Name() string  { return t.name }
func (t *Trader) Class() string { return t.class }

// Plan is the base decision loop, shared by every class. Priorities: survive,
// then pursue the current goal (choosing one if needed), then travel toward it.
func (t *Trader) Plan(v *View) Action {
	if !v.Self.HasTurns() {
		return pass()
	}

	// survival reflex: a hostile force in this sector - slip to a safer neighbor
	if a, ok := t.flee(v); ok {
		return a
	}
	// low on fighters after a scrape: restock at Sol - but only if we can pay
	// for a meaningful number, else keep trading to earn the credits first
	// (chasing Sol while broke is a wasted day, and buying zero is a free-action
	// loop that trips the action cap).
	if v.Self.Fighters < t.fighterFloor && v.Self.Money > 3000+10*v.Prices.Fighter {
		if a, ok := t.restockFighters(v); ok {
			return a
		}
	}
	// surplus built up: divert to Sol to reinvest in holds and fighters. This
	// runs even mid-trade-loop, where chooseGoal never fires (job stays "dock").
	if t.reinvestDue(v) && t.job != "sol" {
		if a, ok := t.headForSol(v); ok && len(a.Tokens) > 0 {
			return a // already at Sol: reinvest immediately
		}
		// otherwise headForSol set dest/job to Sol; fall through to travel there
	}
	if t.job == "" || t.dest == 0 {
		t.chooseGoal(v)
	}
	if v.Self.Sector == t.dest {
		if a, ok := t.doJob(v); ok {
			return a
		}
		// the goal didn't pan out (stale memory); pick another and move
		t.job, t.dest = "", 0
		t.chooseGoal(v)
	}
	return t.travel(v)
}

// reinvestDue reports whether the bot has traded enough round trips and holds
// enough surplus to justify a detour to Sol to grow its ship.
func (t *Trader) reinvestDue(v *View) bool {
	return t.trips-t.lastReinvest >= t.reinvestEvery && v.Self.Money > t.reinvestReserve
}

// chooseGoal sets t.dest/t.job for this decision cycle.
func (t *Trader) chooseGoal(v *View) {
	cur := v.Self.Sector

	// have a pair? head to whichever end we're not at
	if t.pairA != 0 && t.pairB != 0 {
		target := t.pairA
		if cur == t.pairA {
			target = t.pairB
		}
		t.dest, t.job = target, "dock"
		return
	}

	// try to form a pair from what we know
	if a, b, ok := t.findPair(v); ok {
		t.pairA, t.pairB = a, b
		target := a
		if t.routeLen(v, cur, b) < t.routeLen(v, cur, a) {
			target = b
		}
		t.dest, t.job = target, "dock"
		return
	}

	// nothing reachable to trade yet: explore. travel() walks the frontier one
	// safe hop at a time, and chooseGoal re-runs each step (dest stays 0) so the
	// bot starts trading the moment its growing map connects a viable pair.
	t.dest, t.job = 0, "explore"
}

// doJob performs the action for the current goal at the destination. ok=false
// means the goal was stale (e.g. no port here after all) and the caller should
// re-choose.
func (t *Trader) doJob(v *View) (Action, bool) {
	switch t.job {
	case "dock":
		port, ok := v.Here.Port()
		if !ok || !port.IsTrading() {
			return Action{}, false
		}
		a := t.dockTrade(v, port)
		// having traded here, head to the other end of the pair next
		if t.pairA != 0 && t.pairB != 0 {
			if t.dest == t.pairA {
				t.dest = t.pairB
			} else {
				t.dest = t.pairA
			}
		} else {
			t.job, t.dest = "", 0
		}
		return a, true
	case "sol":
		port, ok := v.Here.Port()
		if !ok || port.Kind != galwar.Sol {
			return Action{}, false
		}
		t.lastReinvest = t.trips
		t.job, t.dest = "", 0 // resume trading afterwards
		return t.solReinvest(v, port), true
	case "explore":
		t.job, t.dest = "", 0 // scan once, then choose a new frontier next cycle
		if !v.Self.SensorsOK {
			return Action{}, false // blind: just move on
		}
		return act("scan", "s"), true
	}
	return Action{}, false
}

// dockTrade sells everything the port buys (our inbound cargo) and buys a full
// load of the one good it sells (our outbound cargo). A trading port has
// exactly three goods, so the dialog is three defaulted answers: sell the two
// it buys, buy the one it sells.
func (t *Trader) dockTrade(v *View, port PortView) Action {
	tokens := []string{"p"}
	sell := port.SellsGood()
	for _, g := range tradeGoods { // loop 1: goods the port buys, in inventory order
		if g == sell {
			continue
		}
		tokens = append(tokens, "") // sell all we carry (engine default)
	}
	tokens = append(tokens, "") // loop 2: buy a full load of the good it sells

	detail := map[string]any{"port": port.Name, "sells": sell, "money_before": v.Self.Money}
	if t.lastDockMoney >= 0 && t.trips > 0 {
		net := v.Self.Money - t.lastDockMoney
		detail["net_since_prev"] = net
		// Depletion detector: a healthy round-trip leg nets a few credits per
		// hold; a picked-over port pays pennies. Ports restock far slower than
		// a growing ship drains them, so milking a drained pair wastes the
		// day - after a few consecutive poor legs, rest these ports and find a
		// fresh pair (the re-pair rule, PLAN-BOTS 3.1).
		if net < max1(2*v.Self.Holds) {
			t.poorTrips++
			if t.poorTrips >= poorTripsBeforeRepair && t.pairA != 0 {
				t.coolUntil[t.pairA] = v.Day + portCooldownDays
				t.coolUntil[t.pairB] = v.Day + portCooldownDays
				detail["repaired"] = true
				t.pairA, t.pairB = 0, 0
				t.job, t.dest = "", 0
				t.poorTrips = 0
			}
		} else {
			t.poorTrips = 0
		}
	}
	t.lastDockMoney = v.Self.Money
	t.trips++
	return act("trade", tokens...).withDetail(detail)
}

// solReinvest buys holds and fighters at Sol in an 80/20 split of the surplus.
func (t *Trader) solReinvest(v *View, port PortView) Action {
	surplus := v.Self.Money - t.reinvestReserve
	if surplus < 0 {
		surplus = 0
	}
	holdQty := (surplus * t.holdPct / 100) / max1(v.Prices.Hold)
	ftrQty := (surplus * (100 - t.holdPct) / 100) / max1(v.Prices.Fighter)

	tokens := []string{"p"}
	if idx := port.MenuIndex(galwar.HOLDS); idx > 0 && holdQty > 0 {
		tokens = append(tokens, strconv.Itoa(idx), strconv.Itoa(holdQty))
	}
	if idx := port.MenuIndex(galwar.FIGHTERS); idx > 0 && ftrQty > 0 {
		tokens = append(tokens, strconv.Itoa(idx), strconv.Itoa(ftrQty))
	}
	tokens = append(tokens, "q")
	// spending here would read as a catastrophic "loss" at the next dock;
	// recalibrate the profit tracker instead of judging it
	t.lastDockMoney = -1
	return act("reinvest", tokens...).withDetail(map[string]any{"holds": holdQty, "fighters": ftrQty})
}

// travel advances the current goal by one careful hop. It moves sector by
// sector over scanned, threat-checked warps - both toward a trade destination
// and while exploring - so the bot sees each sector before entering it. A blind
// autopilot jump is a last resort, taken only occasionally to escape a pocket
// the known map has walled off.
func (t *Trader) travel(v *View) Action {
	cur := v.Self.Sector
	// look before you leap: sweep new ground with sensors once before moving on,
	// so the next hop is informed rather than a step into the dark
	if v.Self.SensorsOK && cur != t.lastScan && t.hasUnvisitedNeighbor(v) {
		t.lastScan = cur
		return act("scan", "s")
	}
	// step toward a reachable goal over known-safe warps
	if t.dest != 0 && t.dest != cur {
		if route := v.Mem.SafeRoute(cur, t.dest, v.Day); len(route) >= 2 {
			return t.step(route[1])
		}
	}
	// no safe route to the goal: push the frontier outward one hop, which
	// reveals new sectors and their ports as we go
	if f := v.Mem.Frontier(cur, v.Day); f != 0 {
		if route := v.Mem.SafeRoute(cur, f, v.Day); len(route) >= 2 {
			return t.step(route[1])
		}
	}
	// the known frontier is exhausted or threat-walled: occasionally let the
	// ship's computer plot a blind long jump to break out; otherwise poke a
	// safe neighbor and keep mapping locally
	if t.rng.Intn(100) < t.blindJumpPct {
		if dest := t.blindJumpTarget(v); dest != 0 {
			t.dest, t.job = 0, "" // a blind jump abandons the current goal
			return act("autopilot", "y", strconv.Itoa(dest), "y").
				withDetail(map[string]any{"to": dest, "blind": true})
		}
	}
	return t.step(t.safeWarp(v))
}

// step is a single-sector move (the [M] command), the bot's default mode of
// travel now that it navigates hop by hop.
func (t *Trader) step(sector int) Action {
	return act("move", "m", strconv.Itoa(sector))
}

// moveToward advances one hop toward a specific known destination (used by
// subclasses to reach the bank, Sol, or a colony). It steps over known-safe
// warps and, only if it has no mapped route at all, falls back to the ship's
// autopilot - these targets are places the bot has already visited, so a route
// almost always exists.
func (t *Trader) moveToward(v *View, dest int) Action {
	cur := v.Self.Sector
	if dest == 0 || dest == cur {
		return t.step(t.safeWarp(v))
	}
	if route := v.Mem.SafeRoute(cur, dest, v.Day); len(route) >= 2 {
		return t.step(route[1])
	}
	return act("autopilot", "y", strconv.Itoa(dest), "y").
		withDetail(map[string]any{"to": dest})
}

// hasUnvisitedNeighbor reports whether the current sector warps to somewhere the
// bot has never stood - i.e. it is on the frontier, worth a sensor sweep.
func (t *Trader) hasUnvisitedNeighbor(v *View) bool {
	for _, w := range v.Here.Warps {
		if !v.Mem.Visited(w) {
			return true
		}
	}
	return false
}

// safeWarp picks a warp out of the current sector that has no known fresh
// threat, preferring unvisited ones to keep exploration moving; falls back to
// any warp.
func (t *Trader) safeWarp(v *View) int {
	warps := v.Here.Warps
	var safe, safeNew []int
	for _, w := range warps {
		if v.Mem.Blocked(w, v.Day) {
			continue
		}
		safe = append(safe, w)
		if !v.Mem.Visited(w) {
			safeNew = append(safeNew, w)
		}
	}
	switch {
	case len(safeNew) > 0:
		return safeNew[t.rng.Intn(len(safeNew))]
	case len(safe) > 0:
		return safe[t.rng.Intn(len(safe))]
	case len(warps) > 0:
		return warps[t.rng.Intn(len(warps))]
	}
	return v.Self.Sector
}

// blindJumpTarget picks a far, valid sector for a rare autopilot escape.
func (t *Trader) blindJumpTarget(v *View) int {
	for tries := 0; tries < 8; tries++ {
		s := 1 + t.rng.Intn(maxInt(v.NumSectors, 1))
		if s != v.Self.Sector && !v.Mem.Visited(s) {
			return s
		}
	}
	return 0
}

// flee moves to the safest neighbor when a hostile force shares this sector.
// Federation space is exempt: combat is illegal in sectors 1-10 (the engine
// enforces it), so a "stronger ship" at Sol is just another customer - fleeing
// from it burns turns for nothing.
func (t *Trader) flee(v *View) (Action, bool) {
	if v.Self.Sector <= 10 {
		return Action{}, false
	}
	threatHere := false
	for _, bg := range v.Here.Battlegroups {
		if !bg.Mine && bg.Fighters > 0 {
			threatHere = true
		}
	}
	for _, sh := range v.Here.Ships {
		if sh.Value > v.Self.Value*3/2 {
			threatHere = true
		}
	}
	if !threatHere {
		return Action{}, false
	}
	// pick an adjacent sector without a known fresh threat
	warps := v.Here.Warps
	best := 0
	for _, w := range warps {
		if !v.Mem.Blocked(w, v.Day) {
			best = w
			break
		}
	}
	if best == 0 && len(warps) > 0 {
		best = warps[t.rng.Intn(len(warps))]
	}
	if best == 0 {
		return Action{}, false
	}
	t.job, t.dest = "", 0 // abandon the current goal; re-plan after fleeing
	return act("flee", "m", strconv.Itoa(best)).withDetail(map[string]any{"from": v.Self.Sector, "to": best}), true
}

// headForSol sets a goal to reach Sol for reinvestment. If already at Sol it
// returns the reinvest action directly (ok, non-empty tokens); otherwise it
// sets the goal and returns ok with no tokens, so the caller travels there.
func (t *Trader) headForSol(v *View) (Action, bool) {
	sol := v.Mem.NearestServicePort(v.Self.Sector, galwar.Sol)
	if sol == 0 {
		return Action{}, false
	}
	if v.Self.Sector == sol {
		if port, ok := v.Here.Port(); ok && port.Kind == galwar.Sol {
			t.lastReinvest = t.trips
			return t.solReinvest(v, port), true
		}
		return Action{}, false
	}
	t.dest, t.job = sol, "sol"
	return Action{}, true
}

// restockFighters routes to Sol and buys fighters up to twice the floor,
// bounded by what the bot can afford while keeping a trading reserve. Returns
// ok=false (fall through to trading) when Sol is unknown or nothing is
// affordable, so the bot never loops buying zero.
func (t *Trader) restockFighters(v *View) (Action, bool) {
	sol := v.Mem.NearestServicePort(v.Self.Sector, galwar.Sol)
	if sol == 0 {
		return Action{}, false
	}
	if v.Self.Sector != sol {
		return t.moveToward(v, sol), true
	}
	port, ok := v.Here.Port()
	if !ok || port.Kind != galwar.Sol {
		return Action{}, false
	}
	idx := port.MenuIndex(galwar.FIGHTERS)
	if idx == 0 {
		return Action{}, false
	}
	want := 2*t.fighterFloor - v.Self.Fighters
	afford := (v.Self.Money - 3000) / max1(v.Prices.Fighter)
	qty := want
	if qty > afford {
		qty = afford
	}
	if qty <= 0 {
		return Action{}, false // can't afford: go trade to earn first
	}
	t.lastDockMoney = -1 // spending, not trading: recalibrate the profit tracker
	return act("reinvest", "p", strconv.Itoa(idx), strconv.Itoa(qty), "q").
		withDetail(map[string]any{"fighters": qty, "restock": true}), true
}

// findPair looks for two remembered trading ports that sell different goods -
// each buys what the other sells, a profitable round trip. It only considers
// ports the bot can already reach over its own known-safe map, so the resulting
// trade route is always walkable hop by hop: the bot explores to grow that map
// rather than committing to a pair it would have to blind-jump to. Shorter,
// safer routes score best.
func (t *Trader) findPair(v *View) (a, b int, ok bool) {
	ports := v.Mem.TradingPorts()
	cur := v.Self.Sector

	// precompute each usable port's safe hop-distance from here; ports with no
	// safe route are out of reach until exploration connects them. Two ports
	// both reachable from `cur` are reachable from each other (via cur at
	// worst), so this doubles as the shuttle-route check.
	dist := make(map[int]int, len(ports))
	usable := ports[:0]
	for _, p := range ports {
		if p.SellsGood() == "" || t.coolUntil[p.Sector] > v.Day {
			continue
		}
		if r := v.Mem.SafeRoute(cur, p.Sector, v.Day); r != nil {
			dist[p.Sector] = len(r) - 1
			usable = append(usable, p)
		}
	}

	bestScore := 1 << 30
	for i := 0; i < len(usable); i++ {
		for j := i + 1; j < len(usable); j++ {
			if usable[i].SellsGood() == usable[j].SellsGood() {
				continue
			}
			score := dist[usable[i].Sector] + dist[usable[j].Sector]
			if score < bestScore {
				bestScore = score
				a, b, ok = usable[i].Sector, usable[j].Sector, true
			}
		}
	}
	return a, b, ok
}

// routeLen is the known-route hop count, or a large finite penalty when the bot
// hasn't mapped a route there (used only to pick the nearer end of a pair it
// already knows it can reach).
func (t *Trader) routeLen(v *View, from, to int) int {
	if from == to {
		return 0
	}
	if r := v.Mem.Route(from, to); r != nil {
		return len(r) - 1
	}
	return 1000 + abs(to-from)
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
