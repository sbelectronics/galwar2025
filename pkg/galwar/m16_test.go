package galwar

import (
	"testing"
	"time"
)

// M16: per-player news cap.

func TestPerPlayerNewsCap(t *testing.T) {
	u := NewUniverse()
	u.Generate(30)
	u.SeedDefaultConfig()
	now := time.Now().Unix()

	victim, _ := u.RegisterPlayer("Victim", "v@x.com", "")
	noisy, _ := u.RegisterPlayer("Noisy", "n@x.com", "")

	// the victim gets one important notice, delivered or not
	u.AddNews(victim.Id, now, "IMPORTANT: you were killed")

	// a noisy attacker floods their own news far past the per-player cap
	for i := 0; i < maxNewsPerPlayer*3; i++ {
		u.AddNews(noisy.Id, now+int64(i), "spam")
	}

	// the victim's single notice survives - the flood only evicted the
	// attacker's own oldest items
	found := false
	victimCount := 0
	for _, n := range u.News {
		if n.Player == victim.Id {
			victimCount++
			if n.Msg == "IMPORTANT: you were killed" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("victim's notice was evicted by another player's flood")
	}
	if victimCount != 1 {
		t.Errorf("victim has %d news items, want 1", victimCount)
	}

	// the noisy player is capped at maxNewsPerPlayer
	noisyCount := 0
	for _, n := range u.News {
		if n.Player == noisy.Id {
			noisyCount++
		}
	}
	if noisyCount != maxNewsPerPlayer {
		t.Errorf("noisy player has %d news items, want the cap %d", noisyCount, maxNewsPerPlayer)
	}
}

func TestNewsCapKeepsNewest(t *testing.T) {
	u := NewUniverse()
	u.Generate(30)
	u.SeedDefaultConfig()
	p, _ := u.RegisterPlayer("Chatty", "c@x.com", "")

	for i := 0; i < maxNewsPerPlayer+10; i++ {
		u.AddNews(p.Id, int64(i), "msg")
	}
	// the very latest item must be present (cap drops oldest, keeps newest)
	recent := u.RecentNews(p.Id, maxNewsPerPlayer)
	if len(recent) != maxNewsPerPlayer {
		t.Errorf("kept %d items, want %d", len(recent), maxNewsPerPlayer)
	}
}
