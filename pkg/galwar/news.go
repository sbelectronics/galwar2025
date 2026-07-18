package galwar

// Per-player durable news - the descendant of the original's add_playernews.
// Every inter-player interaction resolves in the actor's single move (the
// defender may be offline, as in the original), so the defender learns what
// happened from their news at the next session start. There is deliberately
// no interactive path: stored state in, stored news out.

type NewsItem struct {
	Player    PlayerId
	At        int64
	Msg       string
	Delivered bool
}

// maxNewsItems bounds the global news list (the original trimmed its log to
// 75 lines and messages to 3 days; we trim by count here and by age in
// daily maintenance). maxNewsPerPlayer additionally bounds any one player's
// share, so a noisy attacker (or a busy faction night) can't push another
// player's undelivered "you were killed" notice off the end of the global list.
const (
	maxNewsItems     = 2000
	maxNewsPerPlayer = 50
)

// AddNews queues a news line for a player. NPC owners don't read the news.
func (u *UniverseType) AddNews(id PlayerId, now int64, msg string) {
	if p := u.Players.GetById(id); p == nil || p.IsNPC() {
		return
	}
	u.News = append(u.News, &NewsItem{Player: id, At: now, Msg: msg})

	// per-player cap: drop this player's oldest items (News is append-ordered,
	// so oldest-first) before the global cap can evict anyone else's news
	count := 0
	for _, n := range u.News {
		if n.Player == id {
			count++
		}
	}
	if drop := count - maxNewsPerPlayer; drop > 0 {
		kept := u.News[:0]
		for _, n := range u.News {
			if drop > 0 && n.Player == id {
				drop--
				continue
			}
			kept = append(kept, n)
		}
		u.News = kept
	}

	if len(u.News) > maxNewsItems {
		u.News = u.News[len(u.News)-maxNewsItems:]
	}
	u.MarkDirty()
}

// TakeNews returns a player's undelivered news, oldest first, and marks it
// delivered.
func (u *UniverseType) TakeNews(id PlayerId) []string {
	var msgs []string
	for _, n := range u.News {
		if n.Player == id && !n.Delivered {
			msgs = append(msgs, n.Msg)
			n.Delivered = true
		}
	}
	if len(msgs) > 0 {
		u.MarkDirty()
	}
	return msgs
}

// trimNews drops delivered items older than cutoff (called from daily
// maintenance, like the original's trim_message 3-day rule).
func (u *UniverseType) trimNews(cutoff int64) {
	kept := u.News[:0]
	for _, n := range u.News {
		if n.Delivered && n.At < cutoff {
			continue
		}
		kept = append(kept, n)
	}
	u.News = kept
}
