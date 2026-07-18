package galwar

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// NPC faction AI. Every nightly maintenance the factions'
// dormant/active state is re-evaluated against how utilized the game is, then
// active factions take their turn. There is no calendar activation: an
// underplayed or newbie-only world keeps the factions asleep, so nobody gets
// obliterated on "day 4".
//
// The action layer is a deliberately simplified realization of the original's
// MAINT2 machinery: we have no starbases and model faction strikes as direct
// combat resolution (like AttackPlayer) rather than moving sector-fighter
// stacks. The dormancy policy and newbie protection - the point of the design
// - are implemented in full.

// Each faction gates in and fights from one dedicated stronghold, identified by
// name so it's never confused with planets the faction has inherited from dead
// players (forfeitAssets hands those to the NPC heirs).
const (
	cabalStrongholdName = "Cabal Stronghold"
	renegadeNestName    = "Renegade Nest"
)

// factionMetrics summarizes the state of the world for the dormancy decision.
func (u *UniverseType) factionMetrics(now time.Time) (activeCount, leaderValue int, quiet bool) {
	var mostRecentLogin int64
	for _, p := range u.Players.Players {
		if p.IsNPC() || p.IsDead() {
			continue
		}
		if p.LastSeen > mostRecentLogin {
			mostRecentLogin = p.LastSeen
		}
		if u.IsDormant(p, now) {
			continue
		}
		activeCount++
		if v := u.PlayerValue(p); v > leaderValue {
			leaderValue = v
		}
	}
	quietDays := int64(u.ConfigInt("faction_quiet_days", 3))
	quiet = mostRecentLogin == 0 || now.Unix()-mostRecentLogin > quietDays*86400
	return activeCount, leaderValue, quiet
}

// topActivePlayer returns the highest-value active (non-NPC, alive,
// non-dormant) player, or nil.
func (u *UniverseType) topActivePlayer(now time.Time) *Player {
	var top *Player
	best := -1
	for _, p := range u.Players.Players {
		if p.IsNPC() || p.IsDead() || u.IsDormant(p, now) {
			continue
		}
		if v := u.PlayerValue(p); v > best {
			best = v
			top = p
		}
	}
	return top
}

func (u *UniverseType) setFactionActive(key string, active bool) {
	if active {
		u.SetConfig(key, "1")
	} else {
		u.SetConfig(key, "0")
	}
}

// updateFactionStates applies the wake/sleep triggers with hysteresis. The
// Federation's active military follows: it wakes iff the Cabal or Renegades
// are active (there is disorder to fight). Its static duties - fed-space
// safety, dead-asset inheritance - are not gated here and always apply.
func (u *UniverseType) updateFactionStates(activeCount, leaderValue int, quiet bool) (cabal, ren bool) {
	cabal = u.ConfigInt("cabal_active", 0) == 1
	ren = u.ConfigInt("ren_active", 0) == 1

	// Cabal: population AND a worthy leader AND a live game; hysteresis band
	if cabal {
		if activeCount < u.ConfigInt("cabal_min_players", 3) ||
			leaderValue < u.ConfigInt("cabal_sleep_value", 268000) || quiet {
			cabal = false
			log.Printf("faction AI: the Cabal goes dormant")
		}
	} else {
		if activeCount >= u.ConfigInt("cabal_min_players", 3) &&
			leaderValue >= u.ConfigInt("cabal_wake_value", 536000) && !quiet {
			cabal = true
			log.Printf("faction AI: the Cabal AWAKENS (leader value %d)", leaderValue)
		}
	}

	// Renegades: population and a live game only (ambient chaos, no leader bar)
	if ren {
		if activeCount < u.ConfigInt("ren_min_players", 2) || quiet {
			ren = false
			log.Printf("faction AI: the Renegades go dormant")
		}
	} else {
		if activeCount >= u.ConfigInt("ren_min_players", 2) && !quiet {
			ren = true
			log.Printf("faction AI: the Renegades stir")
		}
	}

	u.setFactionActive("cabal_active", cabal)
	u.setFactionActive("ren_active", ren)
	return cabal, ren
}

// runFactionAI is the nightly faction pass, called from RunDailyMaintenance.
func (u *UniverseType) runFactionAI(now time.Time) {
	activeCount, leaderValue, quiet := u.factionMetrics(now)
	cabalActive, renActive := u.updateFactionStates(activeCount, leaderValue, quiet)
	unix := now.Unix()
	events := 0

	// a per-stronghold fighter ceiling shared by every gated faction planet
	strongholdCap := u.ConfigInt("cabal_max_planet_fighters", 15000)

	if cabalActive {
		cabal := u.EnsureNPC("cabal")
		targetVal := leaderValue * u.ConfigInt("cabal_scale_pct", 35) / 100
		fortress := u.gateFactionPlanet(cabal, targetVal, cabalStrongholdName, strongholdCap)
		// hunt the leader with half the stronghold's fighters
		floor := u.ConfigInt("faction_target_floor", 200000)
		if leader := u.topActivePlayer(now); fortress != nil && leader != nil && u.PlayerValue(leader) >= floor {
			if u.factionStrike(cabal, fortress, leader, fortress.GetQuantity(FIGHTERS)/2, unix) {
				events++
			}
		}
		// plus a little mayhem against random worthy players (the leader hunted
		// above may be re-rolled here - the Cabal is not tidy about it)
		events += u.factionMayhem(cabal, fortress, now, 2)
	}

	if renActive {
		ren := u.EnsureNPC("renegades")
		fortress := u.gateFactionPlanet(ren, u.ConfigInt("ren_target_value", 134000), renegadeNestName, strongholdCap)
		events += u.factionMayhem(ren, fortress, now, 2)
	}

	// Federation active military: erode the Cabal while it's loose
	if cabalActive || renActive {
		u.fedCounterCabal()
	}

	if cabalActive || renActive {
		log.Printf("faction AI: cabal=%v renegades=%v, %d strikes resolved", cabalActive, renActive, events)
	}
}

// gateFactionPlanet is the GatePlanet equivalent: if the faction's total value
// is below targetValue, add fighters to its named stronghold (creating one if
// needed) to close the gap - but no more than maxFighters, and never beyond the
// gap itself. So a modest world gets a modest faction; the original's flat
// 15,000-fighter floor is replaced by this proportional one.
func (u *UniverseType) gateFactionPlanet(faction *Player, targetValue int, name string, maxFighters int) *Planet {
	fortress := u.factionStronghold(faction, name)
	if u.PlayerValue(faction) >= targetValue {
		return fortress
	}
	deficit := targetValue - u.PlayerValue(faction)
	want := deficit / u.ConfigInt("cost_of_fighter", 98)
	// maxFighters is a ceiling on the stronghold's total fighters, not a
	// per-night increment, so a runaway leader can't pump it up forever.
	existing := 0
	if fortress != nil {
		existing = fortress.GetQuantity(FIGHTERS)
	}
	if existing+want > maxFighters {
		want = maxFighters - existing
	}
	if want < 1 {
		return fortress
	}
	if fortress == nil {
		sec := u.freeSectorForPlanet()
		if sec == 0 {
			return nil
		}
		fortress = u.NewPlanet(faction.Id, sec, name)
		for _, n := range []string{ORE, ORGANICS, EQUIPMENT} {
			if c := fortress.GetCommodity(n); c != nil {
				c.Prod = 1000
			}
		}
	}
	fortress.AdjustQuantity(FIGHTERS, want)
	u.MarkDirty()
	return fortress
}

// factionStronghold returns the faction's dedicated stronghold - the planet it
// owns under the given name - or nil. Matching by name (not just ownership)
// skips planets the faction inherited from dead players, so gating and the Fed
// counter-strike always act on the real HQ.
func (u *UniverseType) factionStronghold(faction *Player, name string) *Planet {
	for _, p := range u.Planets.Planets {
		if p.Owner == faction.Id && p.Name == name {
			return p
		}
	}
	return nil
}

// inFedSpace reports whether a sector is protected from combat: the Federation
// safe zone (sectors 1-10) plus the unplaced sector 0, for players and factions
// alike.
func inFedSpace(sector int) bool {
	return sector <= 10
}

// freeSectorForPlanet finds a non-Federation sector with no planet: a handful of
// random picks first (to spread strongholds around), then a deterministic scan
// so a dense-but-not-full universe never fails spuriously. 0 if none is free.
func (u *UniverseType) freeSectorForPlanet() int {
	numsec := len(u.Sectors) - 1
	if numsec <= 10 {
		return 0
	}
	for tries := 0; tries < 50; tries++ {
		s := 11 + rand.Intn(numsec-10)
		if len(u.GetObjectsInSector(s, TYPE_PLANET)) == 0 {
			return s
		}
	}
	for s := 11; s <= numsec; s++ {
		if len(u.GetObjectsInSector(s, TYPE_PLANET)) == 0 {
			return s
		}
	}
	return 0
}

// factionStrike launches ships from a stronghold at a target player. The fleet
// takes the shortest warp path to the target and fights through the placed
// defenses on the way - hostile sector defense forces and minefields (M21) -
// so leaving fighters and mines in a sector genuinely blunts a faction strike.
// The target's own home garrison/mines are the last hop, so fortifying your
// sector is a real defense. Survivors then resolve combat against the target;
// a fleet wiped en route never reaches them. Survivors return to the
// stronghold. Emergency Warp still saves the target. Returns whether the
// target was killed.
//
// Transit ships are NOT engaged: a mobile ship merely passing through a route
// sector is reconned but not attacked (battleSector engageShips=false), so a
// bystander is never collateral - only the intended target is struck.
func (u *UniverseType) factionStrike(faction *Player, source *Planet, target *Player, ships int, now int64) bool {
	if source == nil || target == nil {
		return false
	}
	// Federation space (sectors 1-10) is always safe - no faction can strike a
	// player sheltering there, mirroring the AttackPlayer rule (combat.go).
	if inFedSpace(target.Sector) {
		return false
	}
	if avail := source.GetQuantity(FIGHTERS); ships > avail {
		ships = avail
	}
	if ships < 1 {
		return false
	}
	route := u.ShortestPathTo(source.Sector, target.Sector)
	if route == nil {
		return false // no warp path: the fleet can't reach them
	}
	source.AdjustQuantity(FIGHTERS, -ships)
	fleet := ships

	// transit: fight through placed garrisons and minefields on the route
	// (including the target's home sector, the last hop). Ships in transit are
	// not attacked.
	for _, sec := range route[1:] {
		u.battleSector(faction, sec, &fleet, now, false)
		if fleet <= 0 {
			break // the approach's defenses destroyed the fleet; target untouched
		}
	}

	killed := false
	sector := target.Sector
	if fleet > 0 {
		aLoss, dLoss := attrition(fleet, target.GetQuantity(FIGHTERS))
		fleet -= aLoss
		target.AdjustQuantity(FIGHTERS, -dLoss)
		if target.GetQuantity(FIGHTERS) <= 0 {
			if u.tryEmWarp(target, now) {
				u.AddNews(target.Id, now, fmt.Sprintf("%s's fleet attacked you in sector %d - your Emergency Warp saved you!", faction.GetName(), sector))
			} else {
				u.AddNews(target.Id, now, fmt.Sprintf("You were destroyed by %s's fleet in sector %d!", faction.GetName(), sector))
				u.KillPlayer(target, now)
				killed = true
			}
		} else {
			u.AddNews(target.Id, now, fmt.Sprintf("%s's fleet raided you in sector %d: you lost %d fighters (%d remain).", faction.GetName(), sector, dLoss, target.GetQuantity(FIGHTERS)))
		}
	}
	source.AdjustQuantity(FIGHTERS, fleet) // survivors return
	u.MarkDirty()
	return killed
}

// factionMayhem strikes up to count random worthy (above-floor, non-dormant)
// players with a force proportional to each target - a fair fight, not a
// massacre. Players below faction_target_floor (newbies and freshly
// reconstructed players) are never chosen. Returns strikes resolved.
func (u *UniverseType) factionMayhem(faction *Player, fortress *Planet, now time.Time, count int) int {
	if fortress == nil {
		return 0
	}
	floor := u.ConfigInt("faction_target_floor", 200000)
	var targets []*Player
	for _, p := range u.Players.Players {
		if p.IsNPC() || p.IsDead() || u.IsDormant(p, now) {
			continue
		}
		if inFedSpace(p.Sector) || u.PlayerValue(p) < floor {
			continue
		}
		targets = append(targets, p)
	}
	struck := 0
	for i := 0; i < count && len(targets) > 0; i++ {
		if fortress.GetQuantity(FIGHTERS) < 1 {
			break
		}
		// pick a target and drop it from the pool so one pass hits distinct
		// players rather than focus-firing the same one
		idx := rand.Intn(len(targets))
		t := targets[idx]
		targets[idx] = targets[len(targets)-1]
		targets = targets[:len(targets)-1]
		force := t.GetQuantity(FIGHTERS) // proportional: roughly a fair fight
		if force < 100 {
			force = 100
		}
		u.factionStrike(faction, fortress, t, force, now.Unix())
		struck++
	}
	return struck
}

// fedCounterCabal is the Federation's war on the Cabal: each active night it
// erodes the Cabal stronghold, keeping it from snowballing.
func (u *UniverseType) fedCounterCabal() {
	cabal := u.EnsureNPC("cabal")
	fortress := u.factionStronghold(cabal, cabalStrongholdName)
	if fortress == nil {
		return
	}
	if cf := fortress.GetQuantity(FIGHTERS); cf > 0 {
		fortress.AdjustQuantity(FIGHTERS, -cf/4) // Feds shave ~25% nightly
		u.MarkDirty()
	}
}
