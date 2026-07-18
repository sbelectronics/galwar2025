package consoleui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// This is the consoleui package's scripted-terminal test harness. The three
// real front-ends (console, telnet, web) all reach the game through the
// Terminal interface, so a Terminal that reads from a canned script and
// captures everything printed lets a test drive any UI method exactly as a
// player would: feed the keystrokes, run the method, assert on what the
// player would have seen. See session_test.go for the loop-level tests it
// exists to make cheap.

// fakeTerm is a scripted Terminal: canned input lines in, captured output out.
//
//	term := feed("m", "5", "q")
//	ui := NewConsoleUI(u, p, term)
//	ui.Run()
//	term.wants(t, "Move to what sector?")
//
// ReadLine hands back the next scripted line; once the script is exhausted it
// returns io.EOF, which every input path treats as a disconnect (so a method
// that keeps prompting past its script ends the session rather than spinning).
// fakeTerm deliberately does not implement SecretReader, so password reads
// fall through ReadPassword to ReadLine and are drawn from the same script.
type fakeTerm struct {
	in   []string
	out  bytes.Buffer
	eofs int // reads attempted past the end of the script
}

// feed builds a fakeTerm from a script of input lines.
func feed(lines ...string) *fakeTerm { return &fakeTerm{in: lines} }

func (t *fakeTerm) Printf(format string, args ...any) {
	fmt.Fprintf(&t.out, format, args...)
}

func (t *fakeTerm) ReadLine() (string, error) {
	if len(t.in) == 0 {
		t.eofs++
		return "", io.EOF
	}
	line := t.in[0]
	t.in = t.in[1:]
	return line, nil
}

// raw is everything printed, ANSI intact.
func (t *fakeTerm) raw() string { return t.out.String() }

// text is everything printed with ANSI stripped - what the player read.
func (t *fakeTerm) text() string { return StripANSI(t.out.String()) }

// wants asserts every substring appears somewhere in the (ANSI-stripped)
// output. On failure it dumps the whole transcript so the miss is diagnosable.
func (t *fakeTerm) wants(tb testing.TB, subs ...string) {
	tb.Helper()
	out := t.text()
	for _, s := range subs {
		if !strings.Contains(out, s) {
			tb.Errorf("output missing %q\n--- transcript ---\n%s", s, out)
		}
	}
}

// rejects asserts none of the substrings appear in the output.
func (t *fakeTerm) rejects(tb testing.TB, subs ...string) {
	tb.Helper()
	out := t.text()
	for _, s := range subs {
		if strings.Contains(out, s) {
			tb.Errorf("output unexpectedly contains %q\n--- transcript ---\n%s", s, out)
		}
	}
}

// session wires a scripted terminal to a fresh UI in one step.
func session(t *testing.T, u *galwar.UniverseType, p *galwar.Player, lines ...string) (*ConsoleUI, *fakeTerm) {
	t.Helper()
	term := feed(lines...)
	return NewConsoleUI(u, p, term), term
}

// displayUniverse builds a small generated universe with default config,
// enough sectors and ports for the UI tests to have somewhere to move, dock,
// and rank.
func displayUniverse(t *testing.T) *galwar.UniverseType {
	t.Helper()
	u := galwar.NewUniverse()
	u.Generate(60)
	u.SeedDefaultConfig()
	return u
}

// addShip registers a player, marks them moved (so they are visible), parks
// them in a sector, and gives them a distinguishing net worth.
func addShip(t *testing.T, u *galwar.UniverseType, name, email string, sector, money int) *galwar.Player {
	t.Helper()
	p, err := u.RegisterPlayer(name, email, "")
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
	p.EverMoved = true
	p.MoveTo(sector)
	p.Money = money
	return p
}
