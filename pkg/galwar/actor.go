package galwar

import (
	"fmt"
	"log"
	"runtime/debug"
)

// The universe actor: exactly one goroutine mutates the universe. Sessions
// (console, and later WebSocket) gather player intent interactively, then
// submit one complete command via Do/DoErr. Because commands execute one at
// a time, every command is atomic with respect to every other - no locks,
// no lock ordering, no torn state. This is also why the old "TODO: lock"
// comments are gone: the design makes them unnecessary.
//
// Rules for command functions:
//   - never prompt or block inside a command (gather inputs first)
//   - never call Do/DoErr from inside a command (it would deadlock)

type task struct {
	fn       func()
	done     chan struct{}
	panicVal any
	stack    []byte
}

// Start launches the universe actor goroutine. Until Start is called, Do
// runs its function directly on the caller's goroutine, which keeps
// single-threaded tests simple.
func (u *UniverseType) Start() {
	u.tasks = make(chan *task)
	go func() {
		for t := range u.tasks {
			u.runTask(t)
		}
	}()
}

func (u *UniverseType) runTask(t *task) {
	defer close(t.done)
	defer func() {
		if r := recover(); r != nil {
			// Keep the actor alive, but don't swallow the failure: record it
			// so Do can re-raise it on the submitting goroutine. Otherwise a
			// panicking command would make DoErr return nil and the caller
			// would mistake a crash for success.
			t.panicVal = r
			t.stack = debug.Stack()
		}
	}()
	t.fn()
}

// Do runs fn on the universe actor goroutine and waits for it to complete.
// All reads and writes of universe state must go through Do (or DoErr) once
// Start has been called. If fn panics, the actor survives and the panic is
// re-raised here, on the caller's goroutine.
func (u *UniverseType) Do(fn func()) {
	if u.tasks == nil {
		fn()
		return
	}
	t := &task{fn: fn, done: make(chan struct{})}
	u.tasks <- t
	<-t.done
	if t.panicVal != nil {
		log.Printf("BUG: panic in universe command: %v\n%s", t.panicVal, t.stack)
		panic(fmt.Sprintf("universe command panicked: %v", t.panicVal))
	}
}

// DoErr is Do for functions that return an error.
func (u *UniverseType) DoErr(fn func() error) error {
	var err error
	u.Do(func() {
		err = fn()
	})
	return err
}
