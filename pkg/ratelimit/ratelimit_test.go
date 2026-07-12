package ratelimit

import (
	"testing"
	"time"
)

func TestBucketBurstThenRefill(t *testing.T) {
	b := NewBucket(10, 3) // 10/sec, burst 3

	// burst: 3 immediate allows, then denied
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("burst token %d denied", i)
		}
	}
	if b.Allow() {
		t.Errorf("4th immediate token allowed past burst of 3")
	}

	// after ~150ms at 10/sec, ~1 token refills
	time.Sleep(150 * time.Millisecond)
	if !b.Allow() {
		t.Errorf("token did not refill after 150ms")
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
