package galwar

import (
	"log"
	"math"
	"sync"
	"time"
)

// Daily maintenance - the descendant of the original's nightly MAINT run.
// The daemon checks periodically (and immediately at startup) whether the
// UTC date has changed since the last run; missed days collapse into one
// run, like a BBS whose sysop ran maintenance on the first call of the day.

// bil caps commodity stockpiles, as in the original (GLOBALS.PAS: bil=1.5e9).
const bil = 1500000000

type MaintenanceDaemon struct {
	Interval time.Duration

	u        *UniverseType
	quit     chan struct{}
	done     chan struct{}
	started  bool
	stopOnce sync.Once
}

func NewMaintenanceDaemon(u *UniverseType) *MaintenanceDaemon {
	return &MaintenanceDaemon{
		Interval: time.Minute,
		u:        u,
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start and Stop are idempotent: a second Start is a no-op, Stop without
// Start returns immediately, and a second Stop just waits again.
func (m *MaintenanceDaemon) Start() {
	if m.started {
		return
	}
	m.started = true
	go m.run()
}

func (m *MaintenanceDaemon) Stop() {
	if !m.started {
		return
	}
	m.stopOnce.Do(func() { close(m.quit) })
	<-m.done
}

func (m *MaintenanceDaemon) run() {
	defer close(m.done)
	m.check()
	t := time.NewTicker(m.Interval)
	defer t.Stop()
	for {
		select {
		case <-m.quit:
			return
		case <-t.C:
			m.check()
		}
	}
}

func (m *MaintenanceDaemon) check() {
	m.u.Do(func() {
		m.u.RunDailyMaintenance(time.Now())
	})
}

// RunDailyMaintenance performs the nightly pass if the UTC date has changed
// since the last run. Returns whether it ran. Must run on the universe actor.
func (u *UniverseType) RunDailyMaintenance(now time.Time) bool {
	today := now.UTC().Format("2006-01-02")
	if u.ConfigString("last_maint", "") == today {
		return false
	}

	// update_users (MAINT1.PAS:230-249): turns reset to the daily allowance,
	// not accumulated - turns bought yesterday don't survive the night.
	turnsPerDay := u.ConfigInt("turns_per_day", 250)
	for _, p := range u.Players.Players {
		p.SetQuantity(TURNS, turnsPerDay)
	}

	// update_ports: the real-time restock (commodity.Restock) subsumes the
	// original's nightly +2*prod; this sweep just keeps long-idle ports from
	// accruing unboundedly stale clocks.
	unix := now.Unix()
	for _, p := range u.Ports.Ports {
		p.Restock(unix)
	}

	// increase_planets (MAINT1.PAS:161-227); a dormant owner's planets keep
	// producing at their frozen rate but stop compounding (Tier-1 dormancy)
	maxMines := u.ConfigInt("planet_max_mines", 1000)
	for _, p := range u.Planets.Planets {
		owner := u.Players.GetById(p.Owner)
		frozen := owner != nil && u.IsDormant(owner, now)
		growPlanet(p, maxMines, frozen)
	}

	// Tier-2 dormancy: forfeit the assets of the long-absent and reset them
	// to a starter ship (idempotent via the Expired flag)
	expired := 0
	for _, p := range u.Players.Players {
		if u.IsExpired(p, now) {
			u.ExpirePlayer(p, unix)
			expired++
		}
	}

	// drop delivered news older than a week (the original's trim_message
	// used 3 days; we're a little more generous)
	u.trimNews(now.Add(-7 * 24 * time.Hour).Unix())
	u.trimAudit()

	u.SetConfig("last_maint", today) // also marks dirty
	log.Printf("daily maintenance for %s: %d players reset to %d turns, %d ports restocked, %d planets grown, %d expired",
		today, len(u.Players.Players), turnsPerDay, len(u.Ports.Ports), len(u.Planets.Planets), expired)
	return true
}

// growPlanet applies one day of planetary production, faithful to
// increase_planets (MAINT1.PAS:161-227):
//   - stockpiles grow by their production rates (capped)
//   - production compounds when the stockpile exceeds 10x the rate
//   - fighter/mine production derives from the weighted commodity index
//     t = ore/4 + org/2 + eqp; fighters = t/5, mines = t/1000
//
// When frozen (a dormant owner), stockpiles still accrue at the current rate
// but the rate itself stops compounding and the derived production stops
// recalculating - the planet coasts instead of snowballing while unattended.
func growPlanet(p *Planet, maxMines int, frozen bool) {
	ore := p.GetCommodity(ORE)
	org := p.GetCommodity(ORGANICS)
	eqp := p.GetCommodity(EQUIPMENT)
	fighters := p.GetCommodity(FIGHTERS)
	mines := p.GetCommodity(MINES)

	if ore == nil || org == nil || eqp == nil {
		return // not a producing planet; nothing to grow
	}

	for _, c := range []*Commodity{ore, org, eqp} {
		c.Quantity = min(c.Quantity+c.Prod, bil)
	}
	if fighters != nil {
		fighters.Quantity = min(fighters.Quantity+fighters.Prod, bil)
	}
	if mines != nil {
		mines.Quantity = min(mines.Quantity+mines.Prod, maxMines)
	}

	if frozen {
		return
	}

	for _, c := range []*Commodity{ore, org, eqp} {
		if c.Quantity > c.Prod*10 {
			c.Prod += (c.Quantity - c.Prod*10) / 10
		}
	}

	t := int(math.Round(float64(ore.Prod)/4 + float64(org.Prod)/2 + float64(eqp.Prod)))
	if fighters != nil {
		fighters.Prod = t / 5
	}
	if mines != nil {
		mines.Prod = t / 1000
	}
}
