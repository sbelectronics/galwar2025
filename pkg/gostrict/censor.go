package gostrict

import "strings"

// The matcher is a clean-room port of rustrict's core censor algorithm
// (src/censor.rs, src/mtch.rs), restricted to the load-bearing behaviours for
// ASCII text: a simultaneous trie walk from every input position, with
// tolerance for leet substitutions, repeated characters, and skipped
// punctuation, and with false-positive/safe spans cancelling the profanity
// they cover. The Unicode-confusable, self-censor-spam, and context-tracking
// layers of the original are intentionally omitted (see data/README.md).

// deviation budgets, relative to a matched word's length (node depth). A match
// that strays further than this from the literal word is discarded before it
// can be accepted, so we don't "find" profanity in arbitrary noise.
const (
	maxRepeats = 20 // capped run of one repeated character ("shhhhit")
)

// pmatch is one in-flight walk of the trie.
type pmatch struct {
	node       *trieNode
	start      int  // input index where the walk was seeded
	firstAlnum int  // index of first alphanumeric char consumed (-1 = none yet)
	lastAlnum  int  // index of last alphanumeric char consumed
	skips      int  // punctuation characters skipped mid-word ("f*u*c*k")
	repl       int  // leet substitutions used ("$hit")
	repeats    int  // repeated characters absorbed ("fuuuck")
	lastRaw    rune // last raw input rune consumed, for repetition detection
}

func (m pmatch) deviation() int { return m.skips + m.repl + m.repeats }

// acceptable reports whether a completed word (m.node.word) is a confident
// enough match to count. The deviation must stay within the word's own length,
// and at least one real character must have been consumed.
func (m pmatch) acceptable() bool {
	if m.firstAlnum < 0 {
		return false
	}
	d := m.node.depth
	return m.skips <= d && m.repl <= d
}

type span struct {
	start, end int
	typ        Type
}

// isAlnum reports an ASCII letter or digit (input is pre-lowercased).
func isAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// analyze walks input against the trie and returns the OR of every profanity
// type that survives false-positive and safe-span cancellation.
func (c *Censor) analyze(input string) Type {
	// Lowercase and pad with separators so a leading/trailing word boundary is
	// a real character the trie's boundary spaces can match against.
	runes := []rune(" " + strings.ToLower(input) + " ")
	root := c.root

	type key struct {
		n     *trieNode
		start int
	}
	active := map[key]pmatch{}
	var spans []span

	addTo := func(m map[key]pmatch, pm pmatch) {
		k := key{pm.node, pm.start}
		if old, ok := m[k]; ok && pm.deviation() >= old.deviation() {
			return // keep the cheaper walk to the same place
		}
		m[k] = pm
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		alnum := isAlnum(r)
		hardSpace := r == ' '

		// Seed a fresh walk at this position before iterating (safe: we mutate
		// `active` only here, then read it below and write only to `next`).
		active[key{root, i}] = pmatch{node: root, start: i, firstAlnum: -1, lastAlnum: -1, lastRaw: -1}

		next := map[key]pmatch{}
		for _, m := range active {
			// Repetition: absorb a rerun of the same raw character, staying put.
			if alnum && r == m.lastRaw && m.repeats < maxRepeats {
				nm := m
				nm.repeats++
				nm.lastAlnum = i
				addTo(next, nm)
			}

			// A separator in the input may satisfy a boundary/space in the trie
			// word (its ' ' child), consuming the separator with no penalty.
			if !alnum {
				if ch := m.node.children[' ']; ch != nil {
					nm := m
					nm.node = ch
					nm.lastRaw = r
					addTo(next, nm)
				}
			}

			// A non-space separator can be skipped mid-word ("f.u.c.k",
			// "f*u*c*k"). A real space is a hard boundary and is never skipped,
			// which is what stops "push it" from matching "shit".
			if !alnum && !hardSpace {
				nm := m
				nm.skips++
				nm.lastRaw = r
				if nm.skips <= nm.node.depth+2 {
					addTo(next, nm)
				}
			}

			// Advance through the trie by every character this input rune could
			// stand for (itself, plus leet expansions).
			for _, cnd := range c.expand(r) {
				ch := m.node.children[cnd]
				if ch == nil {
					continue
				}
				nm := m
				nm.node = ch
				nm.lastRaw = r
				if cnd != r {
					nm.repl++
				}
				if isAlnum(cnd) {
					if nm.firstAlnum < 0 {
						nm.firstAlnum = i
					}
					nm.lastAlnum = i
				}
				if nm.repl <= nm.node.depth+1 {
					addTo(next, nm)
				}
			}
		}

		active = next
		for _, m := range active {
			if m.node.word && m.acceptable() {
				spans = append(spans, span{m.firstAlnum, m.lastAlnum + 1, m.node.typ})
			}
		}
	}

	return resolve(spans)
}

// resolve accumulates the profanity types of every match that is not wholly
// contained within a false-positive (Type None) or Safe span. Containment - not
// mere overlap - is the rule, so "ass" inside "assassin" is cancelled while a
// standalone "ass" is not.
func resolve(spans []span) Type {
	covers := func(s span) bool {
		for _, f := range spans {
			if f.typ != None && f.typ.Isnt(Safe) {
				continue // only false-positive / safe spans cancel
			}
			// s is profanity, f is FP/safe (different type), so equal
			// coordinates still mean f cancels s.
			if f.start <= s.start && f.end >= s.end {
				return true
			}
		}
		return false
	}

	var out Type
	for _, s := range spans {
		if s.typ.Isnt(Any) {
			continue // s is itself a false positive or safe phrase
		}
		if covers(s) {
			continue
		}
		out |= s.typ
	}
	return out
}
