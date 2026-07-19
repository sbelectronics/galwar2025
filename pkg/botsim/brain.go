package botsim

import "math/rand"

// A Brain is a bot's decision-maker. It is a pure function of a View: given
// what the bot can legitimately perceive, it returns the next Action (a line of
// player input) to feed the ConsoleUI. Brains hold their own strategy state
// (current trade pair, funds earmarked, day counter) but never touch the
// universe or the transcript - which makes them unit-testable against a crafted
// View with no scheduler, no terminal, no engine goroutine.
type Brain interface {
	// Name is the bot's player handle (also its log identity).
	Name() string
	// Class is the strategy label ("trader", "aggressor", ...) for the log.
	Class() string
	// Plan returns the next action at a main-command boundary. Returning a
	// Pass action means "done for the day"; the scheduler stops granting the
	// bot slices until the next day.
	Plan(v *View) Action
}

// promptFiller is implemented by fuzzers (ChaosMonkey): rather than desyncing
// when the UI asks for input it didn't queue, a filler generates a plausible
// answer for every prompt. Its finding class is panic/hang/state corruption
// (the invariant sweep), not desync - so botTerm routes its unqueued prompts
// here instead of logging a desync.
type promptFiller interface {
	// fillPrompt answers an arbitrary prompt. tail is the recent transcript for
	// whatever context the filler cares to use (it mostly ignores it).
	fillPrompt(rng *rand.Rand, tail string) string
}

// Action is one decision: a sequence of input tokens the ConsoleUI will consume
// (the first is the command letter, the rest answer its prompts), plus metadata
// for the event log. An Action must be self-contained: it navigates into and
// back out of any sub-menu it enters, ending at the main-command prompt, so the
// next Plan starts cleanly. Under-supplying tokens is a desync (a finding);
// over-supplying is too.
type Action struct {
	Tokens []string       // input lines; Tokens[0] is the command letter
	Kind   string         // event kind for the log ("move", "trade", ...)
	Detail map[string]any // structured detail for the log
	Pass   bool           // done for the day (no tokens executed)
}

// pass is the done-for-the-day action.
func pass() Action { return Action{Pass: true, Kind: "pass"} }

// act builds an action from a command kind and its token line.
func act(kind string, tokens ...string) Action {
	return Action{Tokens: tokens, Kind: kind}
}

// withDetail attaches structured log detail and returns the action.
func (a Action) withDetail(d map[string]any) Action {
	a.Detail = d
	return a
}
