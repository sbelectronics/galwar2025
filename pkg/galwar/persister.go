package galwar

import (
	"log"
	"sync"
	"time"
)

// Persister is the write-behind queue between the universe actor and the
// SQLite store. Engine commands call Universe.MarkDirty after a successful
// mutation; the persister coalesces those notifications and writes at most
// one consistent snapshot per interval (throttle, not trailing-edge
// debounce: continuous play must not postpone the flush, or crash exposure
// would become unbounded). Command latency never includes a database write,
// and staleness on crash is bounded by the interval. Stop performs a final
// synchronous flush.

type Persister struct {
	Interval time.Duration

	u        *UniverseType
	store    *Store
	kick     chan struct{}
	quit     chan struct{}
	done     chan struct{}
	started  bool
	stopOnce sync.Once
}

func NewPersister(u *UniverseType, store *Store) *Persister {
	return &Persister{
		Interval: time.Second,
		u:        u,
		store:    store,
		kick:     make(chan struct{}, 1),
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start and Stop are idempotent: a second Start is a no-op, Stop without
// Start returns immediately, and a second Stop just waits again.
func (p *Persister) Start() {
	if p.started {
		return
	}
	p.started = true
	// through Do: the notifier field belongs to the universe, and the actor
	// may already be executing commands that read it via MarkDirty
	p.u.Do(func() {
		p.u.SetDirtyNotifier(p.Notify)
	})
	go p.run()
}

// Notify is safe to call from any goroutine, including the actor; it never
// blocks.
func (p *Persister) Notify() {
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// Stop flushes any pending state and shuts the persister down.
func (p *Persister) Stop() {
	if !p.started {
		return
	}
	p.stopOnce.Do(func() { close(p.quit) })
	<-p.done
}

func (p *Persister) run() {
	defer close(p.done)
	for {
		select {
		case <-p.quit:
			p.flush()
			return
		case <-p.kick:
			// throttle: let a burst of commands settle into one write
			t := time.NewTimer(p.Interval)
			select {
			case <-p.quit:
				t.Stop()
				p.flush()
				return
			case <-t.C:
			}
			// fold any notifications that arrived during the wait into this
			// flush - the snapshot below sees their mutations, so a leftover
			// kick would only trigger a redundant write of identical state.
			// Anything marked dirty after the snapshot re-kicks and gets its
			// own flush next cycle.
			select {
			case <-p.kick:
			default:
			}
			p.flush()
		}
	}
}

func (p *Persister) flush() {
	var snap *Snapshot
	p.u.Do(func() {
		snap = p.u.Snapshot()
	})
	if err := p.store.SaveUniverse(snap); err != nil {
		// the in-memory universe is still authoritative; a MarkDirty from
		// the next command will retry
		log.Printf("persist failed: %v", err)
	}
}
