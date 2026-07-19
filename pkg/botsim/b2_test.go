package botsim

import (
	"math/rand"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
	"github.com/sbelectronics/galwar/pkg/moderation"
)

// observeService teaches a memory about a service port at a sector.
func observeService(m *Memory, sector int, kind galwar.PortType) {
	pv := PortView{Sector: sector, Name: "svc", Kind: kind, Sells: map[string]bool{}, Full: true}
	m.Observe(SectorView{Sector: sector, Warps: []int{sector + 1}, Ports: []PortView{pv}}, "self", 1, true)
	m.MarkVisited(sector)
}

func TestBankerDepositsSurplus(t *testing.T) {
	b := NewBanker("B", rand.New(rand.NewSource(1)))
	v := baseView(40) // sector 40 holds the bank
	v.Self.Money = 60000
	observeService(v.Mem, 40, galwar.Interstel)

	a := b.Plan(v)
	if a.Kind != "bank" || len(a.Tokens) < 3 || a.Tokens[1] != "d" {
		t.Fatalf("Banker with surplus at the bank should deposit, got %+v", a)
	}
}

func TestBankerTradesWhenNotBanking(t *testing.T) {
	b := NewBanker("B", rand.New(rand.NewSource(1)))
	v := baseView(40)
	v.Self.Money = 15000 // between keep and 2*keep: no banking, just trade/explore
	observeService(v.Mem, 40, galwar.Interstel)
	v.Mem.Observe(SectorView{Sector: 40, Warps: []int{41, 42}}, "self", 1, false)

	a := b.Plan(v)
	if a.Kind == "bank" {
		t.Errorf("Banker without surplus should not bank, got %+v", a)
	}
}

func TestDefensiveGarrisonsHomePort(t *testing.T) {
	d := NewDefensiveTrader("D", rand.New(rand.NewSource(1)))
	v := baseView(30) // sector 30 > 10, legal to garrison
	d.pairA, d.pairB = 30, 45
	v.Self.Fighters = 400 // well above the floor
	// a trading port here so it's a plausible home; no foreign force
	here := tradingPortView(30, galwar.ORE)
	v.Here.Ports = []PortView{here}

	a := d.Plan(v)
	if a.Kind != "garrison" || a.Tokens[0] != "f" {
		t.Fatalf("DefensiveTrader at a home port with spare fighters should garrison, got %+v", a)
	}
}

func TestDefensiveWontGarrisonFedSpace(t *testing.T) {
	d := NewDefensiveTrader("D", rand.New(rand.NewSource(1)))
	v := baseView(5) // fed space: galactic law forbids leaving forces
	d.pairA, d.pairB = 5, 45
	v.Self.Fighters = 400
	a := d.Plan(v)
	if a.Kind == "garrison" {
		t.Errorf("garrisoning sector <= 10 is illegal; bot must not try, got %+v", a)
	}
}

func TestPlanetBuilderPlantsWithDevice(t *testing.T) {
	p := NewPlanetBuilder("P", rand.New(rand.NewSource(1)))
	v := baseView(30) // > 10, no planet here
	v.Self.Devices[galwar.GENESIS] = 1

	a := p.Plan(v)
	if a.Kind != "genesis" || a.Tokens[0] != "j" || len(a.Tokens) != 2 {
		t.Fatalf("PlanetBuilder with a device in open space should found a colony, got %+v", a)
	}
	// the chosen name must pass the real moderation gate the engine will apply
	if err := moderation.CheckPlanetName(a.Tokens[1]); err != nil {
		t.Errorf("planet name %q would be rejected: %v", a.Tokens[1], err)
	}
}

func TestPlanetBuilderBuysDeviceWhenFlush(t *testing.T) {
	p := NewPlanetBuilder("P", rand.New(rand.NewSource(1)))
	v := baseView(1) // at Sol (sector 1)
	v.Self.Money = 30000
	sol := PortView{Sector: 1, Name: "Sol", Kind: galwar.Sol, Sells: map[string]bool{}, Full: true,
		Menu: []string{galwar.HOLDS, galwar.FIGHTERS, galwar.MINES, galwar.GENESIS}}
	v.Here.Ports = []PortView{sol}
	observeService(v.Mem, 1, galwar.Sol)

	a := p.Plan(v)
	if a.Kind != "buy_device" {
		t.Fatalf("a flush PlanetBuilder at Sol with no colony should buy a Genesis Device, got %+v", a)
	}
}
