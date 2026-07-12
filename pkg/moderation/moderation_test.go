package moderation

import (
	"strings"
	"testing"
)

func TestCheckNameAccepts(t *testing.T) {
	good := []string{
		"Scott", "Defs Sacre", "Trader Bob", "O'Brien", "Jean-Luc",
		"Dr. Zaius", "R2D", "Big Al", "xXNovaXx",
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
