package galwar

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
		})
		snapCommodities(&s.commodities, "player", string(p.Id), p.Inventory)
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
