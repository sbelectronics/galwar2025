package galwar

import "time"

// Dormancy and expiry (PLAN.md section 6): players who stop logging in fade
// out in two tiers, so the universe isn't cluttered with abandoned ships and
// so active players can't farm the long-absent as loot pinatas.
//
//   Tier 1 (dormant, default 5 days absent): the ship vanishes from sector
//   displays and scans and can't be attacked. Battlegroups and planets keep
//   working, but planet production growth freezes (see maintenance).
//
//   Tier 2 (expired, default 30 days absent): assets are forfeited to the NPC
//   factions exactly as on death and the ship is reset to a starter, but the
//   account and its stats survive. Returning simply finds the fresh ship.
//
// Any login clears both tiers instantly: TouchLastSeen runs at session start,
// before the sector display, so a returning player rematerializes as they
// arrive.

func (p *Player) daysAbsent(now time.Time) float64 {
	if p.LastSeen == 0 {
		return 0
	}
	return now.Sub(time.Unix(p.LastSeen, 0)).Hours() / 24
}

// IsDormant reports whether a player's ship should be hidden (Tier 1). NPCs
// and dead players are never "dormant" - they're already off the map.
func (u *UniverseType) IsDormant(p *Player, now time.Time) bool {
	if p.IsNPC() || p.IsDead() {
		return false
	}
	return p.daysAbsent(now) >= float64(u.ConfigInt("dormant_days", 5))
}

// IsExpired reports whether a player is due for Tier-2 forfeiture. The Expired
// flag makes the sweep idempotent: once cleaned up, an absent player isn't
// re-expired every night.
func (u *UniverseType) IsExpired(p *Player, now time.Time) bool {
	if p.IsNPC() || p.IsDead() || p.Expired {
		return false
	}
	return p.daysAbsent(now) >= float64(u.ConfigInt("expire_days", 30))
}

// GetVisibleObjectsInSector is GetObjectsInSector with Tier-1 dormant ships
// filtered out. Front-ends use this for sector display and scans; game logic
// that needs the true contents uses GetObjectsInSector (combat targeting
// refuses dormant players separately, in AttackPlayer).
func (u *UniverseType) GetVisibleObjectsInSector(sector int, kind string, now time.Time) []ObjectInterface {
	objs := u.GetObjectsInSector(sector, kind)
	out := objs[:0]
	for _, obj := range objs {
		if p, ok := obj.(*Player); ok && u.IsDormant(p, now) {
			continue
		}
		out = append(out, obj)
	}
	return out
}
