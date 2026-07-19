package botsim

import (
	"bytes"
	"fmt"
	"io"
)

// mainPromptMarker is the tail the ConsoleUI prints at a fresh main-command
// boundary. Seeing it (and only it) tells botTerm the previous action finished
// and the Brain should plan the next one; seeing any other prompt with an empty
// token queue is a desync (PLAN-BOTS.md 2.2).
const mainPromptMarker = "Main Command (?=Help) ?"

// desyncAbortLimit bounds how many safe-abort tokens botTerm will feed trying to
// escape a dialog the Brain under-supplied, before giving up and ending the
// bot's session (a serious finding - an inescapable prompt would hang a run).
const desyncAbortLimit = 24

// recentCap bounds the transcript context retained for desync diagnostics.
const recentCap = 4096

// botTerm is the Terminal a bot's ConsoleUI session drives. Printf captures
// output; ReadLine feeds the Brain's planned tokens, recognizing the
// main-command boundary to pace the scheduler and to detect desyncs.
type botTerm struct {
	b *bot

	pending bytes.Buffer // printed since the last ReadLine (boundary detection)
	recent  []byte       // bounded tail of the whole transcript (desync context)
	full    io.Writer    // optional per-bot transcript sink (-transcripts)

	queue    []string // remaining tokens of the current action
	aborts   int      // consecutive desync aborts in the current dialog
	torndown bool     // session ended by teardown or a stuck finding, not death
}

func newBotTerm(b *bot) *botTerm { return &botTerm{b: b} }

// reset clears per-session state so a rebuilt session (after a death) starts
// clean. The transcript sink is preserved.
func (t *botTerm) reset() {
	t.pending.Reset()
	t.recent = t.recent[:0]
	t.queue = nil
	t.aborts = 0
	t.torndown = false
}

func (t *botTerm) Printf(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	t.pending.WriteString(s)
	t.recent = appendBounded(t.recent, s, recentCap)
	if t.full != nil {
		io.WriteString(t.full, s)
	}
}

// ReadLine feeds the next input token. Queued tokens (mid-action answers) are
// served directly; an empty queue means we're at a boundary - either the main
// prompt (plan the next action, pacing the scheduler) or, if not, a desync.
func (t *botTerm) ReadLine() (string, error) {
	atMain := bytes.Contains(t.pending.Bytes(), []byte(mainPromptMarker))
	t.pending.Reset()

	if atMain {
		t.aborts = 0
		// Reaching a main-command boundary with tokens still queued means the
		// last action's dialog ended before consuming them (e.g. the autopilot
		// bailed, or a trade was cut short by a mid-action death under
		// -concurrent). Discard the orphans rather than executing one as a
		// spurious command, and record the over-provision as a desync.
		if len(t.queue) > 0 {
			t.queue = nil
			t.b.errors++
			t.b.sim.finding(Event{Day: t.b.sim.day, T: t.b.sim.clock().Unix(), Bot: t.b.name,
				Class: t.b.class, Ev: "desync", Extra: map[string]any{
					"kind": "over-provision", "tail": string(t.recent)}})
		}
		return t.beginAction()
	}
	if len(t.queue) > 0 {
		tok := t.queue[0]
		t.queue = t.queue[1:]
		return tok, nil
	}
	// a fuzzer answers every prompt rather than desyncing (its findings are
	// panics/hangs/corruption, caught by the invariant sweep, not desync)
	if t.b.filler != nil {
		return t.b.filler.fillPrompt(t.b.rng, string(t.recent)), nil
	}
	return t.handleDesync()
}

// beginAction paces the scheduler and asks the Brain for the next action. In
// deterministic mode acquire blocks until this bot's round-robin turn; in
// concurrent mode it returns immediately. A Pass parks the bot until the next
// day. Returns the command letter (first token); the rest queue behind it.
func (t *botTerm) beginAction() (string, error) {
	b := t.b
	for {
		if !b.sim.sched.acquire(b) {
			t.torndown = true
			return "", io.EOF
		}
		// a bot killed by another player's action while parked comes back here
		// dead; end the session so the goroutine can handle reconstruction.
		v := perceive(b, b.sim.day, b.sim.clock())
		if v.Self.Dead {
			return "", io.EOF
		}
		if b.actionsToday >= b.sim.maxActionsPerDay {
			// hitting the cap is a stuck finding for a strategic bot, but normal
			// for a fuzzer that pokes free commands all day - just pass it
			if b.filler == nil {
				b.sim.finding(Event{Day: b.sim.day, T: b.sim.clock().Unix(), Bot: b.name, Class: b.class,
					Ev: "stuck", Extra: map[string]any{"detail": "exceeded max actions/day; forcing pass"}})
			}
			if !b.sim.sched.passDay(b) {
				t.torndown = true
				return "", io.EOF
			}
			continue
		}

		action := b.brain.Plan(v)
		if action.Pass {
			if !b.sim.sched.passDay(b) {
				t.torndown = true
				return "", io.EOF
			}
			continue // released into a new day: re-plan
		}
		if len(action.Tokens) == 0 {
			action.Tokens = []string{""} // defensive: never busy-loop on an empty action
		}
		b.actionsToday++
		if !b.loggedInToday {
			// first real action of the day is the "login": clears dormancy,
			// like TouchLastSeen at a real session start. A bot that only ever
			// passes a day (a Casual skip) never logs in, so it accrues absence.
			b.loggedInToday = true
			now := b.sim.clock().Unix()
			b.sim.u.Do(func() { b.sim.u.TouchLastSeen(b.player, now) })
		}
		t.logAction(action, v)
		b.sim.tick(1e9) // +1s: keep event timestamps ordered without crossing a day
		t.queue = action.Tokens[1:]
		return action.Tokens[0], nil
	}
}

// handleDesync responds when the UI asks for input the Brain didn't queue. It
// logs the first occurrence with transcript context, then feeds safe aborts (Q,
// then empty) to unwind back to the main prompt. If it can't escape, it ends the
// session as a stuck finding rather than hang.
func (t *botTerm) handleDesync() (string, error) {
	b := t.b
	t.aborts++
	if t.aborts == 1 {
		b.errors++
		b.sim.finding(Event{Day: b.sim.day, T: b.sim.clock().Unix(), Bot: b.name, Class: b.class,
			Ev: "desync", Extra: map[string]any{"tail": string(t.recent)}})
	}
	if t.aborts > desyncAbortLimit {
		b.sim.finding(Event{Day: b.sim.day, T: b.sim.clock().Unix(), Bot: b.name, Class: b.class,
			Ev: "stuck", Extra: map[string]any{"detail": "could not escape a dialog after safe aborts"}})
		t.torndown = true
		return "", io.EOF
	}
	if t.aborts%2 == 1 {
		return "q", nil // Q cancels most prompts and exits most menus
	}
	return "", nil // a bare Enter unwinds the rest
}

// logAction records a bot's intended action. Realized effects (net profit,
// combat outcomes) are attributed by the Brain in Detail and by the day
// summary; this is the per-action ground truth.
func (t *botTerm) logAction(a Action, v *View) {
	b := t.b
	if a.Kind == "" || quietAction[a.Kind] {
		return
	}
	extra := map[string]any{"sector": v.Self.Sector}
	for k, val := range a.Detail {
		extra[k] = val
	}
	b.sim.log.Emit(Event{Day: b.sim.day, T: b.sim.clock().Unix(), Bot: b.name, Class: b.class,
		Ev: a.Kind, Extra: extra})
}

// quietAction lists high-frequency, low-signal actions kept out of the event
// stream so a Trader's day is tens of events, not thousands (PLAN-BOTS.md 4.1).
var quietAction = map[string]bool{
	"scan": true,
	"info": true,
	"idle": true,
}

func appendBounded(buf []byte, s string, cap int) []byte {
	buf = append(buf, s...)
	if len(buf) > cap {
		buf = append(buf[:0], buf[len(buf)-cap:]...)
	}
	return buf
}
