package gostrict

import (
	"bufio"
	"os"
	"testing"
)

// criticalClean are strings that MUST NOT be flagged - the Scunthorpe corpus.
// Every one embeds a profanity substring but is a legitimate name or word.
var criticalClean = []string{
	"Crassus", "Cassius", "Cassandra", "Assange", "Assisi",
	"Conspicuous", "Class", "Classic", "Classy Lady", "Grass",
	"Glass", "Carcass", "Bassist", "Analyst", "Assassin",
	"Cockpit Charlie", "Cockburn", "Dickens", "Dickinson",
	"Scunthorpe", "Penistone", "Lightwater", "Clitheroe",
	"Shitake Mushroom", "Matsushita", "Wieners",
	"Hello there", "Push it", "Massachusetts", "Sussex", "Essex",
	"Titan", "Titania", "Uranus", "Ballistic", "Shuttlecock",
	"Federation Supply", "Not-Yet-Named port", "Sol", "Betelgeuse",
}

// criticalProfane are strings that MUST be flagged, including leet and
// separator evasions a player would actually try in a handle.
var criticalProfane = []string{
	"fuck", "FUCK", "shit", "bitch", "asshole", "nigger", "faggot",
	"cunt", "$hit", "sh1t", "f*u*c*k", "f-u-c-k", "shhhhit",
	"fuuuck", "n1gger", "phuck", "diccck", "b1tch", "pussy",
}

func TestCriticalClean(t *testing.T) {
	for _, s := range criticalClean {
		if IsInappropriate(s) {
			t.Errorf("false positive: %q flagged (%#o) but should be clean", s, uint32(Analyze(s)))
		}
	}
}

func TestCriticalProfane(t *testing.T) {
	for _, s := range criticalProfane {
		if !IsInappropriate(s) {
			t.Errorf("false negative: %q not flagged but should be", s)
		}
	}
}

// readCorpus loads a newline-delimited test corpus.
func readCorpus(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// TestNegativeCorpus runs rustrict's own "should be clean" corpus. It is a
// regression guard, not a promise of perfection: the corpus contains chat-era
// phrasing our name-focused ASCII port doesn't all model, and rustrict itself
// only reaches ~76% negative accuracy here. We assert a high floor and list
// any regressions.
func TestNegativeCorpus(t *testing.T) {
	corpus := readCorpus(t, "testdata/negative.txt")
	var flagged []string
	for _, s := range corpus {
		if IsInappropriate(s) {
			flagged = append(flagged, s)
		}
	}
	rate := float64(len(corpus)-len(flagged)) / float64(len(corpus))
	t.Logf("negative corpus: %d/%d clean (%.1f%%)", len(corpus)-len(flagged), len(corpus), rate*100)
	for _, s := range flagged {
		t.Logf("  flagged (false positive): %q -> %#o", s, uint32(Analyze(s)))
	}
	if rate < 0.85 {
		t.Errorf("negative corpus pass rate %.1f%% below 85%% floor", rate*100)
	}
}

// TestPositiveCorpus runs rustrict's "should be flagged" corpus as a floor.
func TestPositiveCorpus(t *testing.T) {
	corpus := readCorpus(t, "testdata/positive.txt")
	missed := 0
	for _, s := range corpus {
		if !IsInappropriate(s) {
			missed++
		}
	}
	rate := float64(len(corpus)-missed) / float64(len(corpus))
	t.Logf("positive corpus: %d/%d flagged (%.1f%%)", len(corpus)-missed, len(corpus), rate*100)
	if rate < 0.80 {
		t.Errorf("positive corpus catch rate %.1f%% below 80%% floor", rate*100)
	}
}

func TestSafeCorpus(t *testing.T) {
	for _, s := range readCorpus(t, "testdata/safe.txt") {
		if IsInappropriate(s) {
			t.Errorf("safe phrase flagged: %q -> %#o", s, uint32(Analyze(s)))
		}
	}
}

func TestTypeFlags(t *testing.T) {
	// severity ordering within a category
	if Mild&Moderate != 0 || Moderate&Severe != 0 {
		t.Error("severity masks overlap")
	}
	// a clearly severe slur should register offensive severity
	if Analyze("nigger").Isnt(Offensive) {
		t.Error("racial slur not classified offensive")
	}
	// Is/Isnt duals
	got := Analyze("fuck")
	if got.Is(Profane) == got.Isnt(Profane) {
		t.Error("Is and Isnt disagree")
	}
}

func TestAddSafeAndWord(t *testing.T) {
	c := New()
	// a made-up clean word containing no profanity, then teach it as profane
	if c.IsInappropriate("znorptown") {
		t.Fatal("precondition failed: znorptown already flagged")
	}
	c.AddWord("znorp", Offensive|Severe)
	if !c.IsInappropriate("znorptown") {
		t.Error("AddWord did not take effect")
	}
	// now allow-list the specific name
	c.AddSafe("znorptown")
	if c.IsInappropriate("znorptown") {
		t.Error("AddSafe did not cancel the profanity it covers")
	}
	// the default censor must be unaffected by the custom one
	if IsInappropriate("znorptown") {
		t.Error("custom vocabulary leaked into the shared default censor")
	}
}

func BenchmarkAnalyzeName(b *testing.B) {
	names := []string{"Crassus", "fuck", "Scunthorpe", "Warlord Prime", "$h1tl0rd"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Analyze(names[i%len(names)])
	}
}
