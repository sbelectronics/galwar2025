package botsim

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

func TestExplorerPeddlesAtPort(t *testing.T) {
	e := NewExplorer("E", rand.New(rand.NewSource(1)))
	v := baseView(30)
	here := tradingPortView(30, galwar.ORE)
	v.Here.Ports = []PortView{here}
	if a := e.Plan(v); a.Kind != "trade" {
		t.Fatalf("Explorer at a trading port should peddle, got %+v", a)
	}
	// having peddled here, a second call at the same stop must move on, not
	// re-dock (which would be a free-action loop)
	a := e.Plan(v)
	if a.Kind == "trade" {
		t.Errorf("Explorer should peddle a port only once per stop, got %+v", a)
	}
}

func TestCasualSkipsWholeDays(t *testing.T) {
	c := NewCasual("C", rand.New(rand.NewSource(1)))
	c.playProb = 0 // force a skip day
	v := baseView(10)
	if a := c.Plan(v); !a.Pass {
		t.Errorf("Casual on a skip day must Pass (accrue absence), got %+v", a)
	}
}

func TestCasualPlaysThenQuitsEarly(t *testing.T) {
	c := NewCasual("C", rand.New(rand.NewSource(1)))
	c.playProb = 1 // force a play day
	v := baseView(10)
	v.Mem.Observe(SectorView{Sector: 10, Warps: []int{11, 12}}, "self", 1, false)
	v.Mem.MarkVisited(10)
	if a := c.Plan(v); a.Pass {
		t.Errorf("Casual on a play day with turns should act, got Pass")
	}
	// simulate having spent past its budget -> it quits early
	v.Self.Turns = v.Self.Turns - c.turnBudget
	if a := c.Plan(v); !a.Pass {
		t.Errorf("Casual should quit early after its short turn budget, got %+v", a)
	}
}

func TestChaosMonkeyGeneratesValidCommandsAndAnswers(t *testing.T) {
	c := NewChaosMonkey("K", rand.New(rand.NewSource(1)))
	v := baseView(10)
	valid := map[string]bool{}
	for _, cmd := range chaosCommands {
		valid[cmd] = true
	}
	sawCmds := map[string]bool{}
	for i := 0; i < 200; i++ {
		a := c.Plan(v)
		if a.Pass || len(a.Tokens) != 1 || !valid[a.Tokens[0]] {
			t.Fatalf("ChaosMonkey should issue one known command, got %+v", a)
		}
		sawCmds[a.Tokens[0]] = true
	}
	if len(sawCmds) < 5 {
		t.Errorf("ChaosMonkey should vary its commands, only saw %d distinct", len(sawCmds))
	}
	// fillPrompt must always yield a printable, newline-free token (never panics)
	for i := 0; i < 300; i++ {
		tok := c.fillPrompt(c.rng, "some prompt")
		if strings.ContainsAny(tok, "\n\r") {
			t.Fatalf("fillPrompt produced a token with a newline: %q", tok)
		}
	}
}

func TestChaosMonkeyPassesWithoutTurns(t *testing.T) {
	c := NewChaosMonkey("K", rand.New(rand.NewSource(1)))
	v := baseView(10)
	v.Self.Turns = 0
	v.Self.BankedTurns = 0
	if a := c.Plan(v); !a.Pass {
		t.Errorf("ChaosMonkey with no turns should stop, got %+v", a)
	}
}
