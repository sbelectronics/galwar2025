package galwar

import (
	"fmt"
	"math/rand"
	"time"
)

// Special stockpile items (Sol-sold, carry any number): Plasma devices rewire
// warps, Pulsar bombs wreck a planet's production, and Emergency Warp saves
// you from a combat death. Unlike the original's 15-slot equipped devices,
// these are plain commodities you accumulate and consume - the model the
// original used for plasma and pulsar all along, extended to Emergency Warp.

// PlasmaAction selects what UsePlasma does.
type PlasmaAction int

const (
	PlasmaAdd PlasmaAction = iota
	PlasmaRemove
)

// UsePlasma adds or removes a two-way warp between the player's current
// sector and a target, faithful to the plasma device (GWMISC.PAS:91-168) but
// simplified to add/remove (no mid-warp rerouting). Sectors 1-10 are
// protected and a sector can hold at most max_warps links. Costs 1 plasma and
// 1 turn.
func (u *UniverseType) UsePlasma(p *Player, action PlasmaAction, target int) ([]string, error) {
	if p.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if p.GetQuantity(PLASMA) < 1 {
		return nil, NewGameError(ErrNotEnoughQuantity, "You have no plasma devices!")
	}
	src := p.Sector
	if src <= 10 {
		return nil, NewGameError(ErrFedRestricted, "Sectors 1 through 10 cannot be plasma-linked.")
	}
	if target < 1 || target >= len(u.Sectors) {
		return nil, NewGameError(ErrNotFound, "There is no such sector!")
	}
	if target <= 10 {
		return nil, NewGameError(ErrFedRestricted, "Sectors 1 through 10 cannot be plasma-linked.")
	}
	if target == src {
		return nil, NewGameError(ErrUnknown, "You can't link a sector to itself.")
	}

	switch action {
	case PlasmaAdd:
		if u.Sectors[src].HasWarp(target) {
			return nil, NewGameError(ErrAlreadyExists, "There is already a warp to that sector.")
		}
		maxWarps := u.ConfigInt("max_warps", 8)
		if len(u.Sectors[src].Warps) >= maxWarps {
			return nil, NewGameError(ErrNotEnoughQuantity, "This sector already has the maximum number of warps.")
		}
		if len(u.Sectors[target].Warps) >= maxWarps {
			return nil, NewGameError(ErrNotEnoughQuantity, "That sector already has the maximum number of warps.")
		}
		if err := u.spendTurn(p); err != nil {
			return nil, err
		}
		u.Sectors[src].AddWarp(target)
		u.Sectors[target].AddWarp(src)
		p.AdjustQuantity(PLASMA, -1)
		u.MarkDirty()
		return []string{fmt.Sprintf("High plasma activity detected. A warp now links sectors %d and %d.", src, target)}, nil

	case PlasmaRemove:
		if !u.Sectors[src].HasWarp(target) {
			return nil, NewGameError(ErrNotFound, "There is no warp to that sector.")
		}
		if err := u.spendTurn(p); err != nil {
			return nil, err
		}
		u.Sectors[src].RemoveWarp(target)
		u.Sectors[target].RemoveWarp(src)
		p.AdjustQuantity(PLASMA, -1)
		u.MarkDirty()
		return []string{fmt.Sprintf("High plasma activity detected. The warp between sectors %d and %d is severed.", src, target)}, nil
	}
	return nil, NewGameError(ErrUnknown, "Unknown plasma action.")
}

// UsePulsar drops pulsar bombs on the player's own planet in the current
// sector, faithful to PulsarBomb (PLANET.PAS:243-325): each bomb destroys up
// to 1000 of each commodity's production, capping the stockpile at 10x the
// new rate. If all three production rates reach zero the planet is destroyed.
// Costs the bombs and 1 turn. (You must own the planet - invasion arrives in
// a later milestone.)
func (u *UniverseType) UsePulsar(p *Player, bombs int) ([]string, error) {
	if p.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if bombs < 1 {
		return nil, NewGameError(ErrNegativeQuantity, "You must drop at least one bomb.")
	}
	planet, err := u.Planets.GetPlanet(p, p.Sector, MUST_EXIST)
	if err != nil {
		return nil, err // no planet here, or you don't own it
	}
	if p.GetQuantity(PULSAR) < bombs {
		return nil, NewGameError(ErrNotEnoughQuantity, "You don't have that many pulsar bombs.")
	}
	if err := u.spendTurn(p); err != nil {
		return nil, err
	}
	p.AdjustQuantity(PULSAR, -bombs)
	report := u.bombPlanet(planet, 1000*bombs)
	u.MarkDirty()
	return report, nil
}

// UsePulsarTube is the orbital strike: with a Pulsar Tube device you can bomb
// any planet in your sector (owned or not) at 500 production per bomb - the
// original Pulsar Tube's rate (DEVICE.PAS:884). Consumes the bombs and 1 turn;
// notifies an enemy owner. This is the [B] Use Device action.
func (u *UniverseType) UsePulsarTube(p *Player, bombs int) ([]string, error) {
	if p.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if p.GetQuantity(PULSARTUBE) < 1 {
		return nil, NewGameError(ErrNotEnoughQuantity, "You don't have a Pulsar Tube.")
	}
	if bombs < 1 {
		return nil, NewGameError(ErrNegativeQuantity, "You must launch at least one bomb.")
	}
	var planet *Planet
	for _, pl := range u.Planets.Planets {
		if pl.Sector == p.Sector {
			planet = pl
			break
		}
	}
	if planet == nil {
		return nil, NewGameError(ErrNotFound, "There is no planet in this sector.")
	}
	if p.GetQuantity(PULSAR) < bombs {
		return nil, NewGameError(ErrNotEnoughQuantity, "You don't have that many pulsar bombs.")
	}
	if err := u.spendTurn(p); err != nil {
		return nil, err
	}
	owner := planet.Owner
	name := planet.Name
	p.AdjustQuantity(PULSAR, -bombs)
	report := u.bombPlanet(planet, 500*bombs)
	if owner != p.Id {
		u.AddNews(owner, time.Now().Unix(), fmt.Sprintf("Your planet %s in sector %d was pulsar-bombed from orbit by %s!", name, p.Sector, p.GetName()))
	}
	u.MarkDirty()
	return report, nil
}

// bombPlanet applies perBomb production damage to each of the planet's ore,
// organics, and equipment (capping stock at 10x the new rate), destroying the
// planet if all three production rates reach zero. Shared by the owned-planet
// bomb and the orbital Pulsar Tube.
func (u *UniverseType) bombPlanet(planet *Planet, perBomb int) []string {
	report := []string{fmt.Sprintf("Pulsar bombardment of %s:", planet.Name)}
	for _, name := range []string{ORE, ORGANICS, EQUIPMENT} {
		c := planet.GetCommodity(name)
		if c == nil {
			continue
		}
		d := c.Prod
		if d > perBomb {
			d = perBomb
		}
		c.Prod -= d
		if c.Quantity > c.Prod*10 {
			c.Quantity = c.Prod * 10
		}
		report = append(report, fmt.Sprintf("  %s production destroyed: %d", name, d))
	}
	dead := true
	for _, name := range []string{ORE, ORGANICS, EQUIPMENT} {
		if c := planet.GetCommodity(name); c != nil && c.Prod > 0 {
			dead = false
		}
	}
	if dead {
		report = append(report, fmt.Sprintf("%s's structure collapses - the planet is destroyed!", planet.Name))
		u.Planets.RemovePlanet(planet)
	}
	return report
}

// IsCloaked reports whether a player is hidden by a cloaking device. Binary:
// one or many cloaks give the same effect.
func (p *Player) IsCloaked() bool {
	return p.GetQuantity(CLOAK) >= 1
}

// HasAntiCloak reports whether a player can see through cloaks. Binary.
func (p *Player) HasAntiCloak() bool {
	return p.GetQuantity(ANTICLOAK) >= 1
}

// tryEmWarp fires an Emergency Warp if the player carries one: consume it,
// fling them to a random sector, and return true (they cheated death).
// Faithful to CheckEmWarp (TWLIB1.PAS:1604-1621).
func (u *UniverseType) tryEmWarp(p *Player, now int64) bool {
	if p.GetQuantity(EMWARP) < 1 {
		return false
	}
	numsec := len(u.Sectors) - 1
	if numsec < 1 {
		return false
	}
	p.AdjustQuantity(EMWARP, -1)
	p.MoveTo(1 + rand.Intn(numsec))
	u.AddNews(p.Id, now, "Your Emergency Warp device energized and flung you clear of destruction!")
	return true
}
