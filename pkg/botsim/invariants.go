package botsim

import (
	"fmt"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// sweepInvariants asserts global truths over the whole universe after each
// day's maintenance, with no bot active. It is where ChaosMonkey earns its
// keep: an action that quietly corrupts state is caught here within a day, even
// if nothing complained at the time (PLAN-BOTS.md 5).
func (s *sim) sweepInvariants(day int) {
	var problems []string
	report := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }

	s.u.Do(func() {
		maxSec := len(s.u.Sectors) - 1
		known := map[galwar.PlayerId]bool{}
		for _, p := range s.u.Players.Players {
			known[p.Id] = true
		}

		allGoods := append(append([]string{}, tradeGoods...),
			galwar.HOLDS, galwar.FIGHTERS, galwar.MINES)
		for _, p := range s.u.Players.Players {
			if p.GetMoney() < 0 {
				report("player %s has negative money %d", p.GetName(), p.GetMoney())
			}
			if p.BankBalance < 0 {
				report("player %s has negative bank balance %d", p.GetName(), p.BankBalance)
			}
			for _, g := range allGoods {
				if q := p.GetQuantity(g); q < 0 {
					report("player %s has negative %s (%d)", p.GetName(), g, q)
				}
			}
			if p.GetFreeHolds() < 0 {
				report("player %s cargo exceeds holds (free=%d)", p.GetName(), p.GetFreeHolds())
			}
			// dead ships and NPC records legitimately park in sector 0
			if p.Sector < 0 || p.Sector > maxSec {
				report("player %s in invalid sector %d", p.GetName(), p.Sector)
			}
		}
		for _, pl := range s.u.Planets.Planets {
			if !known[pl.Owner] {
				report("planet %s owned by unknown player %q", pl.GetName(), pl.Owner)
			}
			if pl.Sector < 1 || pl.Sector > maxSec {
				report("planet %s in invalid sector %d", pl.GetName(), pl.Sector)
			}
			if pl.GetQuantity(galwar.FIGHTERS) < 0 || pl.GetQuantity(galwar.MINES) < 0 {
				report("planet %s has negative defenders", pl.GetName())
			}
		}
		for _, bg := range s.u.Battlegroups.Battlegroups {
			if !known[bg.Owner] {
				report("battlegroup in sector %d owned by unknown player %q", bg.Sector, bg.Owner)
			}
			if bg.Sector < 1 || bg.Sector > maxSec {
				report("battlegroup owned by %q in invalid sector %d", bg.Owner, bg.Sector)
			}
			if bg.GetQuantity(galwar.FIGHTERS) < 0 || bg.GetQuantity(galwar.MINES) < 0 {
				report("battlegroup in sector %d has negative contents", bg.Sector)
			}
		}
		for _, port := range s.u.Ports.Ports {
			for _, cm := range port.Inventory {
				if cm.Quantity < 0 {
					report("port %s has negative %s stock (%d)", port.GetName(), cm.Name, cm.Quantity)
				}
			}
		}
	})

	for _, pr := range problems {
		s.finding(Event{Day: day, T: s.clock().Unix(), Ev: "invariant",
			Extra: map[string]any{"detail": pr}})
	}
}
