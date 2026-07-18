package galwar

import (
	"strings"
	"testing"
	"time"
)

func factionUniverse(t *testing.T) *UniverseType {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	return u
}

// makeActivePlayer registers a player, marks them seen now, and sets their
// value via a money grant.
func makeActivePlayer(t *testing.T, u *UniverseType, name, email string, value int, now time.Time) *Player {
	t.Helper()
	p, err := u.RegisterPlayer(name, email, "")
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
	p.Money = value
	u.TouchLastSeen(p, now.Unix())
	return p
}

func TestCabalDormantForNewbieWorld(t *testing.T) {
	u := factionUniverse(t)
	now := time.Now()
	// five players, but all newbie-value (below the wake bar)
	for i := 0; i < 5; i++ {
		makeActivePlayer(t, u, "Newbie"+string(rune('A'+i)), "n"+string(rune('a'+i))+"@x.com", 67100, now)
	}
	_, leader, quiet := u.factionMetrics(now)
	cabal, _ := u.updateFactionStates(5, leader, quiet)
	if cabal {
		t.Errorf("Cabal woke for a newbie-only world (leader value %d)", leader)
	}
}

func TestCabalWakesForStrongLeader(t *testing.T) {
	u := factionUniverse(t)
	cabal, _ := u.updateFactionStates(3, 600000, false)
	if !cabal {
		t.Errorf("Cabal did not wake for a strong leader in a populated world")
	}
	if u.ConfigInt("cabal_active", 0) != 1 {
		t.Errorf("Cabal active state not persisted")
	}
}

func TestCabalHysteresis(t *testing.T) {
	u := factionUniverse(t)
	// wake it
	if cabal, _ := u.updateFactionStates(4, 600000, false); !cabal {
		t.Fatalf("Cabal did not wake")
	}
	// leader shrinks into the hysteresis band (below wake 536k, above sleep 268k):
	// it must STAY awake
	if cabal, _ := u.updateFactionStates(4, 400000, false); !cabal {
		t.Errorf("Cabal slept inside the hysteresis band (400000)")
	}
	// leader drops below the sleep threshold: now it sleeps
	if cabal, _ := u.updateFactionStates(4, 200000, false); cabal {
		t.Errorf("Cabal stayed awake below the sleep threshold (200000)")
	}
}

func TestCabalSleepsWhenQuietOrEmpty(t *testing.T) {
	u := factionUniverse(t)
	u.updateFactionStates(4, 600000, false) // awake
	if cabal, _ := u.updateFactionStates(4, 600000, true); cabal {
		t.Errorf("Cabal stayed awake in a quiet world")
	}
	u.updateFactionStates(4, 600000, false) // awake again
	if cabal, _ := u.updateFactionStates(1, 600000, false); cabal {
		t.Errorf("Cabal stayed awake after population collapse")
	}
}

func TestRenegadesWakeOnLowerBar(t *testing.T) {
	u := factionUniverse(t)
	// two players, both newbie-value: too weak for the Cabal, but the
	// Renegades stir (population + live game only, no leader bar)
	cabal, ren := u.updateFactionStates(2, 67100, false)
	if cabal {
		t.Errorf("Cabal woke for a newbie world")
	}
	if !ren {
		t.Errorf("Renegades did not wake with 2 active players")
	}
}

func TestFactionMetrics(t *testing.T) {
	u := factionUniverse(t)
	now := time.Now()
	makeActivePlayer(t, u, "Rich One", "r@x.com", 1000000, now)
	makeActivePlayer(t, u, "Poor One", "p@x.com", 5000, now)
	// a dormant player (last seen 10 days ago) doesn't count as active
	old := makeActivePlayer(t, u, "Gone One", "g@x.com", 9000000, now)
	u.TouchLastSeen(old, now.Add(-10*24*time.Hour).Unix())
	// a dead player doesn't count
	dead := makeActivePlayer(t, u, "Dead One", "d@x.com", 9000000, now)
	u.KillPlayer(dead, now.Unix())

	active, leader, quiet := u.factionMetrics(now)
	if active != 2 {
		t.Errorf("active count = %d; want 2 (dormant and dead excluded)", active)
	}
	if leader < 1000000 {
		t.Errorf("leader value = %d; want >= 1000000 (Rich One)", leader)
	}
	if quiet {
		t.Errorf("world marked quiet despite fresh logins")
	}
}

func TestGateFactionPlanetProportional(t *testing.T) {
	u := factionUniverse(t)
	cabal := u.EnsureNPC("cabal")

	// scale to 35% of a 1,000,000 leader = 350,000
	fortress := u.gateFactionPlanet(cabal, 350000, "Cabal Stronghold")
	if fortress == nil {
		t.Fatalf("no stronghold created")
	}
	fighters := fortress.GetQuantity(FIGHTERS)
	// 350000 / 98 ~= 3571 fighters, well under the 15000 cap
	if fighters < 3000 || fighters > 4000 {
		t.Errorf("stronghold fighters = %d; want ~3571 (proportional to a modest leader)", fighters)
	}

	// a huge leader is capped, not a runaway
	u.gateFactionPlanet(cabal, 100000000, "Cabal Stronghold")
	if got := u.factionFortress(cabal).GetQuantity(FIGHTERS); got != u.ConfigInt("cabal_max_planet_fighters", 15000) {
		t.Errorf("stronghold not capped for a huge leader: %d", got)
	}
}

func TestFactionStrikeKills(t *testing.T) {
	u := factionUniverse(t)
	now := time.Now()
	cabal := u.EnsureNPC("cabal")
	fortress := u.gateFactionPlanet(cabal, 5000000, "Cabal Stronghold") // big
	victim := makeActivePlayer(t, u, "Marked One", "m@x.com", 500000, now)
	victim.MoveTo(40)
	victim.SetQuantity(FIGHTERS, 100)

	if !u.factionStrike(cabal, fortress, victim, fortress.GetQuantity(FIGHTERS), now.Unix()) {
		t.Fatalf("overwhelming faction strike did not kill the victim")
	}
	if !victim.IsDead() {
		t.Errorf("victim not dead after faction strike")
	}
	joined := strings.Join(u.TakeNews(victim.Id), "\n")
	if !strings.Contains(joined, "destroyed by The Cabal's fleet") {
		t.Errorf("victim not notified of the faction kill: %q", joined)
	}
}

func TestFactionStrikeSparesFedSpace(t *testing.T) {
	u := factionUniverse(t)
	now := time.Now()
	cabal := u.EnsureNPC("cabal")
	fortress := u.gateFactionPlanet(cabal, 5000000, "Cabal Stronghold")

	// a rich, worthy target - but sheltering in Federation space (sector 5)
	safe := makeActivePlayer(t, u, "Docked One", "d2@x.com", 900000, now)
	safe.MoveTo(5)
	safe.SetQuantity(FIGHTERS, 500)

	if u.factionStrike(cabal, fortress, safe, fortress.GetQuantity(FIGHTERS), now.Unix()) {
		t.Errorf("faction strike killed a player in Federation space (sector 5)")
	}
	if safe.IsDead() || safe.GetQuantity(FIGHTERS) != 500 {
		t.Errorf("player in Federation space took losses: dead=%v fighters=%d", safe.IsDead(), safe.GetQuantity(FIGHTERS))
	}
	// mayhem must skip fed-space players too, even above the floor
	for i := 0; i < 20; i++ {
		u.factionMayhem(cabal, fortress, now, 3)
		if safe.IsDead() || safe.GetQuantity(FIGHTERS) != 500 {
			t.Fatalf("mayhem attacked a fed-space player (round %d)", i)
		}
	}
}

func TestFactionMayhemSparesNewbies(t *testing.T) {
	u := factionUniverse(t)
	now := time.Now()
	cabal := u.EnsureNPC("cabal")
	fortress := u.gateFactionPlanet(cabal, 5000000, "Cabal Stronghold")

	newbie := makeActivePlayer(t, u, "Fresh Meat", "f@x.com", 5000, now) // below floor
	newbie.MoveTo(40)
	newbie.SetQuantity(FIGHTERS, 200)
	rich := makeActivePlayer(t, u, "Fat Cat", "fc@x.com", 900000, now) // above floor
	rich.MoveTo(41)
	rich.SetQuantity(FIGHTERS, 5000)

	// hammer mayhem many times; the newbie must never be touched
	for i := 0; i < 20; i++ {
		u.factionMayhem(cabal, fortress, now, 3)
		if newbie.GetQuantity(FIGHTERS) != 200 || newbie.IsDead() {
			t.Fatalf("mayhem attacked a below-floor newbie (round %d)", i)
		}
	}
}

func TestFedCounterCabalErodes(t *testing.T) {
	u := factionUniverse(t)
	cabal := u.EnsureNPC("cabal")
	fortress := u.gateFactionPlanet(cabal, 1500000, "Cabal Stronghold")
	before := fortress.GetQuantity(FIGHTERS)
	u.fedCounterCabal()
	after := fortress.GetQuantity(FIGHTERS)
	if after >= before {
		t.Errorf("Federation did not erode the Cabal: %d -> %d", before, after)
	}
}

func TestRunFactionAIIntegration(t *testing.T) {
	u := factionUniverse(t)
	now := time.Now()
	// a populated world with a genuine leader
	makeActivePlayer(t, u, "Warlord Prime", "w@x.com", 1500000, now)
	makeActivePlayer(t, u, "Second Place", "s@x.com", 400000, now)
	makeActivePlayer(t, u, "Third Place", "th@x.com", 300000, now)

	u.runFactionAI(now)

	if u.ConfigInt("cabal_active", 0) != 1 {
		t.Errorf("Cabal did not wake in a populated, high-value world")
	}
	// the Cabal now holds a stronghold planet
	cabal := u.Players.GetBySub("npc:cabal")
	if cabal == nil || u.factionFortress(cabal) == nil {
		t.Errorf("Cabal has no stronghold after waking")
	}
}
