package consoleui

import (
	"regexp"
	"strings"
)

// The original used the doordriv set_foreground() with Turbo Pascal CRT
// color names. These constants are those names, rendered as classic
// BBS-style ANSI (bold + 30-37, maximum client compatibility). Terminals
// that can't or won't do color strip them in their Printf.
const (
	Reset        = "\x1b[0m"
	Black        = "\x1b[0;30m"
	Blue         = "\x1b[0;34m"
	Green        = "\x1b[0;32m"
	Cyan         = "\x1b[0;36m"
	Red          = "\x1b[0;31m"
	Magenta      = "\x1b[0;35m"
	Brown        = "\x1b[0;33m"
	LightGray    = "\x1b[0;37m"
	DarkGray     = "\x1b[1;30m"
	LightBlue    = "\x1b[1;34m"
	LightGreen   = "\x1b[1;32m"
	LightCyan    = "\x1b[1;36m"
	LightRed     = "\x1b[1;31m"
	LightMagenta = "\x1b[1;35m"
	Yellow       = "\x1b[1;33m"
	White        = "\x1b[1;37m"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// StripANSI removes ANSI escape sequences for colorless terminals.
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// HelpLine replicates the original's help-screen colorizer (swriteln1 /
// swriteln2, TWARS.PAS:1204-1264): light-cyan text, cyan brackets, white
// command letters inside the brackets.
func HelpLine(s string) string {
	var b strings.Builder
	b.WriteString(LightCyan)
	for _, r := range s {
		switch r {
		case '[':
			b.WriteString(Cyan)
			b.WriteRune(r)
			b.WriteString(White)
		case ']':
			b.WriteString(Cyan)
			b.WriteRune(r)
			b.WriteString(LightCyan)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(Reset)
	return b.String()
}
