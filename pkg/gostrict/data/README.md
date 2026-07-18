# gostrict dictionaries

These files are the compiled-in vocabulary for `gostrict`. They are derived
from **rustrict** (https://github.com/finnbear/rustrict) by Finn Bear, used
under its MIT / Apache-2.0 dual license. gostrict is an independent Go port of
rustrict's core matching algorithm; this directory is a filtered copy of
rustrict's `src/` data.

## Files

| File | Source (rustrict `src/`) | Contents |
|---|---|---|
| `profanity.csv` | `profanity.csv` | `word,profane,offensive,sexual,mean,evasive` — weights 0–5 per category. A leading space on a word (e.g. ` ass`) is a required word boundary. |
| `false_positives.txt` | `false_positives.txt` | Benign words/phrases (e.g. `assassin`, `glass`) that cancel any profanity they contain. The Scunthorpe fix. |
| `safe.txt` | `safe.txt` | Explicitly benign phrases. |
| `replacements.csv` | `replacements.csv` | `char,alternates` leet/confusable map (e.g. `4,4a`). |
| `safe_extra.txt` | *(gostrict's own)* | Well-known proper nouns rustrict's lists miss (`scunthorpe`, `dickens`, `assange`, …). Grow this from real false positives. |

## Scope: ASCII only

gostrict targets short, user-visible ASCII strings (player names, place names).
The conversion **drops every non-ASCII row** from the upstream files. This is
why the following rustrict machinery is intentionally NOT ported (see the
package's `censor.go`): Unicode confusables / `unicode_fonts.csv`, diacritic
folding, `character_widths.bin`/zalgo handling, the chat `context` tracker,
censor (asterisk) output, PII detection, and spam scoring. If gostrict is ever
pointed at a non-ASCII or full-chat domain, revisit these.

## Re-syncing from upstream

```sh
U=https://raw.githubusercontent.com/finnbear/rustrict/master/src
for f in profanity.csv false_positives.txt safe.txt replacements.csv; do
  curl -sSL "$U/$f" -o /tmp/$f
done
# profanity.csv: keep header + ASCII rows (preserve significant leading spaces)
grep -aP '^[\x00-\x7F]*$' /tmp/profanity.csv > profanity.csv
# text lists: ASCII rows, drop comments (# ...) and blanks
grep -aP '^[\x00-\x7F]*$' /tmp/false_positives.txt | grep -avE '^\s*(#|$)' > false_positives.txt
grep -aP '^[\x00-\x7F]*$' /tmp/safe.txt          | grep -avE '^\s*(#|$)' > safe.txt
# replacements.csv: ASCII single-char source rows ('#' is a valid source char)
grep -aP '^[\x00-\x7F],' /tmp/replacements.csv > replacements.csv
```

Do NOT strip lines beginning with `#` from `replacements.csv` — `#` is a real
leet source character (`#,#ah`). `safe_extra.txt` is hand-maintained and never
overwritten by a re-sync.
