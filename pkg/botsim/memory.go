package botsim

import (
	"sort"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// Memory is a bot's private, accumulated knowledge of the world - everything it
// has learned by playing, and nothing it hasn't. The harness folds each turn's
// perception into it (Observe) as a side effect of the bot's own actions; the
// Brain routes and plans over it. No bot starts with a map (unless -knownmap
// pre-seeds one for economy experiments).
type Memory struct {
	warps   map[int][]int       // sector -> warps personally seen from it
	visited map[int]bool        // sectors the bot has stood in
	ports   map[int]*PortMemo   // sector -> remembered port
	threats map[int]*threatMemo // sector -> most recent hostile battlegroup sighting

	// staleThreatDays is how long a battlegroup sighting blocks routing before
	// it decays to mere rumor (groups get destroyed, reinforced, or inherited).
	staleThreatDays int
}

// PortMemo is a remembered port. Full is true once the bot has stood in the
// port's sector and read its commerce report (what it buys, its stock);
// otherwise only the sells-headline is known, from a scan.
type PortMemo struct {
	Sector  int
	Name    string
	Kind    galwar.PortType
	Sells   map[string]bool
	Buys    map[string]bool
	Stock   map[string]int
	Full    bool
	Docked  bool
	SeenDay int
}

// SellsGood returns the single trade good this port sells, or "".
func (p *PortMemo) SellsGood() string {
	for _, g := range tradeGoods {
		if p.Sells[g] {
			return g
		}
	}
	return ""
}

// IsTrading reports a regular commodity port.
func (p *PortMemo) IsTrading() bool { return p.Kind == galwar.TradingPort }

type threatMemo struct {
	Sector   int
	Owner    galwar.PlayerId
	Fighters int
	Mines    int
	Day      int
}

// NewMemory builds an empty memory. staleDays is how many days a battlegroup
// sighting keeps blocking routes before decaying to rumor.
func NewMemory(staleDays int) *Memory {
	return &Memory{
		warps:           map[int][]int{},
		visited:         map[int]bool{},
		ports:           map[int]*PortMemo{},
		threats:         map[int]*threatMemo{},
		staleThreatDays: staleDays,
	}
}

// Observe folds one sector's perception into memory. self is the bot's own id,
// so its own battlegroups aren't recorded as threats. full says whether the
// port commerce in sv is ground truth (the current sector) or headline-only (a
// scanned neighbor).
func (m *Memory) Observe(sv SectorView, self galwar.PlayerId, day int, full bool) {
	if sv.Sector <= 0 {
		return
	}
	if len(sv.Warps) > 0 {
		m.warps[sv.Sector] = append([]int(nil), sv.Warps...)
	}
	for _, pv := range sv.Ports {
		m.recordPort(pv, day, full)
	}
	// refresh (or clear) the threat record for this sector from what we see now.
	// A minefield (mines, no fighters) is just as deadly on entry as a fighter
	// group, so both count.
	var worst *threatMemo
	for _, bg := range sv.Battlegroups {
		if bg.Mine || bg.Owner == self || bg.Fighters+bg.Mines <= 0 {
			continue
		}
		if worst == nil || bg.Fighters+bg.Mines > worst.Fighters+worst.Mines {
			worst = &threatMemo{Sector: sv.Sector, Owner: bg.Owner, Fighters: bg.Fighters, Mines: bg.Mines, Day: day}
		}
	}
	if worst != nil {
		m.threats[sv.Sector] = worst
	} else {
		// we can see the sector and there is no hostile group: clear any stale record
		delete(m.threats, sv.Sector)
	}
}

// MarkVisited records that the bot has stood in a sector (first-hand commerce).
func (m *Memory) MarkVisited(sector int) {
	if sector > 0 {
		m.visited[sector] = true
	}
}

func (m *Memory) recordPort(pv PortView, day int, full bool) {
	existing := m.ports[pv.Sector]
	pm := &PortMemo{
		Sector:  pv.Sector,
		Name:    pv.Name,
		Kind:    pv.Kind,
		Sells:   copyBoolMap(pv.Sells),
		Buys:    copyBoolMap(pv.Buys),
		Stock:   copyIntMap(pv.Stock),
		Full:    pv.Full,
		SeenDay: day,
	}
	// don't let a headline-only re-sighting erase full commerce we already have
	if existing != nil && existing.Full && !pv.Full {
		existing.SeenDay = day
		if len(pv.Sells) > 0 {
			existing.Sells = copyBoolMap(pv.Sells)
		}
		return
	}
	if full && pv.Full {
		pm.Docked = true
	}
	m.ports[pv.Sector] = pm
}

// TradingPorts returns every remembered trading port, sorted by sector so
// iteration order is stable (deterministic pair selection across runs).
func (m *Memory) TradingPorts() []*PortMemo {
	var out []*PortMemo
	for _, p := range m.ports {
		if p.IsTrading() {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Sector < out[j].Sector })
	return out
}

// Port returns the remembered port in a sector, or nil.
func (m *Memory) Port(sector int) *PortMemo { return m.ports[sector] }

// PortSector returns the sector of the nearest remembered service port of a
// given kind (Sol, the bank, the device shop), by known-warp distance from
// `from`, or 0 if none is known/reachable over the bot's own map.
func (m *Memory) NearestServicePort(from int, kind galwar.PortType) int {
	best, bestDist := 0, 1<<30
	for _, p := range m.ports {
		if p.Kind != kind {
			continue
		}
		if path := m.Route(from, p.Sector); path != nil && len(path)-1 < bestDist {
			best, bestDist = p.Sector, len(path)-1
		}
	}
	return best
}

// Visited reports whether the bot has stood in a sector.
func (m *Memory) Visited(sector int) bool { return m.visited[sector] }

// Blocked reports whether a sector holds a hostile battlegroup sighting still
// fresh enough to route around. Older sightings decay to rumor and stop
// blocking (they only bias routing, handled by the caller).
func (m *Memory) Blocked(sector, day int) bool {
	t := m.threats[sector]
	if t == nil || t.Fighters+t.Mines <= 0 {
		return false
	}
	return day-t.Day <= m.staleThreatDays
}

// Threat returns the remembered threat in a sector, or nil.
func (m *Memory) Threat(sector int) (owner galwar.PlayerId, fighters int, ok bool) {
	t := m.threats[sector]
	if t == nil {
		return "", 0, false
	}
	return t.Owner, t.Fighters, true
}

// Route finds a shortest path over the bot's OWN known warps from -> to,
// inclusive of both ends, or nil if the bot hasn't mapped a connection. Blocked
// sectors are avoided when avoid is true (except the destination itself).
func (m *Memory) Route(from, to int) []int {
	return m.route(from, to, false, 0)
}

// SafeRoute is Route that detours around fresh threats; falls back to Route if
// no safe path exists over known warps.
func (m *Memory) SafeRoute(from, to, day int) []int {
	if p := m.route(from, to, true, day); p != nil {
		return p
	}
	return m.route(from, to, false, 0)
}

func (m *Memory) route(from, to int, avoid bool, day int) []int {
	if from == to {
		return []int{from}
	}
	prev := map[int]int{from: from}
	queue := []int{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, w := range m.warps[cur] {
			if _, seen := prev[w]; seen {
				continue
			}
			if avoid && w != to && m.Blocked(w, day) {
				continue
			}
			prev[w] = cur
			if w == to {
				return backtrack(prev, from, to)
			}
			queue = append(queue, w)
		}
	}
	return nil
}

func backtrack(prev map[int]int, from, to int) []int {
	var rev []int
	for at := to; ; at = prev[at] {
		rev = append(rev, at)
		if at == from {
			break
		}
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// Frontier picks the nearest sector the bot has seen a warp to but never
// entered, over its own known map, for directed exploration. Returns 0 when the
// known frontier is exhausted (the bot should blind-jump). day and avoid steer
// exploration away from fresh threats.
func (m *Memory) Frontier(from, day int) int {
	prev := map[int]int{from: from}
	queue := []int{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, w := range m.warps[cur] {
			if !m.visited[w] {
				return w // nearest unvisited known sector
			}
			if _, seen := prev[w]; seen {
				continue
			}
			if m.Blocked(w, day) {
				continue
			}
			prev[w] = cur
			queue = append(queue, w)
		}
	}
	return 0
}

func copyBoolMap(m map[string]bool) map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyIntMap(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
