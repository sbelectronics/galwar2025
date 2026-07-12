// Package ratelimit is a small token-bucket limiter used to throttle command
// input and login attempts. It is deliberately simple: a bucket refills at a
// steady rate up to a burst capacity, and each event consumes one token.
package ratelimit

import (
	"sync"
	"time"
)

// Bucket is a single token bucket. The zero value is not usable; call
// NewBucket.
type Bucket struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	rate     float64 // tokens per second
	last     time.Time
}

// NewBucket creates a bucket that refills at ratePerSec up to burst capacity,
// starting full. A non-positive rate or sub-1 burst is clamped to a safe
// minimum: a throttle primitive must never be constructed into a degenerate
// state that divides by zero or blocks forever.
func NewBucket(ratePerSec, burst float64) *Bucket {
	if ratePerSec <= 0 {
		ratePerSec = 1
	}
	if burst < 1 {
		burst = 1
	}
	return &Bucket{capacity: burst, tokens: burst, rate: ratePerSec, last: time.Now()}
}

func (b *Bucket) refill(now time.Time) {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.last = now
}

// Allow consumes a token if one is available, returning true. Non-blocking.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill(time.Now())
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available, then consumes it. Used to throttle
// (rather than reject) a client's command stream.
func (b *Bucket) Wait() {
	for {
		b.mu.Lock()
		b.refill(time.Now())
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return
		}
		need := (1 - b.tokens) / b.rate
		b.mu.Unlock()
		// rate is guaranteed positive by NewBucket, so need is finite and
		// positive; floor the sleep so a near-full bucket can't busy-spin
		sleep := time.Duration(need * float64(time.Second))
		if sleep < time.Millisecond {
			sleep = time.Millisecond
		}
		time.Sleep(sleep)
	}
}

// Keyed is a set of buckets indexed by key (e.g. client IP), created on
// demand and swept of idle entries.
type Keyed struct {
	mu      sync.Mutex
	buckets map[string]*keyedBucket
	rate    float64
	burst   float64
	lastGC  time.Time
}

type keyedBucket struct {
	bucket *Bucket
	seen   time.Time
}

func NewKeyed(ratePerSec, burst float64) *Keyed {
	return &Keyed{buckets: map[string]*keyedBucket{}, rate: ratePerSec, burst: burst, lastGC: time.Now()}
}

// Allow consumes a token for the given key.
func (k *Keyed) Allow(key string) bool {
	k.mu.Lock()
	now := time.Now()
	// occasional sweep of buckets untouched for 10 minutes
	if now.Sub(k.lastGC) > time.Minute {
		for kk, kb := range k.buckets {
			if now.Sub(kb.seen) > 10*time.Minute {
				delete(k.buckets, kk)
			}
		}
		k.lastGC = now
	}
	kb := k.buckets[key]
	if kb == nil {
		kb = &keyedBucket{bucket: NewBucket(k.rate, k.burst)}
		k.buckets[key] = kb
	}
	kb.seen = now
	k.mu.Unlock()
	return kb.bucket.Allow()
}
