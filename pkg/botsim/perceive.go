package botsim

import (
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// perceive builds the View a Brain decides from, under a single Universe.Do,
// and folds what the bot sees into its Memory. This is the sole perception
// boundary: everything a bot may legitimately know is gathered here from engine
// state (respecting cloak/dormancy visibility), never from the transcript.
func perceive(b *bot, day int, simNow time.Time) *View {
	u := b.sim.u
	v := &View{
		Day:    day,
		SimNow: simNow,
		Mem:    b.mem,
		Rand:   b.rng,
		Prices: b.sim.prices,
		Scan:   map[int]SectorView{},
	}
	u.Do(func() {
		p := b.player
		v.NumSectors = len(u.Sectors) - 1
		v.Self = selfView(u, p)
		v.Here = sectorView(u, p, p.Sector, simNow, true)

		// scans reveal adjacent sectors, but only with working sensors - a bot
		// with a damaged sensor array is blind, exactly as at the [S] command
		if v.Self.SensorsOK {
			for _, w := range u.Sectors[p.Sector].GetWarps() {
				v.Scan[w] = sectorView(u, p, w, simNow, false)
			}
		}

		for _, pl := range u.OwnedPlanets(p) {
			v.OwnPlanets = append(v.OwnPlanets, planetView(pl, p, true))
		}
		v.OwnForces = u.PlayerForces(p)
		v.Rankings = u.RankedPlayers(simNow)
	})

	// fold perception into memory: the current sector is ground truth (full
	// commerce), scanned neighbors are headline-only leads.
	b.mem.MarkVisited(v.Self.Sector)
	b.mem.Observe(v.Here, b.player.Id, day, true)
	for _, sv := range v.Scan {
		b.mem.Observe(sv, b.player.Id, day, false)
	}
	return v
}

func selfView(u *galwar.UniverseType, p *galwar.Player) SelfView {
	s := SelfView{
		Name:        p.GetName(),
		Sector:      p.Sector,
		Money:       p.GetMoney(),
		Bank:        p.BankBalance,
		Turns:       p.GetQuantity(galwar.TURNS),
		BankedTurns: p.BankedTurns,
		Holds:       p.GetQuantity(galwar.HOLDS),
		FreeHolds:   p.GetFreeHolds(),
		Fighters:    p.GetQuantity(galwar.FIGHTERS),
		Mines:       p.GetQuantity(galwar.MINES),
		Cargo:       map[string]int{},
		Devices:     map[string]int{},
		SensorsOK:   p.Systems[galwar.SysSensors] == 0,
		EnginesOK:   p.Systems[galwar.SysEngines] == 0,
		Dead:        p.IsDead(),
		Value:       u.PlayerValue(p),
	}
	for _, g := range tradeGoods {
		s.Cargo[g] = p.GetQuantity(g)
	}
	for _, name := range deviceNames {
		if q := p.GetQuantity(name); q > 0 {
			s.Devices[name] = q
		}
	}
	return s
}

// deviceNames are the non-cargo carryables a bot tracks for buying/using.
var deviceNames = []string{
	galwar.GENESIS, galwar.PLASMA, galwar.PULSAR, galwar.EMWARP,
	galwar.CLOAK, galwar.ANTICLOAK, galwar.PULSARTUBE,
	galwar.FUSIONCELL, galwar.PLANETSCANNER, galwar.MINEDEFLECTOR,
}

// sectorView snapshots a sector as viewer p may see it. full=true exposes port
// commerce (the current sector); full=false is a scan sighting: only the
// sells-headline of any port, no buy list or stock.
func sectorView(u *galwar.UniverseType, p *galwar.Player, sector int, now time.Time, full bool) SectorView {
	sv := SectorView{Sector: sector, Warps: append([]int(nil), u.Sectors[sector].GetWarps()...)}
	for _, obj := range u.GetVisibleObjectsInSector(sector, "", p, now) {
		switch o := obj.(type) {
		case *galwar.Port:
			sv.Ports = append(sv.Ports, portView(p, o, full))
		case *galwar.Planet:
			sv.Planets = append(sv.Planets, planetView(o, p, p.HasPlanetScanner()))
		case *galwar.Player:
			if o == p {
				continue
			}
			sv.Ships = append(sv.Ships, ShipView{Id: o.Id, Name: o.GetName(), Value: u.PlayerValue(o)})
		case *galwar.Battlegroup:
			sv.Battlegroups = append(sv.Battlegroups, BattlegroupView{
				Sector:    sector,
				Owner:     o.Owner,
				OwnerName: o.GetOwnerPlayer().GetName(),
				Mine:      o.Owner == p.Id,
				Fighters:  o.GetQuantity(galwar.FIGHTERS),
				Mines:     o.GetQuantity(galwar.MINES),
			})
		}
	}
	return sv
}

func portView(p *galwar.Player, port *galwar.Port, full bool) PortView {
	pv := PortView{
		Sector: port.Sector,
		Name:   port.GetName(),
		Kind:   port.Goods,
		Sells:  map[string]bool{},
		Buys:   map[string]bool{},
		Stock:  map[string]int{},
	}
	for _, cm := range port.Inventory {
		if cm.Sell {
			pv.Sells[cm.Name] = true
		}
	}
	if !full {
		return pv // scan sighting: headline (what it sells) only
	}
	pv.Full = true
	for _, cm := range port.Inventory {
		pv.Menu = append(pv.Menu, cm.Name)
		if !cm.Sell {
			pv.Buys[cm.Name] = true
		}
		pv.Stock[cm.Name] = galwar.ScaleUp(p, cm.Quantity)
	}
	return pv
}

func planetView(pl *galwar.Planet, viewer *galwar.Player, knownDef bool) PlanetView {
	mine := pl.Owner == viewer.Id
	pv := PlanetView{
		Sector:       pl.Sector,
		Name:         pl.GetName(),
		Owner:        pl.Owner,
		OwnerName:    pl.GetOwnerPlayer().GetName(),
		Mine:         mine,
		KnownDefense: mine || knownDef,
	}
	if pv.KnownDefense {
		pv.Fighters = pl.GetQuantity(galwar.FIGHTERS)
		pv.Mines = pl.GetQuantity(galwar.MINES)
	}
	return pv
}
