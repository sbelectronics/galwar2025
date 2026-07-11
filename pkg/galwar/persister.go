package galwar

import (
	"log"
	"time"
)

// Persister is the write-behind queue between the universe actor and the
// SQLite store. Engine commands call Universe.MarkDirty after a successful
// mutation; the persister coalesces those notifications and flushes one
// consistent snapshot per quiet interval, so command latency never includes
// a database write and rapid play produces few writes. Crash exposure is
// bounded by the debounce interval. Stop performs a final synchronous flush.

type Persister struct {
	Interval time.Duration

	u     *UniverseType
	store *Store
	kick  chan struct{}
	quit  chan struct{}
	done  chan struct{}
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

func (p *Persister) Start() {
	p.u.SetDirtyNotifier(p.Notify)
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
	close(p.quit)
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
			// debounce: let a burst of commands settle into one write
			t := time.NewTimer(p.Interval)
			select {
			case <-p.quit:
				t.Stop()
				p.flush()
				return
			case <-t.C:
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
