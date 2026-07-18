package consoleui

import (
	"fmt"
	"strings"
	"testing"
)

// guideText is the whole guide concatenated, for content assertions.
func guideText() string {
	var b strings.Builder
	for _, s := range guideSections {
		b.WriteString(s.title)
		b.WriteString("\n")
		b.WriteString(s.body)
		b.WriteString("\n")
	}
	return b.String()
}

func TestGuidePagesWithoutError(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Reader", "r@x.com", 5, 5000)

	// read every section, then quit. For each section, feed its number, then
	// exactly one Enter per "more" prompt the pager will show (screens-1),
	// which the pager consumes before returning to the menu. A bare Enter at
	// the menu would abort, so the count must be exact.
	var script []string
	for i, s := range guideSections {
		script = append(script, fmt.Sprintf("%d", i+1))
		lines := strings.Split(strings.TrimRight(s.body, "\n"), "\n")
		screens := (len(lines) + guidePageSize - 1) / guidePageSize
		for k := 0; k < screens-1; k++ {
			script = append(script, "") // continue past a "more" prompt
		}
	}
	script = append(script, "q")

	term := &fakeTerm{in: script}
	ui := NewConsoleUI(u, p, term)
	ui.ExecuteGuide()
	if ui.Terminated {
		t.Errorf("guide terminated the session")
	}
	out := term.text()
	// the manual actually rendered content
	if !strings.Contains(out, "down payment on cargo holds") {
		t.Errorf("guide body did not render:\n%s", out[:min(len(out), 400)])
	}
}

func TestGuideCoversImplementedCommands(t *testing.T) {
	text := guideText()
	// every implemented main command should be mentioned by its bracketed
	// letter somewhere in the manual
	for _, cmd := range []string{"A", "B", "C", "D", "F", "G", "H", "I", "J", "L", "M", "P", "Q", "S", "W", "Y", "Z"} {
		if !strings.Contains(text, "<"+cmd+">") && !strings.Contains(text, "["+cmd+"]") {
			t.Errorf("guide never documents the implemented [%s] command", cmd)
		}
	}
}

func TestGuideOmitsUnimplemented(t *testing.T) {
	lower := strings.ToLower(guideText())
	// deferred features must not be documented as if they worked (a manual
	// that lies is the fastest way to lose a new player)
	for _, feature := range []string{"starbase", "outpost", "team", "casino", "lottery", "mail", "message", "armageddon"} {
		if strings.Contains(lower, feature) {
			t.Errorf("guide mentions the unimplemented feature %q", feature)
		}
	}
}

// TestGuideOmitsUnimplementedLetters is the plan's promised regression guard:
// the unimplemented main-menu letters must never be documented as commands.
// Checked against the whole guide in both heading styles, so a future section
// can't quietly document <R> Starbase Transporter back into existence.
func TestGuideOmitsUnimplementedLetters(t *testing.T) {
	text := guideText()
	for _, cmd := range []string{"R", "T", "V", "O", "U"} {
		if strings.Contains(text, "<"+cmd+">") {
			t.Errorf("guide documents the unimplemented main command <%s>", cmd)
		}
	}
	// [U] is legitimate only as the computer sub-menu's universe report; the
	// main-menu section itself must not use it
	for _, s := range guideSections {
		if s.title != "Main Menu Commands" {
			continue
		}
		for _, cmd := range []string{"[R]", "[T]", "[V]", "[O]", "[U]"} {
			if strings.Contains(s.body, cmd) {
				t.Errorf("main-menu section documents the unimplemented command %s", cmd)
			}
		}
	}
}

func TestGuidePricesAccurate(t *testing.T) {
	text := guideText()
	// prices quoted in the manual must match the engine's actual numbers
	for label, price := range map[string]string{
		"cargo hold":        "500",
		"fighter":           "98",
		"Genesis":           "10,000",
		"Emergency Warp":    "27,000",
		"Fusion Cell":       "45,000",
		"Planetary Scanner": "40,000",
		"Mine Deflector":    "6,000",
	} {
		if !strings.Contains(text, price) {
			t.Errorf("guide is missing the correct price %s for %s", price, label)
		}
	}
	// starting kit
	for _, n := range []string{"35,000", "25 cargo holds", "200 fighters", "250"} {
		if !strings.Contains(text, n) {
			t.Errorf("guide is missing the starting-kit figure %q", n)
		}
	}
}
