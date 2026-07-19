package galwar

import (
	"testing"
	"time"
)

// TestClockHookDrivesMaintenance pins the one behavior the simulation harness
// depends on: overriding Now shifts the engine's notion of "today", so a
// synthetic clock can make the world experience day boundaries on demand. If
// this ever regresses (a leaf function reading time.Now directly again), the
// sim's day compression silently stops advancing and this catches it.
func TestClockHookDrivesMaintenance(t *testing.T) {
	saved := Now
	defer func() { Now = saved }()

	u := NewUniverse()
	u.Generate(30)
	u.SeedDefaultConfig()

	base := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	Now = func() time.Time { return base }

	// first run establishes "today"; a second run the same day is a no-op
	if !u.RunDailyMaintenance(Now()) {
		t.Fatal("first maintenance run did not execute")
	}
	if u.RunDailyMaintenance(Now()) {
		t.Fatal("second run on the same simulated day should be a no-op")
	}

	// advance the synthetic clock a day; maintenance must run again purely
	// because Now moved - nothing else changed
	base = base.Add(24 * time.Hour)
	if !u.RunDailyMaintenance(Now()) {
		t.Fatal("maintenance did not run after the synthetic clock advanced a day")
	}

	// and the engine's own leaf reads observe the override too: nowUnix and
	// the exported Now agree, so audit/combat/restock stamps move with it
	if got := nowUnix(); got != base.Unix() {
		t.Errorf("nowUnix()=%d did not follow the clock override (want %d)", got, base.Unix())
	}
}
