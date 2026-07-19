package botsim

import (
	"reflect"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// mapObserve is a small helper to teach a memory a sector's warps.
func observeWarps(m *Memory, sector int, warps ...int) {
	m.Observe(SectorView{Sector: sector, Warps: warps}, "", 1, false)
	m.MarkVisited(sector)
}

func TestMemoryRouteOverKnownWarps(t *testing.T) {
	m := NewMemory(5)
	// a small chain 1-2-3-4 plus a shortcut 1-4
	observeWarps(m, 1, 2, 4)
	observeWarps(m, 2, 1, 3)
	observeWarps(m, 3, 2, 4)
	observeWarps(m, 4, 3, 1)

	got := m.Route(1, 3)
	// 1-2-3 and 1-4-3 are both length 3; either is acceptable, but it must be a
	// valid path of minimum length
	if len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("Route(1,3) = %v, want a 3-sector path ending 1..3", got)
	}
	if r := m.Route(1, 99); r != nil {
		t.Errorf("Route to an unmapped sector should be nil, got %v", r)
	}
}

func TestMemoryFrontierAndBlocked(t *testing.T) {
	m := NewMemory(5)
	// visited 1, which warps to 2 (unvisited) and 3 (unvisited)
	m.Observe(SectorView{Sector: 1, Warps: []int{2, 3}}, "", 1, false)
	m.MarkVisited(1)

	f := m.Frontier(1, 1)
	if f != 2 && f != 3 {
		t.Fatalf("Frontier(1) = %d, want an unvisited neighbor (2 or 3)", f)
	}

	// a fresh hostile battlegroup in sector 2 blocks it
	m.Observe(SectorView{Sector: 2, Battlegroups: []BattlegroupView{{Sector: 2, Owner: "enemy", Fighters: 500}}}, "self", 1, false)
	if !m.Blocked(2, 1) {
		t.Error("a fresh 500-fighter hostile group should block sector 2")
	}
	// and it decays to rumor after staleThreatDays
	if m.Blocked(2, 1+6) {
		t.Error("a 6-day-old sighting should have decayed past the 5-day block window")
	}
}

func TestMemoryKeepsFullCommerceOverHeadline(t *testing.T) {
	m := NewMemory(5)
	full := PortView{Sector: 7, Name: "P", Kind: galwar.TradingPort,
		Sells: map[string]bool{galwar.ORE: true}, Buys: map[string]bool{galwar.ORGANICS: true}, Full: true}
	m.Observe(SectorView{Sector: 7, Ports: []PortView{full}}, "self", 1, true)
	// a later headline-only sighting must not erase the buy list we docked to learn
	headline := PortView{Sector: 7, Name: "P", Kind: galwar.TradingPort, Sells: map[string]bool{galwar.ORE: true}}
	m.Observe(SectorView{Sector: 7, Ports: []PortView{headline}}, "self", 2, false)

	p := m.Port(7)
	if p == nil || !p.Full || !p.Buys[galwar.ORGANICS] {
		t.Fatalf("headline re-sighting clobbered full commerce: %+v", p)
	}
}

func TestMemoryObserveWarpsRoundTrip(t *testing.T) {
	m := NewMemory(5)
	m.Observe(SectorView{Sector: 5, Warps: []int{4, 6, 7}}, "self", 1, false)
	if !reflect.DeepEqual(m.warps[5], []int{4, 6, 7}) {
		t.Errorf("warps not recorded: %v", m.warps[5])
	}
}
