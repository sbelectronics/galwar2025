package gostrict

// Type is a bit-set describing why a string is (in)appropriate. It is a direct
// port of rustrict's Type (src/typ.rs): six 3-bit category fields, each holding
// a severity, plus a SAFE bit.
//
// Within a category the three bits are severity flags - Mild (0b001),
// Moderate (0b010), Severe (0b100) - so a category can carry at most one
// severity. Masks like Profane select a whole category across all severities;
// masks like Moderate select one severity across all categories. Combine them
// (Profane & Severe) to test a specific box.
type Type uint32

const (
	None Type = 0

	// Category masks (3 bits each).
	Profane   Type = 0o000007 // bits 0-2
	Offensive Type = 0o000070 // bits 3-5  (slurs, hate)
	Sexual    Type = 0o000700 // bits 6-8
	Mean      Type = 0o007000 // bits 9-11 (insults)
	Evasive   Type = 0o070000 // bits 12-14 (deliberate obfuscation)
	Spam      Type = 0o700000 // bits 15-17

	// Safe marks a word/phrase as explicitly benign; a Safe span suppresses
	// any profanity it covers (the whitelist half of the Scunthorpe fix).
	Safe Type = 0o1000000 // bit 18

	// Severity masks (one severity across every category).
	Mild     Type = 0o111111
	Moderate Type = 0o222222
	Severe   Type = 0o444444

	MildOrHigher     = Mild | Moderate | Severe
	ModerateOrHigher = Moderate | Severe
	SevereOnly       = Severe

	// Any is every category at every severity - "is this flagged at all".
	Any = Profane | Offensive | Sexual | Mean | Evasive | Spam

	// Inappropriate is rustrict's default reject mask: profane, offensive, or
	// sexual at any severity, plus mean only when severe. Evasive and spam are
	// signals, not verdicts, so they're excluded.
	Inappropriate = Profane | Offensive | Sexual | (Mean & Severe)
)

// Is reports whether any bit in mask is set.
func (t Type) Is(mask Type) bool { return t&mask != 0 }

// Isnt reports whether no bit in mask is set.
func (t Type) Isnt(mask Type) bool { return t&mask == 0 }

// weightBits converts a rustrict CSV weight (0=none, 1=mild, 2=moderate,
// 3+=severe - the source occasionally uses 4/5 as emphatic "severe") into the
// category's 3 severity bits, shifted into place.
func weightBits(weight, shift int) Type {
	if weight <= 0 {
		return 0
	}
	var w Type
	switch weight {
	case 1:
		w = 0o1
	case 2:
		w = 0o2
	default:
		w = 0o4
	}
	return w << shift
}
