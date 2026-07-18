package gostrict

// trie is the word index rustrict walks during matching (src/trie.rs). Each
// terminal node carries a Type; a terminal with Type None is a false positive
// (a benign word that cancels overlapping profanity), a terminal with the Safe
// bit is an allow-listed phrase, and any other terminal is profanity.
//
// Words may contain spaces (e.g. "hail itler"); a leading space (" ass")
// encodes a required word boundary, matched by a separator in the input.

type trieNode struct {
	children map[rune]*trieNode
	word     bool
	typ      Type
	depth    int
}

func newTrieNode(depth int) *trieNode {
	return &trieNode{children: map[rune]*trieNode{}, depth: depth}
}

// add inserts a word with its type. Re-adding a word ORs the new type in, so a
// term listed under several categories accumulates them.
func (n *trieNode) add(word string, typ Type) {
	cur := n
	for _, r := range word {
		ch := cur.children[r]
		if ch == nil {
			ch = newTrieNode(cur.depth + 1)
			cur.children[r] = ch
		}
		cur = ch
	}
	cur.word = true
	cur.typ |= typ
}
