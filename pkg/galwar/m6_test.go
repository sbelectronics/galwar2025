package galwar

import (
	"strings"
	"testing"
	"time"
)

func m6Universe(t *testing.T) *UniverseType {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	return u
}

func TestDormancyHidesShip(t *testing.T) {
	u := m6Universe(t)
	now := time.Now()

	active, _ := u.RegisterPlayer("Active", "a@example.com", "")
	dormant, _ := u.RegisterPlayer("Dozer", "d@example.com", "")
	active.MoveTo(50)
	dormant.MoveTo(50)
	nearly, _ := u.RegisterPlayer("Nearly", "n@example.com", "")
	nearly.MoveTo(70) // elsewhere, to keep sector 50's counts clean
	u.TouchLastSeen(active, now.Unix())
	u.TouchLastSeen(dormant, now.Add(-6*24*time.Hour).Unix()) // 6 days: past the 5-day threshold
	u.TouchLastSeen(nearly, now.Add(-4*24*time.Hour).Unix())  // 4 days: not yet dormant

	if !u.IsDormant(dormant, now) {
		t.Fatalf("6-days-absent player not dormant (default threshold 5)")
	}
	if u.IsDormant(nearly, now) {
		t.Fatalf("4-days-absent player marked dormant (default threshold 5)")
	}
	if u.IsDormant(active, now) {
		t.Fatalf("just-seen player marked dormant")
	}

	// dormant ship hidden from the sector, active ship visible
	vis := u.GetVisibleObjectsInSector(50, TYPE_PLAYER, active, now)
	for _, o := range vis {
		if o == dormant {
			t.Errorf("dormant ship shown in sector")
		}
	}
	// GetObjectsInSector (raw) still contains both
	if len(u.GetObjectsInSector(50, TYPE_PLAYER)) != 2 {
		t.Errorf("raw sector lookup should include the dormant ship")
	}

	// dormant player can't be attacked
	active.SetQuantity(FIGHTERS, 1000)
	if _, err := u.AttackPlayer(active, dormant.Id, 100); err == nil {
		t.Errorf("attack on a dormant player allowed")
	}

	// login clears dormancy instantly
	u.TouchLastSeen(dormant, now.Unix())
	if u.IsDormant(dormant, now) {
		t.Errorf("dormancy not cleared on login")
	}
}

func TestExpirySweep(t *testing.T) {
	u := m6Universe(t)
	now := time.Now()

	p, _ := u.RegisterPlayer("Ghost", "g@example.com", "")
	p.MoveTo(50)
	p.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(p, 50, "Homestead"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	if err := u.AdjustBattlegroup(p, 60, FIGHTERS, 50); err != nil {
		t.Fatalf("battlegroup: %v", err)
	}
	p.Money = 999999
	u.TouchLastSeen(p, now.Add(-31*24*time.Hour).Unix()) // 31 days: past the 30-day threshold

	// dormant but not yet expired at, say, 20 days
	u.TouchLastSeen(p, now.Add(-20*24*time.Hour).Unix())
	if u.IsExpired(p, now) {
		t.Fatalf("20-days-absent player expired early (default threshold 30)")
	}
	u.TouchLastSeen(p, now.Add(-31*24*time.Hour).Unix())
	if !u.IsExpired(p, now) {
		t.Fatalf("31-days-absent player not expired (default threshold 30)")
	}

	u.ExpirePlayer(p, now.Unix())

	// forfeited: planet to an NPC, battlegroup to renegades
	if heir := u.Players.GetById(u.Planets.Planets[0].Owner); heir == nil || !heir.IsNPC() {
		t.Errorf("expired player's planet not forfeited to an NPC")
	}
	if bg, _ := u.GetBattlegroup(p, 60, false); bg != nil {
		t.Errorf("expired player still owns the battlegroup")
	}
	// reset to a fresh starter ship at Sol
	if p.Money != 35000 || p.GetQuantity(FIGHTERS) != 200 || p.Sector != 1 {
		t.Errorf("expiry did not reset the ship: money=%d fighters=%d sector=%d",
			p.Money, p.GetQuantity(FIGHTERS), p.Sector)
	}
	if !p.Expired {
		t.Errorf("Expired flag not set")
	}

	// idempotent: a second sweep does nothing (no re-forfeit, no re-reset)
	p.Money = 12345
	if u.IsExpired(p, now) {
		t.Errorf("already-expired player still reports expired")
	}
	u.ExpirePlayer(p, now.Unix())
	if p.Money != 12345 {
		t.Errorf("second expiry re-reset the ship")
	}

	// return clears the flag
	u.TouchLastSeen(p, now.Unix())
	if p.Expired {
		t.Errorf("Expired flag not cleared on return")
	}
}

func TestPlanetFreezeWhileDormant(t *testing.T) {
	u := m6Universe(t)
	now := time.Now()
	owner, _ := u.RegisterPlayer("Farmer", "f@example.com", "")
	owner.MoveTo(50)
	owner.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(owner, 50, "Farm"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]
	// hoard so the ramp would fire if active
	planet.SetQuantity(ORE, 500)

	// active owner: prod ramps
	u.TouchLastSeen(owner, now.Unix())
	growPlanet(planet, 1000, false)
	rampedProd := planet.GetCommodity(ORE).Prod
	if rampedProd <= 1 {
		t.Fatalf("active planet prod did not ramp: %d", rampedProd)
	}

	// dormant owner: stock still accrues, prod frozen
	planet.SetQuantity(ORE, 999999)
	frozenProd := planet.GetCommodity(ORE).Prod
	qtyBefore := planet.GetQuantity(ORE)
	growPlanet(planet, 1000, true)
	if planet.GetCommodity(ORE).Prod != frozenProd {
		t.Errorf("frozen planet prod changed: %d -> %d", frozenProd, planet.GetCommodity(ORE).Prod)
	}
	if planet.GetQuantity(ORE) != min(qtyBefore+frozenProd, bil) {
		t.Errorf("frozen planet stock did not accrue at the frozen rate")
	}
}

func TestPlayerValueAndRankings(t *testing.T) {
	u := m6Universe(t)
	now := time.Now()
	rich, _ := u.RegisterPlayer("Rich", "r@example.com", "")
	poor, _ := u.RegisterPlayer("Poor", "p@example.com", "")
	rich.Money = 1000000
	poor.Money = 100

	if u.PlayerValue(rich) <= u.PlayerValue(poor) {
		t.Errorf("richer player did not value higher")
	}

	ranks := u.RankedPlayers(now)
	if len(ranks) != 2 || ranks[0].Name != "Rich" {
		t.Fatalf("rankings wrong: %+v", ranks)
	}

	// dead and NPC players are excluded
	u.KillPlayer(poor, now.Unix())
	u.EnsureNPC("cabal")
	ranks = u.RankedPlayers(now)
	if len(ranks) != 1 || ranks[0].Name != "Rich" {
		t.Errorf("rankings included dead/NPC players: %+v", ranks)
	}
}

func TestBanBlocksAndAudits(t *testing.T) {
	u := m6Universe(t)
	u.SetConfig("admins", "boss@example.com")
	admin, _ := u.RegisterPlayer("Boss", "boss@example.com", "")
	bad, _ := u.RegisterPlayer("Baddie", "bad@example.com", "")

	if !u.IsAdmin(admin) {
		t.Fatalf("configured admin not recognized")
	}
	if u.IsAdmin(bad) {
		t.Fatalf("non-admin recognized as admin")
	}

	// the engine refuses non-admin callers, not just the UI
	if err := u.SetBanned(bad, "Boss", true); err == nil {
		t.Errorf("non-admin allowed to ban")
	}
	if err := u.SetBanned(nil, "Boss", true); err == nil {
		t.Errorf("nil caller allowed to ban")
	}
	if err := u.ForceRename(bad, "Boss", "Whatever"); err == nil {
		t.Errorf("non-admin allowed to force-rename")
	}

	if err := u.SetBanned(admin, "Baddie", true); err != nil {
		t.Fatalf("ban: %v", err)
	}
	if !bad.Banned {
		t.Errorf("ban did not take")
	}
	// can't ban another sysop
	if err := u.SetBanned(admin, "Boss", true); err == nil {
		t.Errorf("banning a sysop allowed")
	}

	// the ban is audited
	found := false
	for _, a := range u.Audit {
		if a.Action == "ban" && a.Detail == "Baddie" {
			found = true
		}
	}
	if !found {
		t.Errorf("ban not recorded in the audit log")
	}
}

func TestReportsAndForceRename(t *testing.T) {
	u := m6Universe(t)
	u.SetConfig("admins", "boss@example.com")
	admin, _ := u.RegisterPlayer("Boss", "boss@example.com", "")
	reporter, _ := u.RegisterPlayer("Reporter", "rep@example.com", "")
	offender, _ := u.RegisterPlayer("Rudename", "off@example.com", "")

	if err := u.FileReport(nil, "Rudename", "x"); err == nil {
		t.Errorf("nil reporter accepted (would panic on GetName)")
	}
	if err := u.FileReport(reporter, "Rudename", "offensive handle"); err != nil {
		t.Fatalf("file report: %v", err)
	}
	if len(u.OpenReports()) != 1 {
		t.Fatalf("report not filed")
	}

	// force-rename must pass moderation
	if err := u.ForceRename(admin, "Rudename", "fuck"); err == nil {
		t.Errorf("force-rename to a profane handle allowed")
	}
	if err := u.ForceRename(admin, "Rudename", "Reformed Citizen"); err != nil {
		t.Fatalf("force-rename: %v", err)
	}
	if offender.GetName() != "Reformed Citizen" {
		t.Errorf("rename did not take: %q", offender.GetName())
	}
	// renaming resolves the report against the old handle
	if len(u.OpenReports()) != 0 {
		t.Errorf("report not resolved after force-rename")
	}
}

// TestModerationRejectionsAudited: a rejected handle or planet name leaves an
// audit entry naming the input and the rule that fired - the feedback loop the
// sysop tunes profanity_extra/safelist_extra from. The input is stored
// %q-escaped, so an injection attempt can't reach the sysop's screen through
// its own audit record.
func TestModerationRejectionsAudited(t *testing.T) {
	u := m6Universe(t)

	if _, err := u.RegisterPlayer("c u n t", "who@example.com", ""); err == nil {
		t.Fatalf("bad handle accepted")
	}
	p, _ := u.RegisterPlayer("Genesis Guy", "gg@example.com", "")
	p.MoveTo(20)
	if err := u.UseGenesisDevice(p, 20, "esc\x1b[31mape"); err == nil {
		t.Fatalf("bad planet name accepted")
	}

	for action, frag := range map[string]string{
		"reject-handle":      "c u n t",
		"reject-planet-name": "esc",
	} {
		found := false
		for _, a := range u.Audit {
			if a.Action == action && strings.Contains(a.Detail, frag) {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s audit entry recorded", action)
		}
	}
	// escaped means escaped: no raw control byte may survive into the trail
	for _, a := range u.Audit {
		if strings.ContainsRune(a.Detail, 0x1b) {
			t.Errorf("audit detail contains a raw escape byte: %q", a.Detail)
		}
	}
}
