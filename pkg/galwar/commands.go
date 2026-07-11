package galwar

import (
	"time"
)

// Engine command entry points. The engine is the sole authority on game
// rules; front-ends only prompt and format. Each command validates
// everything before mutating anything, and must be invoked through the
// universe actor (Universe.Do / Universe.DoErr) once Start has been called.

// spendTurn charges one turn, or refuses if the player has none, matching
// the original's passturn gate (TWLIB1.PAS:1644).
func (u *UniverseType) spendTurn(p *Player) error {
	if p.GetQuantity(TURNS) < 1 {
		return NewGameError(ErrNoTurns, "You don't have any turns left! Come back tomorrow.")
	}
	p.AdjustQuantity(TURNS, -1)
	return nil
}

// MovePlayer moves a player to an adjacent sector, validating the warp.
// Movement costs 1 turn, as in the original.

func (u *UniverseType) MovePlayer(p *Player, dest int) error {
	if p.Sector < 1 || p.Sector >= len(u.Sectors) {
		// validate() guards the load path; this guards programmatic callers
		return NewGameError(ErrNotFound, "You are in an invalid sector!")
	}
	if dest < 1 || dest >= len(u.Sectors) {
		return NewGameError(ErrNotFound, "There is no such sector!")
	}
	if !u.Sectors[p.Sector].HasWarp(dest) {
		return NewGameError(ErrNotFound, "You cannot go to that sector!")
	}
	if err := u.spendTurn(p); err != nil {
		return err
	}
	p.MoveTo(dest)
	u.MarkDirty()
	return nil
}

// Dock charges the docking turn and brings the port's stock up to date.
// Docking at Sol is free, as in the original (TWARS.PAS:484).

func (u *UniverseType) Dock(player *Player, port *Port) error {
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
