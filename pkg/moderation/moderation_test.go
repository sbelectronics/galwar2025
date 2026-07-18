package moderation

import (
	"strings"
	"testing"
)

func TestCheckNameAccepts(t *testing.T) {
	good := []string{
		"Scott", "Defs Sacre", "Trader Bob", "O'Brien", "Jean-Luc",
		"Dr. Zaius", "R2D", "Big Al", "xXNovaXx",
		// cross-word seams must not read as profanity when words are joined:
		// "Fresh Two" contains "sh t" (a dictionary abbreviation) at its seam
		"Fresh Two", "Marsh Trader", "Ash Tree",
	}
	for _, name := range good {
		if err := CheckName(name); err != nil {
			t.Errorf("CheckName(%q) rejected a good name: %v", name, err)
		}
	}
}

func TestCheckNameRejects(t *testing.T) {
	cases := []struct {
		name   string
		reason string
	}{
		{"ab", "too short"},
		{"this handle is way way too long", "too long"},
		{"tab\there", "charset"},
		{"esc\x1b[31mape", "charset (closes ANSI injection)"},
		{"héllo", "charset (ASCII only for v1)"},
		{"-Dash", "must start alnum"},
		{"Dash-", "must end alnum"},
		{"two  spaces", "consecutive spaces"},
		{"...", "nothing left after normalizing"},
		{"fuck", "profanity"},
		{"FuCkFace", "profanity, case"},
		{"f u c k", "profanity, separators"},
		{"F0ckHead sh1t", "profanity... close enough to test leet on sh1t"},
		{"sh1t", "profanity, leetspeak"},
		{"FuckMaster99", "profanity, affix compound"},
		{"S.h.i.t", "profanity, punctuation split"},
		{"fuuuuck", "profanity, repeat stretching"},
		// spread across real spaces: gostrict alone won't span these, the
		// short-fragment second pass must
		{"c u n t", "profanity spread across spaces"},
		{"cu nt", "profanity split by one space"},
		{"fu ck", "profanity split by one space"},
		{"fuc k", "profanity split unevenly"},
		{"John cu nt", "spread slur after an honest word"},
		{"Sysop", "reserved"},
		{"ADMIN", "reserved"},
		{"Federation Cmdr", "reserved prefix"},
		{"sysop2", "reserved prefix"},
		{"buy-gold.com", "url"},
		{"www.spam", "url"},
		{"aaaabob", "repeated run"},
		{"12345678", "digit majority"},
		{"a1234", "digit majority"},
	}
	for _, c := range cases {
		if err := CheckName(c.name); err == nil {
			t.Errorf("CheckName(%q) accepted; want rejection (%s)", c.name, c.reason)
		}
	}
}

// TestCheckNameScunthorpe guards the whole point of the gostrict switch: names
// that merely embed a profanity substring must be accepted.
func TestCheckNameScunthorpe(t *testing.T) {
	for _, name := range []string{
		"Crassus", "Cassius", "Conspicuous", "Classy Lady", "Bassist",
		"Cockpit Charlie", "Dickens", "Scunthorpe", "Grass Hopper",
		"Assassin", "Analyst", "Spice Trader", "Shitake Trader",
		"Matt Hitch", "Cassandra",
	} {
		if err := CheckName(name); err != nil {
			t.Errorf("CheckName(%q) rejected a legitimate name: %v", name, err)
		}
	}
}

func TestCheckPlanetName(t *testing.T) {
	good := []string{
		"New Terra", "Betelgeuse", "Federation Supply", "NGC-1976",
		"O'Brien's Rock", "Xanadu!", "Sector 7 (Home)", "Not-Yet-Named port",
	}
	for _, n := range good {
		if err := CheckPlanetName(n); err != nil {
			t.Errorf("CheckPlanetName(%q) rejected a good name: %v", n, err)
		}
	}
	bad := map[string]string{
		"":                                "empty",
		"   ":                             "blank",
		"FuckWorld":                       "profanity",
		"sh1t planet":                     "profanity leet",
		"esc\x1b[31mape":                  "ANSI escape injection",
		"tab\there":                       "control char",
		"way too long a planet name here": "too long",
		"héllo":                           "non-ASCII",
	}
	for n, why := range bad {
		if err := CheckPlanetName(n); err == nil {
			t.Errorf("CheckPlanetName(%q) accepted; want rejection (%s)", n, why)
		}
	}
}

func TestCheckReportReason(t *testing.T) {
	if err := CheckReportReason("They are spamming the sector with 'ass' jokes."); err != nil {
		t.Errorf("report reason quoting a word was rejected: %v", err)
	}
	if err := CheckReportReason(""); err == nil {
		t.Error("empty report reason accepted")
	}
	if err := CheckReportReason("bad\x1b[2Jinjection"); err == nil {
		t.Error("report reason with ANSI escape accepted")
	}
	if err := CheckReportReason(strings.Repeat("x", MaxReportLen+1)); err == nil {
		t.Error("over-long report reason accepted")
	}
}

func TestAddProfanityHook(t *testing.T) {
	// AddProfanity is a startup hook for config-driven tuning. It mutates the
	// shared censor, so this test is written to stay correct if run twice in one
	// process (-count=2): banning the same word again is a no-op, and we never
	// safe-list it back. (The AddSafe cancellation mechanism is proven in
	// gostrict's own TestAddSafeAndWord against a private censor.)
	AddProfanity("znorpzz")
	if CheckName("Znorpzz Trader") == nil {
		t.Error("AddProfanity did not ban a newly-configured word")
	}
}

func TestNormalize(t *testing.T) {
	// the impersonation collisions layer 4 relies on
	for _, pair := range [][2]string{
		{"Scott", "sc0tt"},
		{"Scott", "S c o t t"},
		{"Scott", "S.C.O.T.T"},
		{"Defs Sacre", "DEFS-SACRE"},
	} {
		if Normalize(pair[0]) != Normalize(pair[1]) {
			t.Errorf("Normalize(%q)=%q != Normalize(%q)=%q", pair[0], Normalize(pair[0]), pair[1], Normalize(pair[1]))
		}
	}
	if got := Normalize("S c 0 t-t!"); got != "scotti" && !strings.HasPrefix(got, "scott") {
		t.Errorf("Normalize leet/separator folding broken: %q", got)
	}
}
