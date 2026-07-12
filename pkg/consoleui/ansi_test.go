package consoleui

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	in := LightRed + "Sector: 10" + Reset + "\n" + Green + "Warps" + Reset
	want := "Sector: 10\nWarps"
	if got := StripANSI(in); got != want {
		t.Errorf("StripANSI = %q; want %q", got, want)
	}
	if got := StripANSI("plain text"); got != "plain text" {
		t.Errorf("StripANSI mangled plain text: %q", got)
	}
}

func TestHelpLine(t *testing.T) {
	out := HelpLine("[A] Attack")
	// bracket cyan, letter white, closing bracket cyan, text light cyan
	if !strings.Contains(out, Cyan+"["+White) {
		t.Errorf("opening bracket not colored per the original: %q", out)
	}
	if !strings.Contains(out, Cyan+"]"+LightCyan) {
		t.Errorf("closing bracket not colored per the original: %q", out)
	}
	if !strings.HasSuffix(out, Reset) {
		t.Errorf("help line does not reset: %q", out)
	}
	if got := StripANSI(out); got != "[A] Attack" {
		t.Errorf("HelpLine altered the text: %q", got)
	}
}
