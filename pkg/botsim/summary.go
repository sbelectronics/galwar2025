package botsim

import (
	"github.com/sbelectronics/galwar/pkg/galwar"
)

// scoreboardCap bounds the leaderboard captured each day.
const scoreboardCap = 15

// emitDaySummary records a bot's end-of-day standing, captured before the
// night's maintenance refills turns.
func (s *sim) emitDaySummary(b *bot, day int) {
	var val, money, bank, holds, fighters, mines, turns, planets int
	s.u.Do(func() {
		val = s.u.PlayerValue(b.player)
		money = b.player.GetMoney()
		bank = b.player.BankBalance
		holds = b.player.GetQuantity(galwar.HOLDS)
		fighters = b.player.GetQuantity(galwar.FIGHTERS)
		mines = b.player.GetQuantity(galwar.MINES)
		turns = b.player.GetQuantity(galwar.TURNS)
		planets = len(s.u.OwnedPlanets(b.player))
	})
	s.log.Emit(Event{Day: day, T: s.clock().Unix(), Bot: b.name, Class: b.class, Ev: "day_summary",
		Extra: map[string]any{
			"value": val, "d_value": val - b.dayStartVal, "money": money, "bank": bank,
			"holds": holds, "fighters": fighters, "mines": mines, "planets": planets,
			"turns_left": turns, "actions": b.actionsToday,
			"kills": b.kills, "deaths": b.deaths, "errors": b.errors,
		}})
}

// emitScoreboard records the day's leaderboard (top players by net worth).
func (s *sim) emitScoreboard(day int) {
	var rows []map[string]any
	s.u.Do(func() {
		ranks := s.u.RankedPlayers(s.clock())
		for i, r := range ranks {
			if i >= scoreboardCap {
				break
			}
			rows = append(rows, map[string]any{
				"rank": i + 1, "name": r.Name, "value": r.Value, "dormant": r.Dormant,
			})
		}
	})
	s.log.Emit(Event{Day: day, T: s.clock().Unix(), Ev: "scoreboard", Extra: map[string]any{"top": rows}})
}

// captureFactionState emits an event whenever a faction wakes or sleeps, so the
// digest shows the NPC-AI reacting to a busy world (its whole design premise,
// never before exercised at scale).
func (s *sim) captureFactionState(day int) {
	if s.factionPrev == nil {
		// every fresh universe starts with both factions dormant, so "0" is the
		// true baseline - a night-1 wake must emit, not silently become baseline
		s.factionPrev = map[string]string{"cabal_active": "0", "ren_active": "0"}
	}
	for _, key := range []string{"cabal_active", "ren_active"} {
		var v string
		s.u.Do(func() { v = s.u.ConfigString(key, "0") })
		if s.factionPrev[key] != v {
			s.log.Emit(Event{Day: day, T: s.clock().Unix(), Ev: "faction",
				Extra: map[string]any{"faction": key, "active": v == "1"}})
		}
		s.factionPrev[key] = v
	}
}
