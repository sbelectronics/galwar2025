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
)

const (
	MinNameLen = 3
	MaxNameLen = 20
)

// allowed raw shape: letters, digits, and single internal ' - . or space
var nameCharset = regexp.MustCompile(`^[A-Za-z0-9 .'-]+$`)

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

// profanity is matched as a substring of the normal form. Seed list of
// unambiguous terms; extend from the rejection audit log over time.
var profanity = []string{
	"fuck", "shit", "cunt", "nigger", "nigga", "faggot", "asshole",
	"bitch", "wanker", "dickhead", "cocksuck", "pussy", "whore",
	"slut", "retard", "rapist", "hitler", "nazi", "kike", "spic",
	"chink", "wetback", "tranny",
}

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

	// layer 3a: profanity on the normal form
	for _, w := range profanity {
		if strings.Contains(norm, w) {
			return fmt.Errorf("that handle contains language that isn't allowed")
		}
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
