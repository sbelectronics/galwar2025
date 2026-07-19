package botsim

import (
	"math/rand"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// tradingPortView builds a full-commerce PortView that sells one good and buys
// the other two - the shape every generated trading port has.
func tradingPortView(sector int, sells string) PortView {
	pv := PortView{Sector: sector, Name: "Port" + sells, Kind: galwar.TradingPort,
		Sells: map[string]bool{}, Buys: map[string]bool{}, Stock: map[string]int{}, Full: true}
	for _, g := range tradeGoods {
		if g == sells {
			pv.Sells[g] = true
		} else {
			pv.Buys[g] = true
		}
		pv.Stock[g] = 5000
		pv.Menu = append(pv.Menu, g)
	}
	return pv
}

func baseView(sector int) *View {
	return &View{
		Self: SelfView{Sector: sector, Turns: 250, Money: 30000, Value: 67000,
			Fighters: 200, Holds: 25, FreeHolds: 25, SensorsOK: true, EnginesOK: true,
			Cargo: map[string]int{}, Devices: map[string]int{}},
		Here:       SectorView{Sector: sector, Warps: []int{sector + 1, sector + 2}},
		Scan:       map[int]SectorView{},
		Day:        1,
		NumSectors: 300,
		Mem:        NewMemory(5),
		Rand:       rand.New(rand.NewSource(1)),
		Prices:     Prices{Hold: 500, Fighter: 98},
	}
}

func TestTraderPassesWithNoTurns(t *testing.T) {
	tr := NewTrader("T", "trader", rand.New(rand.NewSource(1)))
	v := baseView(10)
	v.Self.Turns = 0
	v.Self.BankedTurns = 0
	if a := tr.Plan(v); !a.Pass {
		t.Errorf("Trader with no turns should Pass, got %+v", a)
	}
}

func TestTraderTradesAtAPairPort(t *testing.T) {
	tr := NewTrader("T", "trader", rand.New(rand.NewSource(1)))
	v := baseView(10)
	// the bot stands at an Ore-selling port and knows an Organics-selling port
	// one hop away over its mapped warps: a reachable pair, and it is at one
	// end, so it should trade here.
	here := tradingPortView(10, galwar.ORE)
	v.Here.Ports = []PortView{here}
	v.Mem.Observe(SectorView{Sector: 10, Warps: []int{20}, Ports: []PortView{here}}, "self", 1, true)
	v.Mem.MarkVisited(10)
	other := tradingPortView(20, galwar.ORGANICS)
	v.Mem.Observe(SectorView{Sector: 20, Warps: []int{10}, Ports: []PortView{other}}, "self", 1, false)
	v.Mem.MarkVisited(20)

	a := tr.Plan(v)
	if a.Kind != "trade" {
		t.Fatalf("at a pair port the Trader should trade, got kind %q (%+v)", a.Kind, a)
	}
	// dock dialog for a 3-good port: command + three defaulted answers
	if len(a.Tokens) != 4 || a.Tokens[0] != "p" {
		t.Errorf("dock tokens = %v, want [p    ] (p + 3 blanks)", a.Tokens)
	}
	// and the goal should now point at the other end of the pair
	if tr.dest != 20 {
		t.Errorf("after docking at 10 the Trader should target 20, dest=%d", tr.dest)
	}
}

func TestTraderExploresWithNoPair(t *testing.T) {
	tr := NewTrader("T", "trader", rand.New(rand.NewSource(1)))
	v := baseView(10)
	v.Mem.Observe(SectorView{Sector: 10, Warps: []int{11, 12}}, "self", 1, false)
	v.Mem.MarkVisited(10)
	// knows no ports at all -> must explore, i.e. move or autopilot somewhere
	a := tr.Plan(v)
	switch a.Kind {
	case "move", "autopilot", "scan":
		// fine: exploration
	default:
		t.Errorf("with no pair the Trader should explore, got kind %q", a.Kind)
	}
}

func TestTraderFleesHostileSector(t *testing.T) {
	tr := NewTrader("T", "trader", rand.New(rand.NewSource(1)))
	v := baseView(30) // outside fed space, where the threat is real
	v.Here.Battlegroups = []BattlegroupView{{Sector: 30, Owner: "enemy", Fighters: 9000}}
	a := tr.Plan(v)
	if a.Kind != "flee" {
		t.Fatalf("a hostile battlegroup in-sector should trigger flee, got %q", a.Kind)
	}
	if len(a.Tokens) != 2 || a.Tokens[0] != "m" {
		t.Errorf("flee should be a move, got %v", a.Tokens)
	}
}

func TestTraderIgnoresThreatsInFedSpace(t *testing.T) {
	tr := NewTrader("T", "trader", rand.New(rand.NewSource(1)))
	v := baseView(5) // fed space: combat is illegal, nothing here can hurt us
	v.Here.Ships = []ShipView{{Id: "big", Name: "Leviathan", Value: 10000000}}
	v.Mem.Observe(SectorView{Sector: 5, Warps: []int{6, 7}}, "self", 1, false)
	v.Mem.MarkVisited(5)
	if a := tr.Plan(v); a.Kind == "flee" {
		t.Errorf("fleeing inside protected fed space wastes turns, got %+v", a)
	}
}
