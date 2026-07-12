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
// opposite sides. Note the faithful asymmetry: random(100)>50 is a 49/51
// split favoring the defender.
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

// detonateCarriedMines is checkhitmines: every sector mine the victim was
// carrying goes off against the killer. A big enough stockpile takes the
// killer down with the ship.
func (u *UniverseType) detonateCarriedMines(killer *Player, victim *Player, now int64, report []string) []string {
	mines := victim.GetQuantity(MINES)
	for m := 0; m < mines; m++ {
		f, h := mineBlast(killer)
		report = append(report, fmt.Sprintf("One of %s's cargo mines detonates! You lose %d fighters and %d holds.", victim.GetName(), f, h))
		if killer.GetQuantity(FIGHTERS) <= 0 {
			report = append(report, "The blast tears your ship apart! The Traders Guild will reconstruct you tomorrow.")
			u.KillPlayer(killer, now)
			break
		}
	}
	return report
}
