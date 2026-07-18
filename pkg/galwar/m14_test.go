package galwar

import (
	"testing"
	"time"
)

// M14: the ship's computer reports.

func computerUniverse(t *testing.T) (*UniverseType, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(80)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Commander", "c@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	u.TouchLastSeen(p, time.Now().Unix())
	return u, p
}

func TestPlayerForces(t *testing.T) {
	u, p := computerUniverse(t)
	// a planet and a sector defense force
	planet := u.NewPlanet(p.Id, 40, "Homeworld")
	planet.SetQuantity(FIGHTERS, 500)
	planet.SetQuantity(MINES, 20)
	p.MoveTo(41)
	p.SetQuantity(FIGHTERS, 1000)
	if err := u.AdjustBattlegroup(p, 41, FIGHTERS, 300); err != nil {
		t.Fatalf("deploy force: %v", err)
	}

	forces := u.PlayerForces(p)
	if len(forces) != 2 {
		t.Fatalf("PlayerForces returned %d, want 2: %+v", len(forces), forces)
	}
	var planetSeen, forceSeen bool
	for _, f := range forces {
		switch f.Kind {
		case "Planet":
			planetSeen = true
			if f.Sector != 40 || f.Fighters != 500 || f.Mines != 20 {
				t.Errorf("planet force wrong: %+v", f)
			}
		case "Defense Force":
			forceSeen = true
			if f.Sector != 41 || f.Fighters != 300 {
				t.Errorf("defense force wrong: %+v", f)
			}
		}
	}
	if !planetSeen || !forceSeen {
		t.Errorf("missing a force kind: %+v", forces)
	}

	// another player's assets don't appear
	other, _ := u.RegisterPlayer("Rival", "r@x.com", "")
	u.NewPlanet(other.Id, 50, "Not Yours")
	if got := len(u.PlayerForces(p)); got != 2 {
		t.Errorf("rival assets leaked into forces report: %d", got)
	}
}

func TestNearestPort(t *testing.T) {
	u, p := computerUniverse(t)
	// standing on Sol (sector 1, which has a port): nearest is distance 0
	p.MoveTo(1)
	sec, dist, _, found := u.NearestPort(1, nil)
	if !found || dist != 0 || sec != 1 {
		t.Errorf("nearest-any from a port sector: sec=%d dist=%d found=%v", sec, dist, found)
	}

	// nearest port selling ore: must be reachable and actually sell ore
	sec, _, _, found = u.NearestPort(1, PortSells(ORE))
	if !found {
		t.Fatalf("no ore-selling port reachable from sector 1")
	}
	var port *Port
	for _, pp := range u.Ports.Ports {
		if pp.Sector == sec {
			port = pp
		}
	}
	if port == nil {
		t.Fatalf("nearest ore port not found at sector %d", sec)
	}
	if c := port.GetCommodity(ORE); c == nil || !c.Sell {
		t.Errorf("nearest 'selling ore' port does not sell ore: %+v", c)
	}
}

func TestNearestPortNoneReachable(t *testing.T) {
	u := NewUniverse()
	u.Generate(30)
	u.SeedDefaultConfig()
	// a commodity no port sells (fighters aren't a port good)
	if _, _, _, found := u.NearestPort(1, PortSells(FIGHTERS)); found {
		t.Errorf("found a port selling fighters, which no trading port does")
	}
}

func TestUniverseStats(t *testing.T) {
	u, p := computerUniverse(t)
	now := time.Now()

	dormant, _ := u.RegisterPlayer("Sleeper", "s@x.com", "")
	u.TouchLastSeen(dormant, now.Add(-10*24*time.Hour).Unix())
	dead, _ := u.RegisterPlayer("Ghost", "g@x.com", "")
	u.KillPlayer(dead, now.Unix())

	s := u.Stats(now)
	if s.Sectors != 80 {
		t.Errorf("Sectors=%d, want 80", s.Sectors)
	}
	if s.Ports == 0 {
		t.Errorf("Ports=%d, want > 0", s.Ports)
	}
	if s.ActiveTraders != 1 { // only the Commander; dormant and dead excluded
		t.Errorf("ActiveTraders=%d, want 1", s.ActiveTraders)
	}
	if s.TurnsPerDay != 250 {
		t.Errorf("TurnsPerDay=%d, want 250", s.TurnsPerDay)
	}
	_ = p
}

func TestRecentNews(t *testing.T) {
	u, p := computerUniverse(t)
	now := time.Now().Unix()
	for i := 0; i < 25; i++ {
		u.AddNews(p.Id, now, "event "+string(rune('A'+i%26)))
	}
	// delivering doesn't erase the record for RecentNews
	u.TakeNews(p.Id)

	recent := u.RecentNews(p.Id, 20)
	if len(recent) != 20 {
		t.Errorf("RecentNews returned %d, want the 20 most recent", len(recent))
	}
	// the most recent item is last (oldest-first ordering)
	if recent[len(recent)-1] != "event "+string(rune('A'+24%26)) {
		t.Errorf("RecentNews not oldest-first: last=%q", recent[len(recent)-1])
	}
	// another player's news is not included
	other, _ := u.RegisterPlayer("Nobody", "n@x.com", "")
	if got := len(u.RecentNews(other.Id, 20)); got != 0 {
		t.Errorf("RecentNews leaked another player's items: %d", got)
	}
}
