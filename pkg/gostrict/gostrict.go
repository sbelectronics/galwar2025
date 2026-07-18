// Package gostrict is a profanity/inappropriate-language classifier for short,
// user-visible strings such as player names and place names. It is a Go port
// of the core of rustrict (https://github.com/finnbear/rustrict, MIT/Apache),
// keeping that project's key virtue: it does not suffer the Scunthorpe problem.
// "Crassus", "Conspicuous", "assassin", and "glass" pass; "$hit", "f*u*c*k",
// and "shhhhit" do not.
//
// It classifies rather than censors: Analyze returns a Type bit-set of
// categories and severities, and the caller decides policy. A convenience
// Inappropriate applies rustrict's default reject mask.
//
// The dictionaries are embedded, so gostrict is a self-contained package with
// no runtime data files and no dependencies outside the standard library. See
// data/README.md for provenance, the ASCII-only scope, and how to re-sync from
// upstream.
package gostrict

import (
	"bufio"
	"embed"
	"strconv"
	"strings"
)

//go:embed data
var dataFS embed.FS

// Censor holds a compiled dictionary. The zero value is not usable; obtain one
// from New (or use the package-level Analyze/Inappropriate, which share a
// default built from the embedded data).
type Censor struct {
	root *trieNode
	repl map[rune][]rune
}

// New builds a Censor from the embedded dictionaries. Most callers want the
// package-level functions instead; New is for hosts that will layer their own
// vocabulary on top with AddWord/AddSafe.
func New() *Censor {
	c := &Censor{root: newTrieNode(0), repl: map[rune][]rune{}}
	c.loadReplacements("data/replacements.csv")
	c.loadProfanity("data/profanity.csv")
	c.loadWords("data/false_positives.txt", None)
	c.loadWords("data/safe.txt", Safe)
	c.loadWords("data/safe_extra.txt", Safe)
	return c
}

// AddWord teaches the censor an extra profanity with the given type. Intended
// for one-time setup (e.g. from a config table) before concurrent use; it is
// not safe to call while Analyze is running on another goroutine.
func (c *Censor) AddWord(word string, typ Type) {
	if w := normalizeEntry(word); w != "" {
		c.root.add(w, typ)
	}
}

// AddSafe teaches the censor an extra allow-listed word or phrase - one that
// should never be flagged and that cancels any profanity it contains (the fix
// for a proper noun the dictionaries wrongly catch). Same concurrency caveat
// as AddWord.
func (c *Censor) AddSafe(word string) { c.AddWord(word, Safe) }

// Analyze classifies s, returning the OR of every profanity category/severity
// it contains that is not cancelled by an allow-listed or false-positive span.
func (c *Censor) Analyze(s string) Type { return c.analyze(s) }

// Inappropriate reports whether s trips rustrict's default reject mask
// (profane/offensive/sexual at any severity, or severe meanness).
func (c *Censor) IsInappropriate(s string) bool { return c.analyze(s).Is(Inappropriate) }

// expand returns the characters r could stand for: itself plus any leet
// substitutions. The input is pre-lowercased, so the returned set is lowercase.
func (c *Censor) expand(r rune) []rune {
	if v, ok := c.repl[r]; ok {
		return v
	}
	return []rune{r}
}

func (c *Censor) loadReplacements(name string) {
	eachLine(name, func(line string) {
		rs := []rune(line)
		if len(rs) < 3 || rs[1] != ',' {
			return // want "X,YYY"
		}
		src := toLower(rs[0])
		set := map[rune]struct{}{src: {}}
		var vals []rune
		vals = append(vals, src)
		for _, v := range rs[2:] {
			lv := toLower(v)
			if _, dup := set[lv]; dup {
				continue
			}
			set[lv] = struct{}{}
			vals = append(vals, lv)
		}
		c.repl[src] = vals
	})
}

func (c *Censor) loadProfanity(name string) {
	eachLine(name, func(line string) {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			return
		}
		nums := parts[len(parts)-5:]
		word := normalizeEntry(strings.Join(parts[:len(parts)-5], ","))
		if word == "" || word == "word" { // skip header and blanks
			return
		}
		// columns: profane, offensive, sexual, mean, evasive
		shifts := []int{0, 3, 6, 9, 12}
		var typ Type
		for i, n := range nums {
			w, _ := strconv.Atoi(strings.TrimSpace(n))
			typ |= weightBits(w, shifts[i])
		}
		if typ != None {
			c.root.add(word, typ)
		}
	})
}

func (c *Censor) loadWords(name string, typ Type) {
	eachLine(name, func(line string) {
		if w := normalizeEntry(line); w != "" {
			c.root.add(w, typ)
		}
	})
}

// normalizeEntry lowercases a dictionary entry and drops a single leading
// space back to nothing only if the whole thing is blank. A meaningful leading
// space (the word-boundary marker in " ass") is preserved.
func normalizeEntry(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return strings.ToLower(s)
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

func eachLine(name string, fn func(string)) {
	f, err := dataFS.Open(name)
	if err != nil {
		panic("gostrict: missing embedded data file " + name + ": " + err.Error())
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		// Data files are pre-cleaned of comments/blanks at conversion time (see
		// data/README.md), so every line is a real entry. '#' is a legitimate
		// replacement source character, so no '#'-comment filtering happens here.
		fn(strings.TrimRight(sc.Text(), "\r\n"))
	}
}

// The shared default censor.
var std = New()

// Analyze classifies s using the default embedded dictionaries.
func Analyze(s string) Type { return std.Analyze(s) }

// Inappropriate reports whether s trips the default reject mask, using the
// default embedded dictionaries.
func IsInappropriate(s string) bool { return std.IsInappropriate(s) }
