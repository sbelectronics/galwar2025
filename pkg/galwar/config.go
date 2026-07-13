package galwar

import (
	"strconv"
)

// Config is the sysop-tunable settings table (the descendant of the
// original's ginfo record / GALWAR.CTL). Values live in the universe and are
// persisted in the config table, where they can be inspected and edited with
// any sqlite3 client. Readers always supply a default, so a missing key is
// never an error.

func (u *UniverseType) ConfigString(key string, def string) string {
	if u.Config == nil {
		return def
	}
	if v, ok := u.Config[key]; ok {
		return v
	}
	return def
}

func (u *UniverseType) ConfigInt(key string, def int) int {
	if u.Config == nil {
		return def
	}
	v, ok := u.Config[key]
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (u *UniverseType) SetConfig(key string, value string) {
	if u.Config == nil {
		u.Config = map[string]string{}
	}
	u.Config[key] = value
	u.MarkDirty()
}

// SeedDefaultConfig writes the default settings into a fresh universe so the
// tunable keys are discoverable in the database.
func (u *UniverseType) SeedDefaultConfig() {
	defaults := map[string]string{
		"numsec":             "2000",
		"starting_credits":   "35000",
		"starting_holds":     "25",
		"starting_fighters":  "200",
		"turns_per_day":      "250",
		"planet_max_mines":   "1000",
		"cost_of_repair":     "250",
		"max_holds":          "16384",
		"cost_of_fighter":    "98",
		"cost_of_hold":       "500",
		"cost_of_mine":       "15000",
		"cost_of_genesis":    "10000",
		"cost_of_plasma":     "56000",
		"cost_of_pulsar":     "215000",
		"cost_of_escape":     "27000",
		"cost_of_cloak":      "18000",
		"cost_of_anticloak":  "22000",
		"cost_of_pulsartube": "350000",
		"max_warps":          "8",
		"dormant_days":       "5",
		"expire_days":        "30",
		"admins":             "",
		// NPC faction AI. Factions are dormant until the world
		// is populated and someone worth challenging emerges; they sleep again
		// if activity falls off. No calendar activation.
		"cabal_min_players":         "3",
		"cabal_wake_value":          "536000", // ~8x starting kit
		"cabal_sleep_value":         "268000", // ~4x kit (hysteresis low end)
		"cabal_scale_pct":           "35",     // Cabal target strength, % of leader value
		"cabal_max_planet_fighters": "15000",  // cap on a gated Cabal stronghold
		"ren_min_players":           "2",
		"ren_target_value":          "134000", // ~2x kit; gentle, population-scaled
		"faction_quiet_days":        "3",      // no logins this long -> all factions sleep
		"faction_target_floor":      "200000", // ~3x kit; no faction targets a player below this
		"cabal_active":              "0",      // faction state (persisted)
		"ren_active":                "0",
	}
	if u.Config == nil {
		u.Config = map[string]string{}
	}
	for k, v := range defaults {
		if _, ok := u.Config[k]; !ok {
			u.Config[k] = v
		}
	}
}
