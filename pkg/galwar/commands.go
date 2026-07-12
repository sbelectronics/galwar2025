package galwar

import (
	"fmt"
	"time"
)

// Engine command entry points. The engine is the sole authority on game
// rules; front-ends only prompt and format. Each command validates
// everything before mutating anything, and must be invoked through the
// universe actor (Universe.Do / Universe.DoErr) once Start has been called.

// spendTurn charges one turn, or refuses if the player has none, matching
// the original's passturn gate (TWLIB1.PAS:1644). As in passturn, spending
// a turn heals one point on every damaged ship system.
func (u *UniverseType) spendTurn(p *Player) error {
	if p.GetQuantity(TURNS) < 1 {
		return NewGameError(ErrNoTurns, "You don't have any turns left! Come back tomorrow.")
	}
	p.AdjustQuantity(TURNS, -1)
	p.HealSystems()
	return nil
}

// CheckSystem refuses an action when the required ship system is damaged.
func (u *UniverseType) CheckSystem(p *Player, sys int) error {
	p.ensureSystems()
	if p.Systems[sys] > 0 {
		return NewGameError(ErrUnknown, fmt.Sprintf("Your %s is damaged! (%d turns until repaired)", SystemNames[sys], p.Systems[sys]))
	}
	return nil
}

// MovePlayer moves a player to an adjacent sector, validating the warp.
// Movement costs 1 turn and requires working engines, as in the original.
// Entering a sector defended by another player's forces is contested: their
// fighters fight at the edge (conflict, TWLIB1.PAS:830) and their mines
// detonate on arrival (mine_check). The returned report narrates any
// combat; the player may be dead when this returns.

func (u *UniverseType) MovePlayer(p *Player, dest int) ([]string, error) {
	if p.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if p.Sector < 1 || p.Sector >= len(u.Sectors) {
		// validate() guards the load path; this guards programmatic callers
		return nil, NewGameError(ErrNotFound, "You are in an invalid sector!")
	}
	if dest < 1 || dest >= len(u.Sectors) {
		return nil, NewGameError(ErrNotFound, "There is no such sector!")
	}
	if !u.Sectors[p.Sector].HasWarp(dest) {
		return nil, NewGameError(ErrNotFound, "You cannot go to that sector!")
	}
	if err := u.CheckSystem(p, SysEngines); err != nil {
		return nil, err
	}
	if err := u.spendTurn(p); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	var report []string

	// hostile defense force? fight your way in with everything you have
	hostile := u.hostileBattlegroup(p, dest)
	if hostile != nil && hostile.GetQuantity(FIGHTERS) > 0 {
		defenders := hostile.GetQuantity(FIGHTERS)
		owner := hostile.GetOwnerPlayer()
		report = append(report, fmt.Sprintf("%d fighters belonging to %s contest your entry!", defenders, owner.GetName()))

		pLoss, dLoss := attrition(p.GetQuantity(FIGHTERS), defenders)
		p.AdjustQuantity(FIGHTERS, -pLoss)
		hostile.AdjustQuantity(FIGHTERS, -dLoss)
		report = append(report,
			fmt.Sprintf("You lost %d fighters, %d remain.", pLoss, p.GetQuantity(FIGHTERS)),
			fmt.Sprintf("You destroyed %d defending fighters, %d remain.", dLoss, hostile.GetQuantity(FIGHTERS)))

		if p.GetQuantity(FIGHTERS) <= 0 && hostile.GetQuantity(FIGHTERS) > 0 {
			if u.tryEmWarp(p, now) {
				report = append(report, "Your Emergency Warp fires - you escape the defenders and vanish!")
				u.AddNews(hostile.Owner, now, fmt.Sprintf("Your fighters in sector %d drove off %s (they emergency-warped away).", dest, p.GetName()))
				u.MarkDirty()
				return report, nil
			}
			report = append(report, "Your ship is destroyed! The Traders Guild will reconstruct you tomorrow.")
			u.AddNews(hostile.Owner, now, fmt.Sprintf("Your fighters in sector %d destroyed %s's ship (you lost %d fighters).", dest, p.GetName(), dLoss))
			u.KillPlayer(p, now)
			u.MarkDirty()
			return report, nil
		}
		u.AddNews(hostile.Owner, now, fmt.Sprintf("%s fought through your defenses in sector %d: you lost %d fighters (%d remain).", p.GetName(), dest, dLoss, hostile.GetQuantity(FIGHTERS)))
		if !hostile.HasInventory() {
			u.Battlegroups.RemoveBattlegroup(hostile)
		}
	}

	p.MoveTo(dest)

	// hostile mines detonate on arrival, one by one, until spent or the
	// intruder is destroyed
	hostile = u.hostileBattlegroup(p, dest)
	if hostile != nil && hostile.GetQuantity(MINES) > 0 {
		owner := hostile.GetOwnerPlayer()
		blasted := 0
		escaped := false
		for hostile.GetQuantity(MINES) > 0 {
			hostile.AdjustQuantity(MINES, -1)
			blasted++
			f, h := mineBlast(p)
			report = append(report, fmt.Sprintf("A mine belonging to %s detonates! You lose %d fighters and %d holds.", owner.GetName(), f, h))
			if p.GetQuantity(FIGHTERS) <= 0 {
				if u.tryEmWarp(p, now) {
					report = append(report, "Your Emergency Warp fires, hurling you clear of the minefield!")
					u.AddNews(hostile.Owner, now, fmt.Sprintf("Your minefield in sector %d drove off %s (they emergency-warped away).", dest, p.GetName()))
					escaped = true
					break
				}
				report = append(report, "The blast tears your ship apart! The Traders Guild will reconstruct you tomorrow.")
				u.AddNews(hostile.Owner, now, fmt.Sprintf("Your minefield in sector %d destroyed %s's ship (%d mines expended).", dest, p.GetName(), blasted))
				u.KillPlayer(p, now)
				break
			}
		}
		if !p.IsDead() && !escaped {
			u.AddNews(hostile.Owner, now, fmt.Sprintf("%s hit your minefield in sector %d (%d mines expended).", p.GetName(), dest, blasted))
		}
		if !hostile.HasInventory() {
			u.Battlegroups.RemoveBattlegroup(hostile)
		}
	}

	u.MarkDirty()
	return report, nil
}

// hostileBattlegroup returns a battlegroup in the sector owned by someone
// else, or nil.
func (u *UniverseType) hostileBattlegroup(p *Player, sector int) *Battlegroup {
	for _, bg := range u.Battlegroups.Battlegroups {
		if bg.Sector == sector && bg.Owner != p.Id {
			return bg
		}
	}
	return nil
}

// Dock charges the docking turn and brings the port's stock up to date.
// Docking at Sol is free, as in the original (TWARS.PAS:484). A damaged
// cargo bay makes trading impossible.

func (u *UniverseType) Dock(player *Player, port *Port) error {
	if player.IsDead() {
		return NewGameError(ErrDead, "Your ship has been destroyed.")
	}
	if err := u.CheckSystem(player, SysCargoBay); err != nil {
		return err
	}
	if port.Goods == Sol {
		return nil
	}
	if err := u.spendTurn(player); err != nil {
		return err
	}
	port.Restock(time.Now().Unix())
	u.MarkDirty()
	return nil
}

// SolRepair fixes all ship-system damage for cost_of_repair credits per
// point (the original's Sol menu item 4; RepairShip, SOLPORT.PAS:23).
func (u *UniverseType) SolRepair(p *Player) error {
	total := p.TotalSystemDamage()
	if total == 0 {
		return NewGameError(ErrNotFound, "All ship systems are operational.")
	}
	cost := total * u.ConfigInt("cost_of_repair", 250)
	if p.GetMoney() < cost {
		return NewGameError(ErrNotEnoughMoney, fmt.Sprintf("Full repairs cost %d credits; you don't have that.", cost))
	}
	p.AdjustMoney(-cost)
	p.Systems = make([]int, NumSystems)
	u.MarkDirty()
	return nil
}
