package botsim

import (
	"math/rand"
	"strconv"
)

// chaosCommands is the weighted command table the fuzzer pokes, biased toward
// the multi-prompt dialogs where bugs hide. It deliberately omits "q" (which
// would quit the session) and the sysop menu.
var chaosCommands = []string{
	"p", "p", "p", "l", "l", "a", "a", "b", "f", "f", "g",
	"m", "m", "m", "y", "y", "w", "j", "c", "s", "i", "h", "d", "report",
}

// ChaosMonkey is the fuzzer: it issues a random command each action and answers
// every ensuing prompt from a generator biased toward plausible-but-edgy values.
// It expects errors and ignores them; its finding class is a panic, hang, or
// state corruption, caught by the invariant sweep (PLAN-BOTS.md 3.8). It is not
// a Trader - it has no strategy at all.
type ChaosMonkey struct {
	name       string
	rng        *rand.Rand
	numSectors int
}

// NewChaosMonkey builds a ChaosMonkey brain.
func NewChaosMonkey(name string, rng *rand.Rand) *ChaosMonkey {
	return &ChaosMonkey{name: name, rng: rng}
}

func (c *ChaosMonkey) Name() string  { return c.name }
func (c *ChaosMonkey) Class() string { return "chaos" }

// Plan issues one random command; its prompts are answered by fillPrompt.
func (c *ChaosMonkey) Plan(v *View) Action {
	c.numSectors = v.NumSectors
	if !v.Self.HasTurns() {
		return pass()
	}
	return act("chaos", chaosCommands[c.rng.Intn(len(chaosCommands))])
}

// fillPrompt answers an arbitrary prompt with a plausible-but-edgy token,
// biased toward exit tokens so dialogs terminate rather than trap the fuzzer.
func (c *ChaosMonkey) fillPrompt(rng *rand.Rand, tail string) string {
	switch n := rng.Intn(100); {
	case n < 25:
		return "q" // cancel a prompt / quit a menu
	case n < 37:
		return "" // take a default / bare enter
	case n < 47:
		return "y"
	case n < 55:
		return "n"
	case n < 70:
		return strconv.Itoa(rng.Intn(10)) // small int: 0,1,...
	case n < 82:
		if c.numSectors > 0 {
			return strconv.Itoa(1 + rng.Intn(c.numSectors)) // a valid-ish sector
		}
		return "1"
	case n < 92:
		return strconv.Itoa(rng.Intn(200000)) // an edgy amount (over-max, exact-ish)
	default:
		return randToken(rng) // a short printable string (name/handle attempt)
	}
}

// randToken makes a short printable-ASCII string, for prompts that want text.
func randToken(rng *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOP0123456789.-'"
	n := 2 + rng.Intn(12)
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}
