package galwar

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// Death and reconstruction, per kill_player (TWLIB1.PAS:1384-1461) and
// reconstruct_user (LOGON.PAS:121-180): death flags the record and forfeits
// standing assets to the NPC factions; the player is rebuilt at their first
// session of the next day, with an escalating penalty. The first death is
// free.

// EnsureNPC returns the faction record for the given kind ("renegades",
// "cabal", "federation"), creating it on first use - the equivalent of the
// original's reserved user slots 97/98/99. NPCs park in sector 0 (off-map,
// like the original's 9999) so they never appear in a sector or take fire.
func (u *UniverseType) EnsureNPC(kind string) *Player {
	sub := "npc:" + kind
	if p := u.Players.GetBySub(sub); p != nil {
		return p
	}
	names := map[string]string{
		"renegades":  "The Renegades",
		"cabal":      "The Cabal",
		"federation": "The Federation",
	}
	name, ok := names[kind]
	if !ok {
		name = "The " + kind
	}
	p := &Player{
		Id:        PlayerId(uuid.New().String()),
		GoogleSub: sub,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: 0,
		},
		Systems: make([]int, NumSystems),
	}
	u.Players.Players = append(u.Players.Players, p)
	u.MarkDirty()
	return p
}

// forfeitAssets hands a player's standing assets to the NPC factions: sector
// defense forces to the Renegades, planets "revolt" to the Cabal or the
// Federation (kill_player gave them to a teammate first; teams are Phase C).
// Shared by combat death and dormancy expiry.
func (u *UniverseType) forfeitAssets(p *Player, now int64) {
	renegades := u.EnsureNPC("renegades")
	for _, bg := range u.Battlegroups.Battlegroups {
		if bg.Owner == p.Id {
			bg.Owner = renegades.Id
		}
	}

	for _, planet := range u.Planets.Planets {
		if planet.Owner != p.Id {
			continue
		}
		var heir *Player
		if rand.Intn(2) == 0 {
			heir = u.EnsureNPC("cabal")
		} else {
			heir = u.EnsureNPC("federation")
		}
		planet.Owner = heir.Id
		u.AddNews(p.Id, now, fmt.Sprintf("Your planet %s in sector %d has revolted to %s.", planet.Name, planet.Sector, heir.GetName()))
	}
}

// resetToStartingShip wipes a player's inventory and issues a fresh starting
// ship scaled by mult (100 = full kit). Shared by reconstruction and expiry.
func (u *UniverseType) resetToStartingShip(p *Player, mult int) {
	p.Inventory = nil
	p.Money = u.ConfigInt("starting_credits", 35000) * mult / 100
	for _, tg := range TradeGoods {
		quantity := tg.Starting
		switch tg.Name {
		case HOLDS:
			quantity = u.ConfigInt("starting_holds", quantity) * mult / 100
		case FIGHTERS:
			quantity = u.ConfigInt("starting_fighters", quantity) * mult / 100
		case TURNS:
			quantity = u.ConfigInt("turns_per_day", quantity)
		}
		p.Inventory = append(p.Inventory, &Commodity{Name: tg.Name, Quantity: quantity})
	}
	p.Systems = make([]int, NumSystems)
}

// KillPlayer marks a player dead and forfeits their standing assets. The dead
// ship parks in sector 0 until reconstruction.
func (u *UniverseType) KillPlayer(p *Player, now int64) {
	if p.IsDead() {
		return
	}
	p.TimesDied++
	p.DiedAt = now
	u.forfeitAssets(p, now)
	p.MoveTo(0)
	u.MarkDirty()
}

// ExpirePlayer applies Tier-2 dormancy cleanup: forfeit assets and issue a
// fresh starter ship at Sol, no death state, no penalty. Deliberate deviation
// from the original's "erase and reconstruct on return": resetting in place
// is idempotent (guarded by the Expired flag) and spares a returning player
// the day-of-death lockout that never made sense for mere absence.
func (u *UniverseType) ExpirePlayer(p *Player, now int64) {
	if p.IsNPC() || p.IsDead() || p.Expired {
		return
	}
	u.forfeitAssets(p, now)
	u.resetToStartingShip(p, 100)
	p.MoveTo(1)
	p.Expired = true
	u.AddNews(p.Id, now, "You were absent so long the Traders Guild repossessed your holdings and issued you a fresh starter ship.")
	u.MarkDirty()
}

// ReconstructIfDue implements the login-time death check. Returns a message
// for the player and whether they were just rebuilt. If the player is still
// dead afterward (died today), the session should show the message and end.
func (u *UniverseType) ReconstructIfDue(p *Player, now time.Time) (string, bool) {
	if !p.IsDead() {
		return "", false
	}
	diedDay := time.Unix(p.DiedAt, 0).UTC().Format("2006-01-02")
	today := now.UTC().Format("2006-01-02")
	if diedDay == today {
		return "You are DEAD. The Traders Guild is still reassembling your ship - come back tomorrow.", false
	}

	// escalating penalty: first death free, then
	// deathper = round((100 - (1/timesdied)*100) * 0.65)  (LOGON.PAS:139)
	penalty := 0
	if p.TimesDied > 1 {
		penalty = int(math.Round((100 - (1/float64(p.TimesDied))*100) * 0.65))
	}
	mult := 100 - penalty

	u.resetToStartingShip(p, mult)
	p.DiedAt = 0
	p.MoveTo(1)
	u.MarkDirty()

	if penalty == 0 {
		return "The Traders Guild has reconstructed your ship at no charge. Don't let it happen again.", true
	}
	return fmt.Sprintf("THE TRADERS GUILD WILL NOT PERMIT FAILURE! Your ship has been reconstructed with a %d%% penalty (death #%d).", penalty, p.TimesDied), true
}
