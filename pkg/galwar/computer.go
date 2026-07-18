package galwar

import "time"

// The ship's computer (the C command) - read-only reports that make the big
// 2,000-sector world navigable and let a player manage assets without flying
// to each one. Descendants of the original's computer_menu (COMPUTER.PAS),
// pared to the functions that carry their weight. Everything here is a pure
// query over universe state; the UI (consoleui) formats it.

// Force is one owned asset in the "find your forces" report.
type Force struct {
	Kind     string // "Planet" or "Defense Force"
	Name     string
	Sector   int
	Fighters int
	Mines    int
}

// PlayerForces lists every planet and sector defense force the player owns,
// so they can find the fighters and mines they scattered across the galaxy.
func (u *UniverseType) PlayerForces(p *Player) []Force {
	var out []Force
	for _, pl := range u.Planets.Planets {
		if pl.Owner == p.Id {
			out = append(out, Force{
				Kind:     "Planet",
				Name:     pl.Name,
				Sector:   pl.Sector,
				Fighters: pl.GetQuantity(FIGHTERS),
				Mines:    pl.GetQuantity(MINES),
			})
		}
	}
	for _, bg := range u.Battlegroups.Battlegroups {
		if bg.Owner == p.Id {
			out = append(out, Force{
				Kind:     "Defense Force",
				Sector:   bg.Sector,
				Fighters: bg.GetQuantity(FIGHTERS),
				Mines:    bg.GetQuantity(MINES),
			})
		}
	}
	return out
}

// OwnedPlanets returns the player's planets, for the planetary-status report.
func (u *UniverseType) OwnedPlanets(p *Player) []*Planet {
	var out []*Planet
	for _, pl := range u.Planets.Planets {
		if pl.Owner == p.Id {
			out = append(out, pl)
		}
	}
	return out
}

// NearestPort finds the closest sector holding a port that satisfies want (a
// nil predicate matches any port), by shortest warp distance from `from`.
// Breadth-first, so the first hit is the nearest. found is false when no
// matching port is reachable.
func (u *UniverseType) NearestPort(from int, want func(*Port) bool) (sector, distance int, name string, found bool) {
	if from < 1 || from >= len(u.Sectors) {
		return 0, 0, "", false
	}
	portHere := func(sec int) (*Port, bool) {
		for _, p := range u.Ports.Ports {
			if p.Sector == sec && (want == nil || want(p)) {
				return p, true
			}
		}
		return nil, false
	}

	visited := make([]bool, len(u.Sectors))
	type node struct{ sec, dist int }
	queue := []node{{from, 0}}
	visited[from] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if p, ok := portHere(cur.sec); ok {
			return cur.sec, cur.dist, p.Name, true
		}
		for _, w := range u.Sectors[cur.sec].Warps {
			if w >= 1 && w < len(u.Sectors) && !visited[w] {
				visited[w] = true
				queue = append(queue, node{w, cur.dist + 1})
			}
		}
	}
	return 0, 0, "", false
}

// PortBuys / PortSells are predicates for NearestPort: a port that will buy
// (respectively sell) the named commodity from the player right now.
func PortBuys(name string) func(*Port) bool {
	return func(p *Port) bool {
		c := p.GetCommodity(name)
		return c != nil && !c.Sell && c.Quantity > 0
	}
}

func PortSells(name string) func(*Port) bool {
	return func(p *Port) bool {
		c := p.GetCommodity(name)
		return c != nil && c.Sell && c.Quantity > 0
	}
}

// UniverseStats is the "universe specifics" report - the state-of-the-galaxy
// summary, whose active-trader count doubles as a "the server is alive" signal
// in a game with no player-to-player chat.
type UniverseStats struct {
	Sectors       int
	Ports         int
	Planets       int
	ActiveTraders int // non-NPC, alive, not dormant
	TurnsPerDay   int
}

func (u *UniverseType) Stats(now time.Time) UniverseStats {
	s := UniverseStats{
		Sectors:     len(u.Sectors) - 1, // sector 0 is off-map parking
		Ports:       len(u.Ports.Ports),
		Planets:     len(u.Planets.Planets),
		TurnsPerDay: u.ConfigInt("turns_per_day", 250),
	}
	for _, p := range u.Players.Players {
		if p.IsNPC() || p.IsDead() || u.IsDormant(p, now) {
			continue
		}
		s.ActiveTraders++
	}
	return s
}

// RecentNews returns up to max of the player's most recent news items, oldest
// first, so the "what happened" report can re-show an overnight battle report
// the player already dismissed. Unlike TakeNews it changes nothing.
func (u *UniverseType) RecentNews(id PlayerId, max int) []string {
	var all []string
	for _, n := range u.News {
		if n.Player == id {
			all = append(all, n.Msg)
		}
	}
	if len(all) > max {
		all = all[len(all)-max:]
	}
	return all
}
