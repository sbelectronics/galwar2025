package botsim

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
)

// lastLine returns the final non-empty line of s (the freshest transcript
// context for an error), trimmed of ANSI-ish noise.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return oneLine(t, 200)
		}
	}
	return ""
}

// Config parameters a simulation run. Zero values are filled with defaults by
// New, so a caller can set only what it cares about.
type Config struct {
	Days        int            // simulated days to run
	Seed        int64          // RNG seed; a run is a pure function of (Seed, fleet) in deterministic mode
	Sectors     int            // universe size
	Fleet       map[string]int // class -> count (e.g. {"trader":3})
	Out         string         // output directory (events.jsonl, digest.txt, universe.yaml)
	Concurrent  bool           // run bots in parallel (stress test; not reproducible)
	Transcripts bool           // write per-bot UI transcripts
	Strict      bool           // stop the run on the first finding (CI mode)
	KnownMap    bool           // pre-seed every bot's memory with the port map
}

// sim is the running simulation: the universe, the fleet, the clock, the log,
// and the scheduler that paces them.
type sim struct {
	cfg    Config
	u      *galwar.UniverseType
	bots   []*bot
	log    *Logger
	sched  *scheduler
	prices Prices

	day     int
	simNow  time.Time // the synthetic clock (see galwar.Now hook)
	clockMu sync.Mutex

	baseEpoch        time.Time
	maxActionsPerDay int

	prevNow     func() time.Time  // galwar.Now before we installed the sim clock; restored by Run
	factionPrev map[string]string // last-seen faction active flags, for transition events

	strictStop chan struct{} // closed when a finding aborts a -strict run
	stopOnce   sync.Once
}

// bot is one computer-controlled player: its Brain, its ship, its private
// memory, and the plumbing that lets the scheduler pace it and the botTerm feed
// it through a real ConsoleUI session.
type bot struct {
	sim    *sim
	index  int
	name   string
	class  string
	brain  Brain
	filler promptFiller // non-nil for fuzzers (ChaosMonkey): answers every prompt
	player *galwar.Player
	mem    *Memory
	rng    *rand.Rand
	term   *botTerm
	ui     *consoleui.ConsoleUI
	trans  io.WriteCloser // optional per-bot transcript sink (-transcripts)

	// scheduler plumbing (see scheduler.go): grant/ready drive the
	// deterministic round-robin; done reports the bot is out for the day (true
	// if it died); wake resumes it at the next day's start.
	grant chan struct{}
	ready chan struct{}
	done  chan bool
	wake  chan struct{}

	actionsToday  int
	loggedInToday bool
	dayStartVal   int
	kills         int
	deaths        int
	errors        int
}

// loop is the bot's goroutine: run a ConsoleUI session per day. A session ends
// only on teardown (exit the goroutine) or death (report it, then loop to build
// a fresh session next day, after reconstruction at day start). A Pass never
// ends the session - it parks inside the session until the next day.
func (b *bot) loop() {
	defer b.sim.sched.wg.Done()
	for {
		b.startSession()
		b.ui.Run()
		if b.term.torndown {
			return // teardown or a stuck finding
		}
		// Run returned and it wasn't teardown => the bot died this day
		b.deaths++
		if !b.sim.sched.finishDay(b, true) {
			return
		}
		// woken into a new day; the day-start ritual already reconstructed us
	}
}

// startSession builds a fresh ConsoleUI session for a new day (or after a
// death), reusing the bot's botTerm (reset).
func (b *bot) startSession() {
	b.term.reset()
	if b.sim.cfg.Transcripts && b.trans != nil {
		b.term.full = b.trans
	}
	b.ui = consoleui.NewConsoleUI(b.sim.u, b.player, b.term)
	b.ui.OnError = b.onEngineError
}

// expectedErr is the set of engine error codes that are routine outcomes a bot
// deliberately bumps into (can't afford, sold out, out of turns, killed). Any
// other error suggests the bot built a malformed action - a finding worth
// surfacing (PLAN-BOTS.md 5).
var expectedErr = map[galwar.GameErrorCode]bool{
	galwar.ErrNotEnoughMoney:    true,
	galwar.ErrNotEnoughQuantity: true,
	galwar.ErrNotEnoughHolds:    true,
	galwar.ErrNoTurns:           true,
	galwar.ErrDead:              true,
}

// onEngineError classifies an engine error the bot's session hit. Expected ones
// are counted; unexpected ones are logged as findings with the offending
// transcript tail, which is how a broken dialog assumption gets caught.
func (b *bot) onEngineError(err error) {
	b.errors++
	// a fuzzer triggers errors on purpose; its findings are panics/corruption
	// (the invariant sweep), never a mere engine error
	if b.filler != nil {
		return
	}
	if !unexpectedError(err) {
		return // a routine rule violation, not a finding
	}
	b.sim.finding(Event{Day: b.sim.day, T: b.sim.clock().Unix(), Bot: b.name, Class: b.class,
		Ev: "error_unexpected", Extra: map[string]any{
			"error": err.Error(), "tail": lastLine(string(b.term.recent)),
		}})
}

// unexpectedError reports whether an engine error is a finding (a malformed
// action) rather than a routine rule violation a bot deliberately risks.
func unexpectedError(err error) bool {
	var ge *galwar.GameError
	if errors.As(err, &ge) {
		return !expectedErr[ge.Code()]
	}
	return true // a non-game error (panic-ish) is always worth surfacing
}

// startDay runs the login-time ritual for the whole fleet: reconstruct anyone
// who died on a prior day (so their new session doesn't immediately re-die),
// reset per-day counters, and snapshot each bot's starting net worth for the
// day summary.
func (s *sim) startDay(day int) {
	s.setDay(day)
	now := s.clock()
	reconstructed := map[string]bool{}
	s.u.Do(func() {
		for _, b := range s.bots {
			if _, ok := s.u.ReconstructIfDue(b.player, now); ok {
				reconstructed[b.name] = true
			}
			b.dayStartVal = s.u.PlayerValue(b.player)
		}
	})
	for _, b := range s.bots {
		if reconstructed[b.name] {
			s.log.Emit(Event{Day: day, T: now.Unix(), Bot: b.name, Class: b.class, Ev: "reconstructed"})
		}
	}
	for _, b := range s.bots {
		b.actionsToday = 0
		b.loggedInToday = false
	}
}

// emitDeath records a bot's death for the day. Richer combat context is added
// in the combat-logging milestone.
func (s *sim) emitDeath(b *bot) {
	s.log.Emit(Event{Day: s.day, T: s.clock().Unix(), Bot: b.name, Class: b.class, Ev: "died"})
}

// New builds a simulation from a config, generating a fresh universe seeded
// from cfg.Seed. It does not start running; call Run.
func New(cfg Config) (*sim, error) {
	if cfg.Days <= 0 {
		cfg.Days = 30
	}
	if cfg.Sectors <= 0 {
		cfg.Sectors = 1000
	}
	if len(cfg.Fleet) == 0 {
		cfg.Fleet = DefaultFleet()
	}
	if cfg.Out == "" {
		cfg.Out = filepath.Join("simruns", "run")
	}

	// Seed the global RNG so universe generation and engine combat are
	// reproducible in deterministic mode (single-threaded consumption).
	rand.Seed(cfg.Seed)

	u := galwar.NewUniverse()
	u.Generate(cfg.Sectors)
	u.SeedDefaultConfig()

	s := &sim{
		cfg:              cfg,
		u:                u,
		baseEpoch:        time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		maxActionsPerDay: 3*u.ConfigInt("turns_per_day", 250) + 100,
		strictStop:       make(chan struct{}),
	}
	s.simNow = s.baseEpoch
	s.prices = snapshotPrices(u)

	if err := os.MkdirAll(cfg.Out, 0755); err != nil {
		return nil, err
	}
	evFile, err := os.Create(filepath.Join(cfg.Out, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	s.log = NewLogger(evFile, evFile)

	// Install the synthetic clock. The engine reads galwar.Now everywhere it
	// needs wall time; pointing it at the sim clock is what makes dormancy,
	// interest, restock, and faction quiet-days experience compressed days.
	// Run restores the previous clock when the simulation finishes.
	s.prevNow = galwar.Now
	galwar.Now = s.clock

	if err := s.buildFleet(); err != nil {
		return nil, err
	}
	s.sched = newScheduler(s, cfg.Concurrent)
	return s, nil
}

// openTranscript creates a per-bot transcript file under out/transcripts.
func (s *sim) openTranscript(name string) (io.WriteCloser, error) {
	dir := filepath.Join(s.cfg.Out, "transcripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return os.Create(filepath.Join(dir, name+".txt"))
}

// clock is the galwar.Now implementation: the current synthetic time.
func (s *sim) clock() time.Time {
	s.clockMu.Lock()
	defer s.clockMu.Unlock()
	return s.simNow
}

// tick advances the synthetic clock by d, staying within the current UTC day so
// it never crosses a maintenance boundary mid-day. Used between actions in
// deterministic mode to keep event timestamps ordered.
func (s *sim) tick(d time.Duration) {
	s.clockMu.Lock()
	defer s.clockMu.Unlock()
	next := s.simNow.Add(d)
	// keep within ~23h of the day's start so we never trip RunDailyMaintenance
	dayStart := s.baseEpoch.Add(time.Duration(s.day-1) * 24 * time.Hour)
	if limit := dayStart.Add(23 * time.Hour); next.After(limit) {
		next = limit
	}
	s.simNow = next
}

// setDay sets the synthetic clock to the start of a given day (1-based).
func (s *sim) setDay(day int) {
	s.clockMu.Lock()
	defer s.clockMu.Unlock()
	s.day = day
	s.simNow = s.baseEpoch.Add(time.Duration(day-1) * 24 * time.Hour)
}

func snapshotPrices(u *galwar.UniverseType) Prices {
	// starter kit value: credits + fighters + holds at their configured costs
	kit := u.ConfigInt("starting_credits", 35000) +
		u.ConfigInt("starting_fighters", 200)*u.ConfigInt("cost_of_fighter", 98) +
		u.ConfigInt("starting_holds", 25)*u.ConfigInt("cost_of_hold", 500)
	return Prices{
		Hold:        u.ConfigInt("cost_of_hold", 500),
		Fighter:     u.ConfigInt("cost_of_fighter", 98),
		Mine:        u.ConfigInt("cost_of_mine", 15000),
		Genesis:     u.ConfigInt("cost_of_genesis", 10000),
		Scanner:     u.ConfigInt("cost_of_scanner", 40000),
		TurnsPerDay: u.ConfigInt("turns_per_day", 250),
		TargetFloor: u.ConfigInt("faction_target_floor", 200000),
		StartingKit: kit,
	}
}

// Run executes the whole simulation: launches the fleet, runs each day with the
// scheduler, ticks maintenance between days, and writes the final artifacts.
// Returns the number of findings recorded.
func (s *sim) Run() (findings int, err error) {
	// restore the production clock when the run finishes, so a sim doesn't leave
	// galwar.Now pointing at a dead simulation (matters for back-to-back runs).
	if s.prevNow != nil {
		defer func() { galwar.Now = s.prevNow }()
	}
	s.log.Emit(Event{Day: 0, T: s.simNow.Unix(), Ev: "run_start", Extra: map[string]any{
		"seed": s.cfg.Seed, "days": s.cfg.Days, "sectors": s.cfg.Sectors,
		"fleet": s.cfg.Fleet, "concurrent": s.cfg.Concurrent,
	}})

	s.sched.run(s.cfg.Days, s.onDayEnd)

	for _, b := range s.bots {
		if b.trans != nil {
			b.trans.Close()
		}
	}

	s.log.Emit(Event{Day: s.cfg.Days, T: s.simNow.Unix(), Ev: "run_end", Extra: map[string]any{
		"findings": s.log.Findings(),
	}})

	if err := s.writeArtifacts(); err != nil {
		return s.log.Findings(), err
	}
	s.log.Flush()
	if err := s.log.Close(); err != nil {
		return s.log.Findings(), err
	}
	return s.log.Findings(), nil
}

// onDayEnd runs after the last bot has finished a day's actions: advance the
// clock a full day, run maintenance, sweep invariants, and record the
// scoreboard. Runs on the scheduler goroutine with no bot active, so the
// universe is quiescent.
func (s *sim) onDayEnd(day int) {
	// per-bot day summaries (captured before maintenance refills turns)
	for _, b := range s.bots {
		s.emitDaySummary(b, day)
	}
	s.emitScoreboard(day)

	// advance to the next UTC day and run the nightly pass
	next := s.baseEpoch.Add(time.Duration(day) * 24 * time.Hour)
	s.clockMu.Lock()
	s.simNow = next
	s.clockMu.Unlock()
	s.u.Do(func() {
		s.u.RunDailyMaintenance(next)
	})
	s.captureFactionState(day)

	// invariant sweep: catch quiet state corruption a day at the latest
	s.sweepInvariants(day)
}

// finding records a finding-class event and, under -strict, aborts the run.
func (s *sim) finding(ev Event) {
	s.log.Emit(ev)
	if s.cfg.Strict {
		s.stopOnce.Do(func() { close(s.strictStop) })
		if s.sched != nil {
			s.sched.stop() // wake every bot and the day barrier so the run unwinds
		}
	}
}

// stopped reports whether a -strict finding has aborted the run.
func (s *sim) stopped() bool {
	select {
	case <-s.strictStop:
		return true
	default:
		return false
	}
}

// writeArtifacts saves the final universe and the digest.
func (s *sim) writeArtifacts() error {
	uniPath := filepath.Join(s.cfg.Out, "universe.yaml")
	s.u.SetFilename(uniPath)
	if err := s.u.Save(); err != nil {
		return fmt.Errorf("saving universe: %w", err)
	}
	return s.writeDigest()
}
