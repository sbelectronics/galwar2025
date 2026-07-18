package consoleui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// The scripted-terminal harness (fakeTerm, feed, session) and the shared
// displayUniverse/addShip helpers live in harness_test.go.

func TestSectorDisplayCappedInFedSpace(t *testing.T) {
	u := displayUniverse(t)
	viewer := addShip(t, u, "The Viewer", "v@x.com", 5, 1000)
	// eight other ships in sector 5, values ascending with index
	for i := 0; i < 8; i++ {
		addShip(t, u, fmt.Sprintf("Trader %c", 'A'+i), fmt.Sprintf("t%d@x.com", i), 5, 100000*(i+1))
	}

	term := &fakeTerm{}
	ui := NewConsoleUI(u, viewer, term)
	ui.DisplaySector(5)
	out := term.text()

	if got := strings.Count(out, "Player: "); got != fedShipCap {
		t.Errorf("fed-space display shows %d ships, want %d\n%s", got, fedShipCap, out)
	}
	if !strings.Contains(out, "(3 players not displayed)") {
		t.Errorf("missing not-displayed count:\n%s", out)
	}
	// the cap keeps the highest-ranked: H (800k) shown, A (100k) hidden
	if !strings.Contains(out, "Trader H") {
		t.Errorf("highest-value ship not shown:\n%s", out)
	}
	if strings.Contains(out, "Trader A") {
		t.Errorf("lowest-value ship shown despite the cap:\n%s", out)
	}
}

func TestSectorDisplayUncappedOutsideFedSpace(t *testing.T) {
	u := displayUniverse(t)
	viewer := addShip(t, u, "The Viewer", "v@x.com", 40, 1000)
	for i := 0; i < 8; i++ {
		addShip(t, u, fmt.Sprintf("Trader %c", 'A'+i), fmt.Sprintf("t%d@x.com", i), 40, 100000*(i+1))
	}

	term := &fakeTerm{}
	ui := NewConsoleUI(u, viewer, term)
	ui.DisplaySector(40)
	out := term.text()

	if got := strings.Count(out, "Player: "); got != 8 {
		t.Errorf("open-space display shows %d ships, want all 8\n%s", got, out)
	}
	if strings.Contains(out, "not displayed") {
		t.Errorf("open-space display was capped:\n%s", out)
	}
}

func TestRankingsCappedWithViewerBelowCap(t *testing.T) {
	u := displayUniverse(t)
	// 25 players, values descending with rank; the viewer is the poorest
	var viewer *galwar.Player
	for i := 0; i < 25; i++ {
		p := addShip(t, u, fmt.Sprintf("Warlord %c%c", 'A'+i/5, 'A'+i%5), fmt.Sprintf("w%d@x.com", i), 11, (25-i)*10000)
		viewer = p
	}

	term := &fakeTerm{}
	ui := NewConsoleUI(u, viewer, term)
	ui.ShowRankings()
	out := term.text()

	if !strings.Contains(out, "( 5 more not displayed )") {
		t.Errorf("missing rankings summary line:\n%s", out)
	}
	// the viewer (rank 25) appears despite being below the cap
	if !strings.Contains(out, "     25  "+viewer.GetName()) {
		t.Errorf("viewer's own row missing:\n%s", out)
	}
	// 20 capped rows + 1 viewer row
	if got := strings.Count(out, "Warlord "); got != rankingsCap+1 {
		t.Errorf("rankings show %d rows, want %d\n%s", got, rankingsCap+1, out)
	}
}

func TestComputerForcesAndStats(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Commander", "c@x.com", 40, 5000)
	planet := u.NewPlanet(p.Id, 40, "Homeworld")
	planet.SetQuantity(galwar.FIGHTERS, 750)

	// drive the computer menu: E (forces), U (stats), then Q
	term := &fakeTerm{in: []string{"e", "u", "q"}}
	ui := NewConsoleUI(u, p, term)
	ui.ExecuteComputer()
	out := term.text()

	if !strings.Contains(out, "Homeworld") || !strings.Contains(out, "750") {
		t.Errorf("[E] forces report missing the planet:\n%s", out)
	}
	if !strings.Contains(out, "Active traders:") {
		t.Errorf("[U] universe stats missing:\n%s", out)
	}
}

func TestComputerFindNearestPort(t *testing.T) {
	u := displayUniverse(t)
	// sector 1 is Sol (a port); park the player one hop away at a portless
	// sector if possible, else just confirm the "already at" path from Sol
	p := addShip(t, u, "Scout", "s@x.com", 1, 5000)

	term := &fakeTerm{in: []string{"f", "a", "q"}} // nearest ANY port
	ui := NewConsoleUI(u, p, term)
	ui.ExecuteComputer()
	out := term.text()
	if !strings.Contains(out, "Sol") {
		t.Errorf("[F] nearest-port did not find Sol from sector 1:\n%s", out)
	}
}

func TestPromptIntAbortAndHint(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Prompter", "p@x.com", 5, 5000)

	// "q" aborts (ok=false), never spins
	term := &fakeTerm{in: []string{"q"}}
	ui := NewConsoleUI(u, p, term)
	if n, ok := ui.PromptInt("num? "); ok {
		t.Errorf("q did not abort PromptInt: n=%d ok=%v", n, ok)
	}

	// bare Enter aborts a no-default PromptInt
	term = &fakeTerm{in: []string{""}}
	ui = NewConsoleUI(u, p, term)
	if _, ok := ui.PromptInt("num? "); ok {
		t.Errorf("bare Enter did not abort PromptInt")
	}

	// garbage gets a hint, then a real number is accepted (no infinite spin)
	term = &fakeTerm{in: []string{"abc", "42"}}
	ui = NewConsoleUI(u, p, term)
	n, ok := ui.PromptInt("num? ")
	if !ok || n != 42 {
		t.Errorf("garbage-then-number: n=%d ok=%v", n, ok)
	}
	if !strings.Contains(term.text(), "enter a number") {
		t.Errorf("no hint shown on non-numeric input:\n%s", term.text())
	}

	// PromptIntDefault: bare Enter -> default (ok), "q" -> abort
	term = &fakeTerm{in: []string{""}}
	ui = NewConsoleUI(u, p, term)
	if n, ok := ui.PromptIntDefault("num? ", 7); !ok || n != 7 {
		t.Errorf("bare Enter should take the default: n=%d ok=%v", n, ok)
	}
	term = &fakeTerm{in: []string{"q"}}
	ui = NewConsoleUI(u, p, term)
	if _, ok := ui.PromptIntDefault("num? ", 7); ok {
		t.Errorf("q should abort PromptIntDefault")
	}
}

func TestComputerBlockedByDamagedComputer(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Damaged", "d@x.com", 5, 5000)
	p.DamageSystem(galwar.SysComputer, 10)

	term := &fakeTerm{in: []string{"l", "q"}}
	ui := NewConsoleUI(u, p, term)
	ui.ExecuteComputer()
	out := term.text()
	if !strings.Contains(out, "damaged") {
		t.Errorf("damaged computer did not block the menu:\n%s", out)
	}
	if strings.Contains(out, "Net Worth") {
		t.Errorf("rankings shown despite a damaged computer:\n%s", out)
	}
}

func TestRankingsViewerInsideCapNotDuplicated(t *testing.T) {
	u := displayUniverse(t)
	var viewer *galwar.Player
	for i := 0; i < 25; i++ {
		p := addShip(t, u, fmt.Sprintf("Warlord %c%c", 'A'+i/5, 'A'+i%5), fmt.Sprintf("w%d@x.com", i), 11, (25-i)*10000)
		if i == 0 {
			viewer = p // the richest: rank 1, inside the cap
		}
	}

	term := &fakeTerm{}
	ui := NewConsoleUI(u, viewer, term)
	ui.ShowRankings()
	out := term.text()

	if got := strings.Count(out, viewer.GetName()); got != 1 {
		t.Errorf("viewer appears %d times, want exactly once\n%s", got, out)
	}
}
