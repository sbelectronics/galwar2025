package galwar

import (
	"strconv"
	"strings"
)

// A Snapshot is a flat, row-oriented copy of the universe, cheap to build on
// the actor goroutine and safe to write to SQLite from another goroutine
// afterward. The in-memory universe remains authoritative; the snapshot is
// only a courier between the actor and the store.

type playerRow struct {
	id        string
	email     string
	name      string
	sector    int
	money     int
	googleSub string
	passHash  string
	lastSeen  int64
	timesDied int
	diedAt    int64
	systems   string
	banned    bool
	expired   bool
}

type newsRow struct {
	playerID  string
	at        int64
	msg       string
	delivered bool
}

type reportRow struct {
	reporter string
	target   string
	reason   string
	at       int64
	resolved bool
}

type auditRow struct {
	at     int64
	actor  string
	action string
	detail string
}

type portRow struct {
	idx    int
	name   string
	sector int
	goods  int
	money  int
}

type planetRow struct {
	idx    int
	name   string
	sector int
	owner  string
	money  int
}

type battlegroupRow struct {
	idx    int
	name   string
	sector int
	owner  string
	money  int
}

type commodityRow struct {
	ownerType   string
	ownerID     string
	pos         int
	name        string
	prod        int
	quantity    int
	buyPrice    float64
	sellPrice   float64
	sell        bool
	lastRestock int64
}

type Snapshot struct {
	sectors      []int    // sector numbers
	warps        [][2]int // from, to
	players      []playerRow
	ports        []portRow
	planets      []planetRow
	battlegroups []battlegroupRow
	commodities  []commodityRow
	config       map[string]string
	news         []newsRow
	reports      []reportRow
	audit        []auditRow
}

// systemsToString / systemsFromString serialize the per-system damage
// counters as a comma-joined list ("" = undamaged).
func systemsToString(systems []int) string {
	if len(systems) == 0 {
		return ""
	}
	parts := make([]string, len(systems))
	for i, d := range systems {
		parts[i] = itoa(d)
	}
	return strings.Join(parts, ",")
}

func systemsFromString(s string) []int {
	systems := make([]int, NumSystems)
	if s == "" {
		return systems
	}
	for i, part := range strings.Split(s, ",") {
		if i >= NumSystems {
			break
		}
		if n, err := strconv.Atoi(part); err == nil {
			systems[i] = n
		}
	}
	return systems
}

func snapCommodities(rows *[]commodityRow, ownerType string, ownerID string, inv []*Commodity) {
	for pos, c := range inv {
		*rows = append(*rows, commodityRow{
			ownerType:   ownerType,
			ownerID:     ownerID,
			pos:         pos,
			name:        c.Name,
			prod:        c.Prod,
			quantity:    c.Quantity,
			buyPrice:    c.BuyPrice,
			sellPrice:   c.SellPrice,
			sell:        c.Sell,
			lastRestock: c.LastRestock,
		})
	}
}

// Snapshot copies the universe into rows. Once Start has been called it must
// run on the universe actor (inside Do); before Start (initial save,
// migration, tests) calling it directly is fine.
func (u *UniverseType) Snapshot() *Snapshot {
	s := &Snapshot{
		config: map[string]string{},
	}

	for i := range u.Sectors {
		s.sectors = append(s.sectors, u.Sectors[i].Number)
		for _, w := range u.Sectors[i].Warps {
			s.warps = append(s.warps, [2]int{u.Sectors[i].Number, w})
		}
	}

	for _, p := range u.Players.Players {
		s.players = append(s.players, playerRow{
			id:        string(p.Id),
			email:     p.Email,
			name:      p.Name,
			sector:    p.Sector,
			money:     p.Money,
			googleSub: p.GoogleSub,
			passHash:  p.PassHash,
			lastSeen:  p.LastSeen,
			timesDied: p.TimesDied,
			diedAt:    p.DiedAt,
			systems:   systemsToString(p.Systems),
			banned:    p.Banned,
			expired:   p.Expired,
		})
		snapCommodities(&s.commodities, "player", string(p.Id), p.Inventory)
	}

	for _, n := range u.News {
		s.news = append(s.news, newsRow{
			playerID:  string(n.Player),
			at:        n.At,
			msg:       n.Msg,
			delivered: n.Delivered,
		})
	}

	for _, r := range u.Reports {
		s.reports = append(s.reports, reportRow{
			reporter: r.Reporter,
			target:   r.Target,
			reason:   r.Reason,
			at:       r.At,
			resolved: r.Resolved,
		})
	}

	for _, a := range u.Audit {
		s.audit = append(s.audit, auditRow{at: a.At, actor: a.Actor, action: a.Action, detail: a.Detail})
	}

	for i, p := range u.Ports.Ports {
		s.ports = append(s.ports, portRow{
			idx:    i,
			name:   p.Name,
			sector: p.Sector,
			goods:  int(p.Goods),
			money:  p.Money,
		})
		snapCommodities(&s.commodities, "port", itoa(i), p.Inventory)
	}

	for i, p := range u.Planets.Planets {
		s.planets = append(s.planets, planetRow{
			idx:    i,
			name:   p.Name,
			sector: p.Sector,
			owner:  string(p.Owner),
			money:  p.Money,
		})
		snapCommodities(&s.commodities, "planet", itoa(i), p.Inventory)
	}

	for i, b := range u.Battlegroups.Battlegroups {
		s.battlegroups = append(s.battlegroups, battlegroupRow{
			idx:    i,
			name:   b.Name,
			sector: b.Sector,
			owner:  string(b.Owner),
			money:  b.Money,
		})
		snapCommodities(&s.commodities, "battlegroup", itoa(i), b.Inventory)
	}

	for k, v := range u.Config {
		s.config[k] = v
	}

	return s
}
