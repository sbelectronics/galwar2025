package botsim

import (
	"bufio"
	"encoding/json"
	"io"
	"sort"
	"sync"
)

// findingKinds are the event kinds that count as bugs (PLAN-BOTS.md section 5):
// they fail the run under -strict and are surfaced verbatim in the digest.
var findingKinds = map[string]bool{
	"error_unexpected": true,
	"desync":           true,
	"invariant":        true,
	"stuck":            true,
}

// Logger writes the structured event stream (events.jsonl) and retains every
// event in memory for the end-of-run digest. It is safe for concurrent use so
// -concurrent bots can log from their own goroutines.
type Logger struct {
	mu       sync.Mutex
	w        *bufio.Writer
	closer   io.Closer
	events   []Event
	findings int
}

// Event is one structured log record. Core fields are explicit; anything else a
// specific event needs rides in Extra, which is flattened into the JSON object.
// Marshaling sorts keys, so a deterministic run produces byte-identical JSONL.
type Event struct {
	Day   int
	T     int64 // simulated unix seconds when emitted
	Bot   string
	Class string
	Ev    string
	Extra map[string]any
}

// MarshalJSON flattens the core fields and Extra into one object with sorted
// keys (so identical runs are byte-identical).
func (e Event) MarshalJSON() ([]byte, error) {
	m := map[string]any{"day": e.Day, "t": e.T, "ev": e.Ev}
	if e.Bot != "" {
		m["bot"] = e.Bot
	}
	if e.Class != "" {
		m["class"] = e.Class
	}
	for k, v := range e.Extra {
		if _, taken := m[k]; !taken {
			m[k] = v
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	buf := []byte{'{'}
	for i, k := range keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(m[k])
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')
		buf = append(buf, vb...)
	}
	buf = append(buf, '}')
	return buf, nil
}

// NewLogger writes events to w (typically an events.jsonl file). closer, if
// non-nil, is closed by Close.
func NewLogger(w io.Writer, closer io.Closer) *Logger {
	return &Logger{w: bufio.NewWriter(w), closer: closer}
}

// Emit records one event: appended to the in-memory log and written as a JSONL
// line. Finding-class events bump the findings counter.
func (l *Logger) Emit(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
	if findingKinds[e.Ev] {
		l.findings++
	}
	if l.w != nil {
		line, err := json.Marshal(e)
		if err == nil {
			l.w.Write(line)
			l.w.WriteByte('\n')
		}
	}
}

// Findings returns the number of finding-class events emitted so far.
func (l *Logger) Findings() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.findings
}

// Events returns a copy of the recorded events (for the digest).
func (l *Logger) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Event(nil), l.events...)
}

// Flush writes buffered output.
func (l *Logger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w != nil {
		l.w.Flush()
	}
}

// Close flushes and closes the underlying writer.
func (l *Logger) Close() error {
	l.Flush()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}
