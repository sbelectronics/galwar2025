package botsim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// smokeConfig is a small, fast, deterministic run used by the integration tests.
func smokeConfig(dir string) Config {
	return Config{
		Days:    3,
		Seed:    99,
		Sectors: 200,
		Fleet:   map[string]int{"trader": 3},
		Out:     dir,
	}
}

// TestSmokeRun is the payoff test: a full deterministic run must complete with
// no findings, produce its artifacts, and leave a universe that round-trips
// through Save/Load. This exercises the whole stack - scheduler, botTerm, the
// real ConsoleUI, perception, memory, and the Trader - the way a real player
// session does.
func TestSmokeRun(t *testing.T) {
	dir := t.TempDir()
	s, err := New(smokeConfig(dir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	findings, err := s.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if findings != 0 {
		t.Errorf("smoke run reported %d findings; see %s/events.jsonl", findings, dir)
	}

	for _, name := range []string{"events.jsonl", "digest.txt", "universe.yaml"} {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected artifact %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("artifact %s is empty", name)
		}
	}

	// the saved universe must load cleanly (validate passes) - proves the run
	// left the world in a consistent, persistable state
	u := galwar.NewUniverse()
	u.SetFilename(filepath.Join(dir, "universe.yaml"))
	if err := u.Load(); err != nil {
		t.Errorf("final universe failed to reload: %v", err)
	}
}

// TestDeterministicRunsMatch pins reproducibility: two runs with the same seed
// and fleet must produce byte-identical event logs. The synthetic clock makes
// even timestamps reproducible, so any divergence points at unseeded
// nondeterminism (a stray map iteration feeding a decision, say).
func TestDeterministicRunsMatch(t *testing.T) {
	run := func() []byte {
		dir := t.TempDir()
		s, err := New(smokeConfig(dir))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := s.Run(); err != nil {
			t.Fatalf("Run: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
		if err != nil {
			t.Fatalf("read events: %v", err)
		}
		return data
	}
	a := run()
	b := run()
	if len(a) != len(b) || string(a) != string(b) {
		t.Errorf("two same-seed runs diverged: %d vs %d bytes of events.jsonl", len(a), len(b))
	}
}

// TestConcurrentRunNoCorruption stress-tests the actor model: all bots run in
// parallel, hammering Universe.Do. Concurrency drift (desyncs, the odd error)
// is expected and fine, but the run must complete without a panic or deadlock
// and must never corrupt state - no invariant violation may survive. (Run
// under `go test -race` in CI for the data-race guarantee too.)
func TestConcurrentRunNoCorruption(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Days: 4, Seed: 21, Sectors: 250,
		Fleet: map[string]int{"trader": 2, "aggressor": 2, "chaos": 1}, Out: dir, Concurrent: true}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if strings.Contains(string(data), `"ev":"invariant"`) {
		t.Errorf("concurrent run corrupted state (invariant violation): see %s/events.jsonl", dir)
	}
}

// TestStrictModeStopsWithoutDeadlock: a -strict finding mid-run (injected -
// the bots are healthy enough now that organic findings can't be relied on)
// must tear the whole fleet down and return, not hang on a day barrier
// waiting for a bot that already exited. The goroutine+timeout makes a
// regression fail fast instead of blocking the suite.
func TestStrictModeStopsWithoutDeadlock(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Days: 30, Seed: 9, Sectors: 250,
		Fleet: map[string]int{"trader": 2, "aggressor": 2, "chaos": 1},
		Out:   dir, Concurrent: true, Strict: true}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// inject the finding shortly after the run starts, from another goroutine -
	// exactly how a real finding arrives in concurrent mode
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.finding(Event{Day: 1, Ev: "invariant", Extra: map[string]any{"detail": "injected for test"}})
	}()

	type result struct {
		findings int
		err      error
	}
	ch := make(chan result, 1)
	go func() {
		f, err := s.Run()
		ch <- result{f, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("Run: %v", r.err)
		}
		if r.findings == 0 {
			t.Errorf("expected the injected finding to be recorded")
		}
	case <-time.After(90 * time.Second):
		t.Fatal("strict run deadlocked (did not return within 90s)")
	}
}
