// Package moderation is the layered player-handle (and, later, chat/mail
// text) screening pipeline described in PLAN.md section 8:
//
//  1. syntactic allowlist - length, charset, shape. This alone closes ANSI
//     injection and most formatting abuse by construction.
//  2. normalization - casefold, leetspeak folding, separator stripping -
//     so every later check sees through trivial disguises.
//  3. denylists on the normal form - profanity, reserved names, spam shapes.
//
// Layer 4 (uniqueness on the normal form) is the caller's job, since it
// needs the player list: use Normalize for the comparison key. Layer 5 (an
// async LLM check for the gray zone) is deliberately not implemented yet -
// registration must never block on an API - and layer 6 (report/rename
// sysop tooling) belongs to a later milestone.
package moderation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sbelectronics/galwar/pkg/gostrict"
)

const (
	MinNameLen = 3
	MaxNameLen = 20

	// MaxPlanetNameLen bounds a genesis planet name (shown to every passer-by).
	MaxPlanetNameLen = 25
	// MaxReportLen bounds a player-report reason (shown to the sysop).
	MaxReportLen = 200
)

// allowed raw shape: letters, digits, and single internal ' - . or space
var nameCharset = regexp.MustCompile(`^[A-Za-z0-9 .'-]+$`)

// planetCharset is a little broader than handles - a planet may have punctuation
// - but is still a strict printable-ASCII allowlist, which is what closes ANSI
// escape injection on the web input path (planet names are Printf'd into every
// viewer's terminal).
var planetCharset = regexp.MustCompile(`^[A-Za-z0-9 .,'!?&()-]+$`)

// censor is the moderation layer's own gostrict instance. Operators can extend
// it at startup (from config) via AddProfanity/AddSafe; it is not safe to
// mutate once name checks are running concurrently.
var censor = gostrict.New()

// nameReject is the policy for user-visible names: any slur or sexual content
// (offensive/sexual, all severities), swearing at moderate-or-worse, and only
// the severe end of plain meanness. Mild swears ("damn", "hell") are tolerated;
// evasion and spam are signals gostrict reports but we don't reject names on.
var nameReject = (gostrict.Profane & gostrict.ModerateOrHigher) |
	gostrict.Offensive | gostrict.Sexual | (gostrict.Mean & gostrict.Severe)

// AddProfanity teaches the moderation censor an extra banned word (severe), and
// AddSafe allow-lists a word wrongly flagged. Call at startup only (e.g. from
// the config table); see gostrict's concurrency note.
func AddProfanity(word string) { censor.AddWord(word, gostrict.Offensive|gostrict.Severe) }

// AddSafe allow-lists a word the dictionaries wrongly flag.
func AddSafe(word string) { censor.AddSafe(word) }

// containsProfanity reports whether s trips the name-rejection policy.
func containsProfanity(s string) bool { return censor.Analyze(s).Is(nameReject) }

// spacedProfanity catches slurs spread across separators ("c u n t", "cu nt"),
// which gostrict alone misses because it deliberately treats a real space as a
// hard word boundary. Joining the WHOLE name and re-checking would trade that
// bug for Scunthorpe's: honest adjacent words form accidental hits at their
// seam ("Fresh Two" contains "sh t", a dictionary abbreviation). The evasion
// signature is a run of short fragments, so only maximal runs of two or more
// short tokens (<= 3 chars, post leet-fold) are joined and re-checked - a
// spread-out slur is all short fragments, while honest words stay long enough
// to break the run.
func spacedProfanity(name string) bool {
	folded := leetFold.Replace(strings.ToLower(name))
	tokens := strings.FieldsFunc(folded, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	var run []string
	flush := func() bool {
		joined := len(run) >= 2 && containsProfanity(strings.Join(run, ""))
		run = run[:0]
		return joined
	}
	for _, tok := range tokens {
		if len(tok) <= 3 {
			run = append(run, tok)
			continue
		}
		if flush() {
			return true
		}
	}
	return flush()
}

// hasRepeatedRun reports 4+ consecutive occurrences of the same character
// (RE2 has no backreferences, so this is a scan).
func hasRepeatedRun(s string) bool {
	run := 0
	var prev rune = -1
	for _, r := range s {
		if r == prev {
			run++
			if run >= 4 {
				return true
			}
		} else {
			prev = r
			run = 1
		}
	}
	return false
}

// leetFold maps common lookalike substitutions to their letters. Applied
// after lowercasing, before separator stripping.
var leetFold = strings.NewReplacer(
	"0", "o",
	"1", "i",
	"3", "e",
	"4", "a",
	"5", "s",
	"7", "t",
	"8", "b",
	"@", "a",
	"$", "s",
	"!", "i",
)

// reserved names may not be used or embedded at the start of a handle:
// impersonating the system or the NPC factions is off the table.
var reserved = []string{
	"sysop", "admin", "administrator", "moderator", "system", "server",
	"federation", "thefederation", "thecabal", "cabal", "renegade",
	"therenegades", "sol", "galwar", "interstel",
}

// spamMarkers are checked against the lowercased raw name (before separator
// stripping, since URLs need their dots).
var spamMarkers = []string{
	"http:", "https:", "www.", ".com", ".net", ".org", ".io", ".gg",
}

// Normalize reduces a name to the form used for denylist matching and
// uniqueness comparison: lowercased, leetspeak folded, separators stripped.
// "Scott", "sc0tt", and "S c o t t" all normalize identically.
func Normalize(name string) string {
	s := strings.ToLower(name)
	s = leetFold.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CheckName validates a proposed player handle. A nil return means the
// handle passes every layer; otherwise the error says which rule failed in
// terms suitable to show the player.
func CheckName(name string) error {
	// layer 1: shape
	if len(name) < MinNameLen {
		return fmt.Errorf("that handle is too short (minimum %d characters)", MinNameLen)
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("that handle is too long (maximum %d characters)", MaxNameLen)
	}
	if !nameCharset.MatchString(name) {
		return fmt.Errorf("handles may only contain letters, digits, spaces, and . ' -")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("handles may not start or end with a space")
	}
	if strings.Contains(name, "  ") {
		return fmt.Errorf("handles may not contain consecutive spaces")
	}
	first := name[0]
	last := name[len(name)-1]
	if !isAlnum(first) || !isAlnum(last) {
		return fmt.Errorf("handles must start and end with a letter or digit")
	}

	lower := strings.ToLower(name)
	norm := Normalize(name)

	// normalizing everything away ("...", "@ $") leaves nothing to name
	if len(norm) < MinNameLen {
		return fmt.Errorf("that handle needs at least %d letters or digits", MinNameLen)
	}

	// layer 3a: profanity, two passes. First gostrict on the raw name - its own
	// leet/punctuation-aware matching catches "sh1t" and "s.h.i.t" without the
	// Scunthorpe false positives a substring list would produce ("Crassus").
	// Then the short-fragment pass for slurs spread across real spaces, which
	// gostrict deliberately never spans (see spacedProfanity).
	if containsProfanity(name) || spacedProfanity(name) {
		return fmt.Errorf("that handle contains language that isn't allowed")
	}

	// layer 3b: reserved / impersonation
	for _, w := range reserved {
		if norm == w || strings.HasPrefix(norm, w) {
			return fmt.Errorf("that handle impersonates the system or a faction")
		}
	}

	// layer 3c: spam shapes
	for _, m := range spamMarkers {
		if strings.Contains(lower, m) {
			return fmt.Errorf("handles may not contain URLs")
		}
	}
	if hasRepeatedRun(lower) {
		return fmt.Errorf("handles may not repeat a character more than three times")
	}
	if digitMajority(name) {
		return fmt.Errorf("handles may not be mostly digits")
	}

	return nil
}

// CheckPlanetName validates a genesis planet name. Like a handle it must be
// printable ASCII (which closes ANSI-escape injection, since planet names are
// shown to every player in the sector) and free of profanity, but it is NOT
// subject to the reserved-name or spam rules - a planet may legitimately be
// called "Federation Supply" or be mostly stylized. Uniqueness is the engine's
// concern, not this function's.
func CheckPlanetName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a planet needs a name")
	}
	if len(name) > MaxPlanetNameLen {
		return fmt.Errorf("that name is too long (maximum %d characters)", MaxPlanetNameLen)
	}
	if !planetCharset.MatchString(name) {
		return fmt.Errorf("planet names may only contain letters, digits, spaces, and . , ' - ! ? & ( )")
	}
	if strings.Contains(name, "  ") {
		return fmt.Errorf("planet names may not contain consecutive spaces")
	}
	if containsProfanity(name) || spacedProfanity(name) {
		return fmt.Errorf("that name contains language that isn't allowed")
	}
	return nil
}

// CheckReportReason validates the free-text reason on a player report. The
// reason is read by the sysop, so this filters for injection (printable ASCII,
// bounded length) rather than vocabulary - a report should be able to quote
// what it is reporting.
func CheckReportReason(reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("please say why you are reporting them")
	}
	if len(reason) > MaxReportLen {
		return fmt.Errorf("that reason is too long (maximum %d characters)", MaxReportLen)
	}
	for _, r := range reason {
		if r < 0x20 || r > 0x7e {
			return fmt.Errorf("the reason may only contain plain printable characters")
		}
	}
	return nil
}

func isAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func digitMajority(name string) bool {
	digits, letters := 0, 0
	for _, r := range name {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			letters++
		}
	}
	return digits > letters
}
