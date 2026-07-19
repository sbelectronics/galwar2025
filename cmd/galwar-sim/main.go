// Command galwar-sim runs the scripted-bot simulation (PLAN-BOTS.md): it
// generates a fresh universe, sets a fleet of computer-controlled players loose
// on it through the real ConsoleUI, compresses days, and writes a structured
// event log plus a condensed digest for analysis.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/sbelectronics/galwar/pkg/botsim"
)

func main() {
	days := flag.Int("days", 30, "simulated days to run")
	seed := flag.Int64("seed", 1, "RNG seed (a deterministic run is a pure function of seed+fleet)")
	sectors := flag.Int("sectors", 1000, "universe size in sectors")
	fleetSpec := flag.String("fleet", "", "fleet as class=count,... (default: the standard mix)")
	out := flag.String("out", "", "output directory (default: simruns/<seed>)")
	concurrent := flag.Bool("concurrent", false, "run bots in parallel (stress test; not reproducible)")
	transcripts := flag.Bool("transcripts", false, "write per-bot UI transcripts")
	strict := flag.Bool("strict", false, "stop the run on the first finding (CI mode)")
	knownMap := flag.Bool("knownmap", false, "pre-seed every bot's memory with the port map")
	flag.Parse()

	cfg := botsim.Config{
		Days:        *days,
		Seed:        *seed,
		Sectors:     *sectors,
		Out:         *out,
		Concurrent:  *concurrent,
		Transcripts: *transcripts,
		Strict:      *strict,
		KnownMap:    *knownMap,
	}
	if cfg.Out == "" {
		cfg.Out = fmt.Sprintf("simruns/seed%d", *seed)
	}
	if *fleetSpec != "" {
		fleet, err := parseFleet(*fleetSpec)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad -fleet:", err)
			os.Exit(2)
		}
		cfg.Fleet = fleet
	}

	s, err := botsim.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
	fmt.Printf("running %d days, seed %d, %d sectors, fleet %s -> %s\n",
		cfg.Days, cfg.Seed, cfg.Sectors, fleetString(cfg.Fleet), cfg.Out)

	findings, err := s.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	fmt.Printf("done: %d findings. See %s/digest.txt and %s/events.jsonl\n", findings, cfg.Out, cfg.Out)
	if findings > 0 {
		os.Exit(3) // non-zero so CI notices findings
	}
}

// parseFleet parses "trader=3,aggressor=2" into a class->count map.
func parseFleet(spec string) (map[string]int, error) {
	out := map[string]int{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("expected class=count, got %q", part)
		}
		n, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("bad count in %q", part)
		}
		if n > 0 {
			out[strings.TrimSpace(kv[0])] = n
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no bots specified")
	}
	return out, nil
}

func fleetString(fleet map[string]int) string {
	keys := make([]string, 0, len(fleet))
	for k := range fleet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, fleet[k]))
	}
	return strings.Join(parts, ",")
}
