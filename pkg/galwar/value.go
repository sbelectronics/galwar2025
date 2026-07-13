package galwar

import (
	"sort"
	"time"
)

// PlayerValue estimates a player's net worth, the basis for rankings - a
// pared-down playerval (TWLIB1.PAS:1143-1226) covering what exists so far:
// credits, ship contents at their Sol/market prices, and owned planets and
// battlegroups. Devices, starbases, and the vault join in Phase C.
func (u *UniverseType) PlayerValue(p *Player) int {
	costFighter := u.ConfigInt("cost_of_fighter", 98)
	costHold := u.ConfigInt("cost_of_hold", 500)
	costMine := u.ConfigInt("cost_of_mine", 15000)
	costGenesis := u.ConfigInt("cost_of_genesis", 10000)

	value := p.Money
	value += p.GetQuantity(FIGHTERS) * costFighter
	value += p.GetQuantity(HOLDS) * costHold
	value += p.GetQuantity(MINES) * costMine
	value += p.GetQuantity(GENESIS) * costGenesis
	value += p.GetQuantity(PLASMA) * u.ConfigInt("cost_of_plasma", 56000)
	value += p.GetQuantity(PULSAR) * u.ConfigInt("cost_of_pulsar", 215000)
	value += p.GetQuantity(EMWARP) * u.ConfigInt("cost_of_escape", 27000)
	value += p.GetQuantity(CLOAK) * u.ConfigInt("cost_of_cloak", 18000)
	value += p.GetQuantity(ANTICLOAK) * u.ConfigInt("cost_of_anticloak", 22000)
	value += p.GetQuantity(PULSARTUBE) * u.ConfigInt("cost_of_pulsartube", 350000)
	value += cargoValue(p)

	for _, bg := range u.Battlegroups.Battlegroups {
		if bg.Owner != p.Id {
			continue
		}
		value += bg.GetQuantity(FIGHTERS) * costFighter
		value += bg.GetQuantity(MINES) * costMine
	}
	for _, planet := range u.Planets.Planets {
		if planet.Owner != p.Id {
			continue
		}
		value += planet.GetQuantity(FIGHTERS) * costFighter
		value += planet.GetQuantity(MINES) * costMine
		value += cargoValue(planet)
		value += 50000 // a base worth for holding the planet at all
	}
	return value
}

// cargoValue totals the ore/organics/equipment in an inventory at their base
// sell prices.
func cargoValue(inv InventoryInterface) int {
	total := 0
	for _, name := range []string{ORE, ORGANICS, EQUIPMENT} {
		if def := FindCommodityDef(name); def != nil {
			total += inv.GetQuantity(name) * int(def.SellPrice)
		}
	}
	return total
}

// Ranking is one row of the leaderboard.
type Ranking struct {
	Name    string
	Value   int
	Dormant bool
}

// RankedPlayers returns real (non-NPC, alive) players sorted by value,
// highest first. Dormant players are included but flagged.
func (u *UniverseType) RankedPlayers(now time.Time) []Ranking {
	var out []Ranking
	for _, p := range u.Players.Players {
		if p.IsNPC() || p.IsDead() {
			continue
		}
		out = append(out, Ranking{
			Name:    p.GetName(),
			Value:   u.PlayerValue(p),
			Dormant: u.IsDormant(p, now),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Value > out[j].Value
	})
	return out
}
