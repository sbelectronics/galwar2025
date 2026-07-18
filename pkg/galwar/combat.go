package galwar

import (
	"fmt"
	"math/rand"
	"time"
)

// PvP combat, faithful to the original's single-move resolution
// (GWMISC.PAS:269-382): the attacker's one command reads the defender's
// stored record, runs the whole battle - exchange, counter-attack, kill,
// salvage, boobytraps - and writes both records back. The defender does not
// need to be online; they read about it in their news.

// attrition is the original's coin-flip exchange loop, reproduced verbatim
// from GWMISC.PAS:322-334 (do not "simplify" - the two decrements below are
// both in the original and are load-bearing for fidelity):
//
//	repeat
//	 if (n-a>1000) and (fighters-c>1000) then if random(100)>50 then a:=a+100 else c:=c+100;
//	 if random(100)>50 then a:=a+1 else c:=c+1;
//	until (a>=n) or (c>=fighters);
//
// Each tick a 50/50-ish roll costs one side a fighter; while BOTH pools still
// exceed 1000 a second roll additionally moves a block of 100 - so a
// large-fleet iteration removes 101 fighters (100 + 1), the coarse step that
// keeps million-fighter battles from looping forever. The two rolls are
// independent, so the block and the single may land on the same side or
// opposite sides. Note the faithful asymmetry: rand.Intn(100)>50 is true for
// 49 of 100 values, so the attacker takes the loss 49% of the time and the
// defender 51% - the roll slightly favors the ATTACKER (the defender's
// fighters deplete faster).
func attrition(attackers int, defenders int) (attackerLoss int, defenderLoss int) {
	a, c := 0, 0
	for a < attackers && c < defenders {
		if attackers-a > 1000 && defenders-c > 1000 {
			if rand.Intn(100) > 50 {
				a += 100
			} else {
				c += 100
			}
		}
		if rand.Intn(100) > 50 {
			a++
		} else {
			c++
		}
	}
	if a > attackers {
		a = attackers
	}
	if c > defenders {
		c = defenders
	}
	return a, c
}

// mineBlast applies one sector-mine detonation to a player (mine_hit,
// TWLIB1.PAS:619): random(200)+300 fighters and random(6)+7 holds destroyed.
// The random-system damage is a stand-in for the original's stun/krypton
// system damage until devices arrive in Phase C.
func mineBlast(p *Player) (fighters int, holds int) {
	fighters = rand.Intn(200) + 300
	holds = rand.Intn(6) + 7

	if have := p.GetQuantity(FIGHTERS); fighters > have {
		fighters = have
	}
	if have := p.GetQuantity(HOLDS); holds > have {
		holds = have
	}
	p.AdjustQuantity(FIGHTERS, -fighters)
	p.AdjustQuantity(HOLDS, -holds)
	p.DamageSystem(rand.Intn(NumSystems), rand.Intn(15)+5)
	return fighters, holds
}

// AttackPlayer resolves a full player-vs-player attack in one command.
// Returns the attacker's battle report. Deviation from the original, per
// PLAN.md 5.3: an attack costs 1 turn (the original charged none, which is
// safe when defenders are only exposed during their own calls but invites
// grinding in an always-on world).
func (u *UniverseType) AttackPlayer(attacker *Player, targetId PlayerId, commit int) ([]string, error) {
	if attacker.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if attacker.Sector <= 10 {
		return nil, NewGameError(ErrFedRestricted, "Federation law prohibits combat in sectors 1 through 10.")
	}
	target := u.Players.GetById(targetId)
	if target == nil || target.IsDead() || target.Sector != attacker.Sector {
		return nil, NewGameError(ErrNotFound, "They aren't here anymore!")
	}
	// a dormant player's ship is hidden and can't be farmed while its owner
	// is away (Tier-1 dormancy)
	if u.IsDormant(target, time.Now()) {
		return nil, NewGameError(ErrNotFound, "They aren't here anymore!")
	}
	// a cloaked target can't be seen (hence attacked) without an anti-cloak
	if target.IsCloaked() && !attacker.HasAntiCloak() {
		return nil, NewGameError(ErrNotFound, "There's no one here to attack.")
	}
	if target == attacker {
		return nil, NewGameError(ErrUnknown, "Attacking yourself is not a strategy.")
	}
	if commit < 1 {
		return nil, NewGameError(ErrNegativeQuantity, "You must commit at least one fighter.")
	}
	if commit > attacker.GetQuantity(FIGHTERS) {
		return nil, NewGameError(ErrNotEnoughQuantity, "You don't have that many fighters.")
	}
	if err := u.spendTurn(attacker); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	sector := attacker.Sector
	var report []string

	// the exchange: committed fighters vs the defender's full complement
	aLoss, dLoss := attrition(commit, target.GetQuantity(FIGHTERS))
	attacker.AdjustQuantity(FIGHTERS, -aLoss)
	target.AdjustQuantity(FIGHTERS, -dLoss)
	report = append(report,
		fmt.Sprintf("You lost %d fighters, %d remain.", aLoss, attacker.GetQuantity(FIGHTERS)),
		fmt.Sprintf("You destroyed %d enemy fighters, %d remain.", dLoss, target.GetQuantity(FIGHTERS)))

	if target.GetQuantity(FIGHTERS) <= 0 {
		report = u.resolveKill(attacker, target, now, report)
		u.MarkDirty()
		return report, nil
	}

	// the defender survived: 60% chance of an automatic counter-attack with
	// the attacker's full remaining fighters at stake (random(10)>3,
	// GWMISC.PAS:344-370)
	if rand.Intn(10) > 3 {
		report = append(report, fmt.Sprintf("%s counter-attacks!", target.GetName()))
		aLoss2, dLoss2 := attrition(attacker.GetQuantity(FIGHTERS), target.GetQuantity(FIGHTERS))
		attacker.AdjustQuantity(FIGHTERS, -aLoss2)
		target.AdjustQuantity(FIGHTERS, -dLoss2)
		report = append(report,
			fmt.Sprintf("You lost %d fighters, %d remain.", aLoss2, attacker.GetQuantity(FIGHTERS)),
			fmt.Sprintf("You destroyed %d enemy fighters, %d remain.", dLoss2, target.GetQuantity(FIGHTERS)))

		switch {
		case attacker.GetQuantity(FIGHTERS) <= 0:
			// the tables turn: the defender's counter kills the attacker
			u.AddNews(target.Id, now, fmt.Sprintf("%s attacked you in sector %d and DIED in your counter-attack! You lost %d fighters.", attacker.GetName(), sector, dLoss+dLoss2))
			report = u.resolveKillReversed(target, attacker, now, report)
			u.MarkDirty()
			return report, nil
		case target.GetQuantity(FIGHTERS) <= 0:
			report = u.resolveKill(attacker, target, now, report)
			u.MarkDirty()
			return report, nil
		}
	}

	u.AddNews(target.Id, now, fmt.Sprintf("You were attacked by %s in sector %d: you lost %d fighters (%d remain); the attacker lost %d.", attacker.GetName(), sector, dLoss, target.GetQuantity(FIGHTERS), aLoss))
	u.MarkDirty()
	return report, nil
}

// resolveKill handles a defender's death at the attacker's hands: hold
// salvage (KillHim, GWMISC.PAS:244-267), the victim's carried mines
// detonating against the killer (checkhitmines, GWMISC.PAS:170-242) - which
// can kill the killer too - then the death itself.
func (u *UniverseType) resolveKill(killer *Player, victim *Player, now int64, report []string) []string {
	if u.tryEmWarp(victim, now) {
		return append(report, fmt.Sprintf("%s's Emergency Warp fires - they vanish before you can finish them off!", victim.GetName()))
	}
	report = append(report, fmt.Sprintf("%s's ship is destroyed!", victim.GetName()))

	// salvage 30-99% of the victim's holds
	pct := rand.Intn(70) + 30
	salvage := victim.GetQuantity(HOLDS) * pct / 100
	maxHolds := u.ConfigInt("max_holds", 16384)
	if room := maxHolds - killer.GetQuantity(HOLDS); salvage > room {
		salvage = room
	}
	if salvage > 0 {
		killer.AdjustQuantity(HOLDS, salvage)
		report = append(report, fmt.Sprintf("You salvage %d cargo holds from the wreckage.", salvage))
	}

	report = u.detonateCarriedMines(killer, victim, now, report)

	u.AddNews(victim.Id, now, fmt.Sprintf("You were KILLED by %s in sector %d! The Traders Guild will reconstruct you tomorrow.", killer.GetName(), victim.Sector))
	u.KillPlayer(victim, now)
	return report
}

// resolveKillReversed is the counter-attack death: the original attacker
// dies, and the defender (who may be offline) collects the salvage - noted
// in their news rather than a report.
func (u *UniverseType) resolveKillReversed(killer *Player, victim *Player, now int64, report []string) []string {
	if u.tryEmWarp(victim, now) {
		return append(report, "Your Emergency Warp fires and flings you clear - you escape with your life!")
	}
	report = append(report, "Your ship is destroyed! The Traders Guild will reconstruct you tomorrow.")

	pct := rand.Intn(70) + 30
	salvage := victim.GetQuantity(HOLDS) * pct / 100
	maxHolds := u.ConfigInt("max_holds", 16384)
	if room := maxHolds - killer.GetQuantity(HOLDS); salvage > room {
		salvage = room
	}
	if salvage > 0 {
		killer.AdjustQuantity(HOLDS, salvage)
		u.AddNews(killer.Id, now, fmt.Sprintf("You salvaged %d cargo holds from %s's wreckage.", salvage, victim.GetName()))
	}

	u.KillPlayer(victim, now)
	return report
}

// InvadePlanet attacks a hostile planet in the attacker's sector, faithful to
// claim_planet + land (PLANET.PAS:417-1043): the committed fighters fight the
// planet's garrison (attrition), then the planet's mines detonate against the
// attacker; break the garrison and survive the minefield and you capture the
// planet - and a bloody assault damages its economy (damageplanet). The owner
// need not be online; they learn of it from their news. Costs 1 turn.
//
// Phaser and stun-mine phases from the original are placeholders until devices
// land (Phase C), matching the mine-blast system-damage stand-in.
func (u *UniverseType) InvadePlanet(attacker *Player, commit int) ([]string, error) {
	if attacker.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if err := u.CheckSystem(attacker, SysThrusters); err != nil {
		return nil, err
	}
	var planet *Planet
	for _, pl := range u.Planets.Planets {
		if pl.Sector == attacker.Sector {
			planet = pl
			break
		}
	}
	if planet == nil {
		return nil, NewGameError(ErrNotFound, "There is no planet in this sector.")
	}
	if planet.Owner == attacker.Id {
		return nil, NewGameError(ErrAlreadyExists, "You already own this planet - land on it instead.")
	}
	if commit < 1 {
		return nil, NewGameError(ErrNegativeQuantity, "You must commit at least one fighter.")
	}
	if commit > attacker.GetQuantity(FIGHTERS) {
		return nil, NewGameError(ErrNotEnoughQuantity, "You don't have that many fighters.")
	}
	if err := u.spendTurn(attacker); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	sector := attacker.Sector
	oldOwner := planet.Owner
	startFighters := attacker.GetQuantity(FIGHTERS)
	var report []string

	// fighter attrition against the garrison
	if garrison := planet.GetQuantity(FIGHTERS); garrison > 0 {
		aLoss, dLoss := attrition(commit, garrison)
		attacker.AdjustQuantity(FIGHTERS, -aLoss)
		planet.AdjustQuantity(FIGHTERS, -dLoss)
		report = append(report,
			fmt.Sprintf("You lost %d fighters, %d remain.", aLoss, attacker.GetQuantity(FIGHTERS)),
			fmt.Sprintf("You destroyed %d of the planet's fighters, %d remain.", dLoss, planet.GetQuantity(FIGHTERS)))

		if planet.GetQuantity(FIGHTERS) > 0 {
			// the garrison held
			if attacker.GetQuantity(FIGHTERS) <= 0 {
				if u.tryEmWarp(attacker, now) {
					report = append(report, "Your Emergency Warp fires - you flee the planet's defenses!")
				} else {
					report = append(report, "Your ship is destroyed by the planetary defenses! The Traders Guild will reconstruct you tomorrow.")
					u.AddNews(oldOwner, now, fmt.Sprintf("Your planet %s in sector %d repelled and destroyed the invader %s.", planet.Name, sector, attacker.GetName()))
					u.KillPlayer(attacker, now)
				}
			} else {
				report = append(report, "The planet's defenders hold - you failed to break through.")
				u.AddNews(oldOwner, now, fmt.Sprintf("Your planet %s in sector %d repelled an invasion by %s.", planet.Name, sector, attacker.GetName()))
			}
			u.MarkDirty()
			return report, nil
		}
	}

	// garrison broken (or absent): the planet's mines detonate against you
	if mines := planet.GetQuantity(MINES); mines > 0 {
		for i := 0; i < mines; i++ {
			planet.AdjustQuantity(MINES, -1)
			if u.absorbMine(attacker) {
				report = append(report, "A planetary mine detonates - your Mine Deflector absorbs the blast!")
				continue
			}
			f, h := mineBlast(attacker)
			report = append(report, fmt.Sprintf("A planetary mine detonates! You lose %d fighters and %d holds.", f, h))
			if attacker.GetQuantity(FIGHTERS) <= 0 {
				if u.tryEmWarp(attacker, now) {
					report = append(report, "Your Emergency Warp fires, hurling you clear of the minefield!")
				} else {
					report = append(report, "The blast tears your ship apart! The Traders Guild will reconstruct you tomorrow.")
					u.AddNews(oldOwner, now, fmt.Sprintf("Your planet %s's minefield in sector %d destroyed the invader %s.", planet.Name, sector, attacker.GetName()))
					u.KillPlayer(attacker, now)
				}
				u.MarkDirty()
				return report, nil
			}
		}
	}

	// capture
	planet.Owner = attacker.Id
	report = append(report, fmt.Sprintf("You have captured %s!", planet.Name))
	u.AddNews(oldOwner, now, fmt.Sprintf("Your planet %s in sector %d has been captured by %s!", planet.Name, sector, attacker.GetName()))

	// damageplanet: a hard-fought assault (fighters spent / 20 >= 100) wrecks
	// the planet's economy, and can destroy it outright (PLANET.PAS:327-363)
	if dam := (startFighters - attacker.GetQuantity(FIGHTERS)) / 20; dam >= 100 {
		for _, name := range []string{ORE, ORGANICS, EQUIPMENT} {
			c := planet.GetCommodity(name)
			if c == nil {
				continue
			}
			d := c.Prod
			if d > dam {
				d = dam
			}
			c.Prod -= d
			if c.Quantity > c.Prod*10 {
				c.Quantity = c.Prod * 10
			}
		}
		dead := true
		for _, name := range []string{ORE, ORGANICS, EQUIPMENT} {
			if c := planet.GetCommodity(name); c != nil && c.Prod > 0 {
				dead = false
			}
		}
		if dead {
			report = append(report, fmt.Sprintf("The assault was so fierce that %s's structure collapses - the planet is destroyed!", planet.Name))
			u.Planets.RemovePlanet(planet)
		} else {
			report = append(report, "The bombardment gutted the planet's production infrastructure.")
		}
	}

	u.MarkDirty()
	return report, nil
}

// detonateCarriedMines is checkhitmines: every sector mine the victim was
// carrying goes off against the killer. A big enough stockpile takes the
// killer down with the ship.
func (u *UniverseType) detonateCarriedMines(killer *Player, victim *Player, now int64, report []string) []string {
	mines := victim.GetQuantity(MINES)
	for m := 0; m < mines; m++ {
		if u.absorbMine(killer) {
			report = append(report, fmt.Sprintf("One of %s's cargo mines detonates - your Mine Deflector absorbs the blast!", victim.GetName()))
			continue
		}
		f, h := mineBlast(killer)
		report = append(report, fmt.Sprintf("One of %s's cargo mines detonates! You lose %d fighters and %d holds.", victim.GetName(), f, h))
		if killer.GetQuantity(FIGHTERS) <= 0 {
			if u.tryEmWarp(killer, now) {
				report = append(report, "Your Emergency Warp fires, hurling you clear of the blast!")
				break
			}
			report = append(report, "The blast tears your ship apart! The Traders Guild will reconstruct you tomorrow.")
			u.KillPlayer(killer, now)
			break
		}
	}
	return report
}

// LaunchBattleGroup sends a strike fleet of `ships` fighters from the sender's
// sector to a target sector, fighting through every sector on the shortest
// path - hostile garrisons, minefields, and enemy players (whom it can kill
// outright, remotely) - faithful to battlegroup + battle_sector
// (TWARS.PAS:913-1202). Surviving ships return to the sender. A single-ship
// group is a scout: it recons and takes fire from sector defenses but won't
// pick fights with enemy ships. Requires an intact Battle-Group Computer and
// costs 1 turn.
//
// This is the G command's mobile fleet - distinct from the stationary
// Battlegroup garrison type (the D/F commands), which is what it fights.
func (u *UniverseType) LaunchBattleGroup(sender *Player, target int, ships int) ([]string, error) {
	if sender.IsDead() {
		return nil, NewGameError(ErrDead, "You are dead.")
	}
	if err := u.CheckSystem(sender, SysBGComputer); err != nil {
		return nil, err
	}
	if target < 1 || target >= len(u.Sectors) {
		return nil, NewGameError(ErrNotFound, "There is no such sector!")
	}
	if target == sender.Sector {
		return nil, NewGameError(ErrUnknown, "You are in that sector!")
	}
	if ships < 1 {
		return nil, NewGameError(ErrNegativeQuantity, "You must send at least one ship.")
	}
	if ships > sender.GetQuantity(FIGHTERS) {
		return nil, NewGameError(ErrNotEnoughQuantity, "You don't have that many fighters.")
	}
	route := u.ShortestPathTo(sender.Sector, target)
	if route == nil {
		return nil, NewGameError(ErrNotFound, "There is no route to that sector!")
	}
	if err := u.spendTurn(sender); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	sender.AdjustQuantity(FIGHTERS, -ships) // the fleet launches
	fleet := ships
	report := []string{fmt.Sprintf("Battle group of %d ships away, bound for sector %d!", ships, target)}

	for _, sec := range route[1:] {
		report = append(report, u.battleSector(sender, sec, &fleet, now, true)...)
		if fleet <= 0 {
			report = append(report, "Your battle group has been destroyed!")
			break
		}
	}
	if fleet > 0 {
		sender.AdjustQuantity(FIGHTERS, fleet) // survivors come home
		report = append(report, fmt.Sprintf("Your battle group returns with %d ships.", fleet))
	}
	u.MarkDirty()
	return report, nil
}

// battleSector resolves one sector of a battle group's route: recon of ports
// and planets, then combat against a hostile garrison's fighters and mines,
// then (when engageShips) enemy players. fleet is reduced in place; the
// returned lines are nil for an empty transit sector (so long routes don't
// spam). Mines ignore a fleet under 150, and only a battle-mode (>1 ship)
// group outside Federation space attacks enemy ships.
//
// engageShips is false for faction strikes (factions.go), which fight through
// placed sector defenses and minefields but never attack a mobile ship merely
// in transit - only the strike's intended target is hit, at the destination.
func (u *UniverseType) battleSector(sender *Player, sec int, fleet *int, now int64, engageShips bool) []string {
	var report []string
	note := func(s string) { report = append(report, s) }

	for _, obj := range u.GetObjectsInSector(sec, TYPE_PORT) {
		note(fmt.Sprintf("Sector %d - Port: %s", sec, obj.GetName()))
	}
	for _, obj := range u.GetObjectsInSector(sec, TYPE_PLANET) {
		note(fmt.Sprintf("Sector %d - Planet: %s", sec, obj.GetName()))
	}

	// hostile garrison: fighters first, then mines
	if garrison := u.hostileBattlegroup(sender, sec); garrison != nil {
		if gf := garrison.GetQuantity(FIGHTERS); gf > 0 {
			owner := garrison.GetOwnerPlayer()
			note(fmt.Sprintf("Sector %d - %d fighters belonging to %s", sec, gf, owner.GetName()))
			aLoss, dLoss := attrition(*fleet, gf)
			*fleet -= aLoss
			garrison.AdjustQuantity(FIGHTERS, -dLoss)
			note(fmt.Sprintf("  Your battle group lost %d ships and destroyed %d fighters.", aLoss, dLoss))
			u.AddNews(garrison.Owner, now, fmt.Sprintf("%s's battle group hit your defense force in sector %d: you lost %d fighters, they lost %d ships.", sender.GetName(), sec, dLoss, aLoss))
		}
		if gm := garrison.GetQuantity(MINES); gm > 0 && *fleet > 0 {
			note(fmt.Sprintf("Sector %d - %d mines belonging to %s", sec, gm, garrison.GetOwnerPlayer().GetName()))
			if *fleet < 150 {
				note("  They did not go off - the fleet was too small to trigger them.")
			} else {
				lost, blown := 0, 0
				for garrison.GetQuantity(MINES) > 0 && *fleet > 0 {
					a := rand.Intn(200) + 300
					if a > *fleet {
						a = *fleet
					}
					*fleet -= a
					lost += a
					blown++
					garrison.AdjustQuantity(MINES, -1)
				}
				note(fmt.Sprintf("  Mines hit! %d ships destroyed clearing %d mines.", lost, blown))
				u.AddNews(garrison.Owner, now, fmt.Sprintf("%s's battle group hit your minefield in sector %d: they lost %d ships, %d mines expended.", sender.GetName(), sec, lost, blown))
			}
		}
		if !garrison.HasInventory() {
			u.Battlegroups.RemoveBattlegroup(garrison)
		}
		if *fleet <= 0 {
			return report
		}
	}

	// enemy players: recon always, attack only in battle mode outside fed space
	// (and only when ship engagement is enabled - faction strikes recon but
	// don't attack ships merely in transit)
	battleMode := engageShips && *fleet > 1 && sec > 10
	for _, obj := range u.GetObjectsInSector(sec, TYPE_PLAYER) {
		p, ok := obj.(*Player)
		if !ok || p == sender || p.IsDead() {
			continue
		}
		if !p.EverMoved {
			continue // never-moved ships are hidden, from recon too
		}
		if u.IsDormant(p, time.Unix(now, 0)) {
			continue
		}
		if p.IsCloaked() && !sender.HasAntiCloak() {
			continue
		}
		note(fmt.Sprintf("Sector %d - %s with %d fighters!", sec, p.GetName(), p.GetQuantity(FIGHTERS)))
		if !battleMode {
			continue
		}
		bLoss, cLoss := attrition(*fleet, p.GetQuantity(FIGHTERS))
		*fleet -= bLoss
		p.AdjustQuantity(FIGHTERS, -cLoss)
		note(fmt.Sprintf("  Your battle group lost %d ships and destroyed %d enemy fighters.", bLoss, cLoss))
		if p.GetQuantity(FIGHTERS) <= 0 {
			if u.tryEmWarp(p, now) {
				u.AddNews(p.Id, now, fmt.Sprintf("%s's battle group attacked you in sector %d - your Emergency Warp saved you!", sender.GetName(), sec))
			} else {
				note(fmt.Sprintf("  You destroyed %s's ship!", p.GetName()))
				u.AddNews(p.Id, now, fmt.Sprintf("You were killed by %s's battle group in sector %d!", sender.GetName(), sec))
				u.KillPlayer(p, now)
			}
		} else {
			u.AddNews(p.Id, now, fmt.Sprintf("%s's battle group attacked you in sector %d: you lost %d fighters (%d remain).", sender.GetName(), sec, cLoss, p.GetQuantity(FIGHTERS)))
		}
		if *fleet <= 0 {
			return report
		}
	}

	return report
}
