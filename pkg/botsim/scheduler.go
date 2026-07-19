package botsim

import (
	"sync"
)

// scheduler paces the fleet. Two modes share one bot-side protocol
// (acquire/finishDay); only the run loop differs:
//
//   - deterministic (default): strict round-robin. Exactly one bot acts at a
//     time, so a run is a pure function of (seed, fleet) and any bug replays.
//   - concurrent: every bot runs free within a day, all hammering Universe.Do;
//     the day boundary is a barrier. Not reproducible - a stress test.
//
// The universe actor is started only in concurrent mode; deterministic mode
// runs every Universe.Do inline on the single active goroutine.
type scheduler struct {
	s          *sim
	concurrent bool
	teardown   chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
}

func newScheduler(s *sim, concurrent bool) *scheduler {
	return &scheduler{s: s, concurrent: concurrent, teardown: make(chan struct{})}
}

// stop tears the fleet down: every bot blocked in acquire/finishDay and every
// day-barrier wakes and exits. Idempotent, so a -strict finding (from any
// goroutine) and the normal end-of-run both call it safely.
func (sc *scheduler) stop() {
	sc.stopOnce.Do(func() { close(sc.teardown) })
}

// acquire is called at each main-command boundary. Deterministic: announce
// readiness and block for the round-robin grant. Concurrent: proceed at once.
// Returns false when the run is tearing down.
func (sc *scheduler) acquire(b *bot) bool {
	if sc.concurrent {
		select {
		case <-sc.teardown:
			return false
		default:
			return true
		}
	}
	select {
	case b.ready <- struct{}{}:
	case <-sc.teardown:
		return false
	}
	select {
	case <-b.grant:
		return true
	case <-sc.teardown:
		return false
	}
}

// finishDay marks a bot done for the day (passed or died) and blocks until the
// next day wakes it. Returns false on teardown.
func (sc *scheduler) finishDay(b *bot, died bool) bool {
	select {
	case b.done <- died:
	case <-sc.teardown:
		return false
	}
	select {
	case <-b.wake:
		return true
	case <-sc.teardown:
		return false
	}
}

// passDay is finishDay for the ordinary out-of-turns pass.
func (sc *scheduler) passDay(b *bot) bool { return sc.finishDay(b, false) }

// run drives the whole simulation: start the fleet, run each day per the mode's
// policy, and call onDayEnd (maintenance, logging) between days with no bot
// active. Tears the fleet down at the end.
func (sc *scheduler) run(days int, onDayEnd func(day int)) {
	s := sc.s
	if sc.concurrent {
		s.u.Start() // serialize Do across parallel bot goroutines
	}
	for _, b := range s.bots {
		sc.wg.Add(1)
		go b.loop()
	}

	for day := 1; day <= days; day++ {
		s.startDay(day)
		if day > 1 {
			// resume every bot parked at yesterday's boundary
			for _, b := range s.bots {
				select {
				case b.wake <- struct{}{}:
				case <-sc.teardown:
				}
			}
		}
		if sc.concurrent {
			sc.runDayConcurrent()
		} else {
			sc.runDayDeterministic()
		}
		// a -strict finding aborts before the nightly pass (which could false-
		// positive on mid-abort state); a normal day runs maintenance/logging
		if s.stopped() {
			break
		}
		onDayEnd(day)
	}

	sc.stop()
	sc.wg.Wait()
}

// runDayDeterministic grants each live bot exactly one action per round,
// round-robin, until every bot has passed or died.
func (sc *scheduler) runDayDeterministic() {
	pending := append([]*bot(nil), sc.s.bots...)
	// prime: every bot reaches its first boundary and announces readiness
	for _, b := range pending {
		select {
		case <-b.ready:
		case <-sc.teardown:
			return
		}
	}
	for len(pending) > 0 && !sc.s.stopped() {
		next := pending[:0]
		for _, b := range pending {
			select {
			case b.grant <- struct{}{}:
			case <-sc.teardown:
				return
			}
			select {
			case <-b.ready: // acted, parked at the next boundary
				next = append(next, b)
			case died := <-b.done: // passed or died: out for the day
				if died {
					sc.s.emitDeath(b)
				}
			case <-sc.teardown:
				return
			}
		}
		pending = next
	}
	// If the run stopped early, bots parked at a boundary (ready sent, awaiting
	// a grant) stay put; onDayEnd runs with no bot active, then run() closes
	// teardown, which releases them to exit.
}

// runDayConcurrent lets every bot run free until it passes or dies, then
// barriers until all have finished the day.
func (sc *scheduler) runDayConcurrent() {
	for _, b := range sc.s.bots {
		select {
		case died := <-b.done:
			if died {
				sc.s.emitDeath(b)
			}
		case <-sc.teardown:
			return
		}
	}
}
