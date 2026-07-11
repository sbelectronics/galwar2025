package galwar

// Engine command entry points. The engine is the sole authority on game
// rules; front-ends only prompt and format. Each command validates
// everything before mutating anything, and must be invoked through the
// universe actor (Universe.Do / Universe.DoErr) once Start has been called.

// MovePlayer moves a player to an adjacent sector, validating the warp.

func (u *UniverseType) MovePlayer(p *Player, dest int) error {
	if dest < 1 || dest >= len(u.Sectors) {
		return NewGameError(ErrNotFound, "There is no such sector!")
	}
	if !u.Sectors[p.Sector].HasWarp(dest) {
		return NewGameError(ErrNotFound, "You cannot go to that sector!")
	}
	p.MoveTo(dest)
	return nil
}
