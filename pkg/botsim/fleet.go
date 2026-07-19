package botsim

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
)

// classDisplay maps a fleet-spec class key to the player-handle prefix and the
// log's class label. Handles must pass moderation (no reserved words, right
// shape), so the display names are chosen accordingly.
var classDisplay = map[string]string{
	"trader":    "Trader",
	"planet":    "PlanetBuilder",
	"defensive": "DefensiveTrader",
	"aggressor": "Aggressor",
	"banker":    "Banker",
	"explorer":  "Explorer",
	"casual":    "Casual",
	"chaos":     "ChaosMonkey",
}

// DefaultFleet is the standard mix. It grows as each class is implemented;
// unimplemented classes are omitted so the default always runs.
func DefaultFleet() map[string]int {
	return map[string]int{
		"trader":    3,
		"planet":    2,
		"defensive": 2,
		"aggressor": 2,
		"banker":    1,
		"explorer":  1,
		"casual":    1,
		"chaos":     1,
	}
}

// newBrain constructs the Brain for a class. Unknown/not-yet-implemented
// classes return an error, so a fleet spec naming one fails loudly at setup
// rather than silently dropping bots.
func newBrain(class, name string, rng *rand.Rand, prices Prices) (Brain, error) {
	switch class {
	case "trader":
		return NewTrader(name, "trader", rng), nil
	case "planet":
		return NewPlanetBuilder(name, rng), nil
	case "defensive":
		return NewDefensiveTrader(name, rng), nil
	case "banker":
		return NewBanker(name, rng), nil
	case "aggressor":
		return NewAggressor(name, rng), nil
	case "explorer":
		return NewExplorer(name, rng), nil
	case "casual":
		return NewCasual(name, rng), nil
	case "chaos":
		return NewChaosMonkey(name, rng), nil
	default:
		return nil, fmt.Errorf("bot class %q is not implemented yet", class)
	}
}

// buildFleet registers a player and wires a bot for every entry in the fleet
// spec, in a deterministic order (sorted by class, then index) so a seed
// reproduces the same fleet.
func (s *sim) buildFleet() error {
	classes := make([]string, 0, len(s.cfg.Fleet))
	for c := range s.cfg.Fleet {
		classes = append(classes, c)
	}
	sort.Strings(classes)

	idx := 0
	for _, class := range classes {
		display, ok := classDisplay[class]
		if !ok {
			return fmt.Errorf("unknown bot class %q", class)
		}
		for n := 1; n <= s.cfg.Fleet[class]; n++ {
			name := fmt.Sprintf("%s-%d", display, n)
			rng := rand.New(rand.NewSource(s.cfg.Seed + int64(idx) + 1))
			brain, err := newBrain(class, name, rng, s.prices)
			if err != nil {
				return err
			}

			var player *galwar.Player
			regErr := s.u.DoErr(func() error {
				p, err := s.u.RegisterPlayer(name, name+"@bots.sim", "")
				player = p
				return err
			})
			if regErr != nil {
				return fmt.Errorf("registering %s: %w", name, regErr)
			}

			b := &bot{
				sim:    s,
				index:  idx,
				name:   name,
				class:  class,
				brain:  brain,
				player: player,
				mem:    NewMemory(s.u.ConfigInt("dormant_days", 5)),
				rng:    rng,
				grant:  make(chan struct{}),
				ready:  make(chan struct{}),
				done:   make(chan bool),
				wake:   make(chan struct{}),
			}
			if f, ok := brain.(promptFiller); ok {
				b.filler = f // a fuzzer: answers every prompt instead of desyncing
			}
			b.term = newBotTerm(b)
			b.ui = consoleui.NewConsoleUI(s.u, player, b.term)
			if s.cfg.Transcripts {
				if w, err := s.openTranscript(name); err == nil {
					b.trans = w
				}
			}
			s.bots = append(s.bots, b)
			idx++
		}
	}
	if len(s.bots) == 0 {
		return fmt.Errorf("empty fleet")
	}
	if s.cfg.KnownMap {
		s.seedKnownMap()
	}
	return nil
}

// seedKnownMap pre-populates every bot's memory with the full port map (as if
// each had a map website), for fast economy-convergence experiments. The
// default is honest discovery; this is the -knownmap escape hatch.
func (s *sim) seedKnownMap() {
	s.u.Do(func() {
		for sec := 1; sec < len(s.u.Sectors); sec++ {
			warps := append([]int(nil), s.u.Sectors[sec].GetWarps()...)
			var ports []PortView
			for _, obj := range s.u.GetObjectsInSector(sec, galwar.TYPE_PORT) {
				if p, ok := obj.(*galwar.Port); ok {
					ports = append(ports, portView(s.bots[0].player, p, true))
				}
			}
			sv := SectorView{Sector: sec, Warps: warps, Ports: ports}
			for _, b := range s.bots {
				b.mem.Observe(sv, b.player.Id, 0, true)
				b.mem.MarkVisited(sec)
			}
		}
	})
}
