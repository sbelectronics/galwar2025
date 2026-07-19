package botsim

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// writeDigest renders the condensed, human/AI-readable narrative (digest.txt)
// from the event log. It is derived entirely from events.jsonl, so it can be
// re-condensed differently later, and is sized to paste into one analysis
// session (PLAN-BOTS.md 4.2).
func (s *sim) writeDigest() error {
	events := s.log.Events()
	var b strings.Builder

	fmt.Fprintf(&b, "GALACTIC WARZONE - bot simulation digest\n")
	fmt.Fprintf(&b, "seed=%d days=%d sectors=%d bots=%d concurrent=%v\n",
		s.cfg.Seed, s.cfg.Days, s.cfg.Sectors, len(s.bots), s.cfg.Concurrent)
	fmt.Fprintf(&b, "findings=%d\n", s.log.Findings())
	fmt.Fprint(&b, strings.Repeat("=", 60)+"\n")

	byDay := map[int][]Event{}
	maxDay := 0
	for _, e := range events {
		byDay[e.Day] = append(byDay[e.Day], e)
		if e.Day > maxDay {
			maxDay = e.Day
		}
	}

	for day := 1; day <= maxDay; day++ {
		evs := byDay[day]
		if len(evs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n--- Day %d ---\n", day)

		// per-bot one-liners
		var summaries []Event
		var notable []Event
		var board Event
		for _, e := range evs {
			switch e.Ev {
			case "day_summary":
				summaries = append(summaries, e)
			case "scoreboard":
				board = e
			case "died", "faction", "genesis", "invaded", "killed",
				"desync", "invariant", "stuck", "error_unexpected":
				notable = append(notable, e)
			}
		}
		sort.SliceStable(summaries, func(i, j int) bool { return summaries[i].Bot < summaries[j].Bot })
		for _, e := range summaries {
			fmt.Fprintf(&b, "  %s\n", summaryLine(e))
		}
		for _, e := range notable {
			fmt.Fprintf(&b, "  * %s\n", notableLine(e))
		}
		if board.Ev == "scoreboard" {
			fmt.Fprintf(&b, "  standings: %s\n", scoreLine(board))
		}
	}

	fmt.Fprint(&b, "\n"+strings.Repeat("=", 60)+"\n")
	fmt.Fprintf(&b, "FINAL: %d findings over %d days\n", s.log.Findings(), s.cfg.Days)

	return os.WriteFile(filepath.Join(s.cfg.Out, "digest.txt"), []byte(b.String()), 0644)
}

func summaryLine(e Event) string {
	g := func(k string) int { return toInt(e.Extra[k]) }
	dv := g("d_value")
	sign := "+"
	if dv < 0 {
		sign = ""
	}
	extra := ""
	if g("planets") > 0 {
		extra += fmt.Sprintf(" planets %d", g("planets"))
	}
	if g("deaths") > 0 {
		extra += fmt.Sprintf(" DIED %d", g("deaths"))
	}
	if g("errors") > 0 {
		extra += fmt.Sprintf(" errs %d", g("errors"))
	}
	return fmt.Sprintf("%-16s val %d (%s%d) money %d bank %d holds %d ftr %d acts %d%s",
		e.Bot, g("value"), sign, dv, g("money"), g("bank"), g("holds"), g("fighters"), g("actions"), extra)
}

func notableLine(e Event) string {
	switch e.Ev {
	case "faction":
		return fmt.Sprintf("faction %v active=%v", e.Extra["faction"], e.Extra["active"])
	case "died":
		return fmt.Sprintf("%s was destroyed", e.Bot)
	case "desync":
		return fmt.Sprintf("DESYNC %s: %s", e.Bot, oneLine(fmt.Sprint(e.Extra["tail"]), 120))
	case "invariant":
		return fmt.Sprintf("INVARIANT: %v", e.Extra["detail"])
	case "stuck":
		return fmt.Sprintf("STUCK %s: %v", e.Bot, e.Extra["detail"])
	case "error_unexpected":
		return fmt.Sprintf("ERROR %s: %v", e.Bot, e.Extra["detail"])
	default:
		return fmt.Sprintf("%s %s %v", e.Bot, e.Ev, e.Extra)
	}
}

func scoreLine(e Event) string {
	rows, _ := e.Extra["top"].([]map[string]any)
	var parts []string
	for i, r := range rows {
		if i >= 5 {
			break
		}
		parts = append(parts, fmt.Sprintf("%v=%v", r["name"], toInt(r["value"])))
	}
	return strings.Join(parts, "  ")
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func oneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}
