package ratelimit

import (
	"testing"
	"time"
)

func TestBucketBurstThenRefill(t *testing.T) {
	// drive a fake clock so the test is deterministic - no wall-clock sleeps,
	// so neither scheduler jitter nor a stalled runner can make it flaky
	fake := time.Unix(1000, 0)
	b := NewBucket(10, 3) // 10/sec, burst 3
	b.now = func() time.Time { return fake }
	b.last = fake

	// burst: 3 allows with time frozen, then denied (no refill can sneak in)
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("burst token %d denied", i)
		}
	}
	if b.Allow() {
		t.Errorf("4th token allowed past burst of 3")
	}

	// advance exactly 150ms of clock: at 10/sec that's 1.5 tokens
	fake = fake.Add(150 * time.Millisecond)
	if !b.Allow() {
		t.Errorf("token did not refill after 150ms of clock")
	}
	// and no more than that one whole token
	if b.Allow() {
		t.Errorf("more than one token refilled from 150ms")
	}
}

func TestKeyedIsolatesKeys(t *testing.T) {
	k := NewKeyed(1, 2) // 1/sec, burst 2 per key

	// key A exhausts its burst
	if !k.Allow("a") || !k.Allow("a") {
		t.Fatalf("key a burst denied")
	}
	if k.Allow("a") {
		t.Errorf("key a allowed past its burst")
	}
	// key B is unaffected
	if !k.Allow("b") {
		t.Errorf("key b throttled by key a's usage")
	}
}
