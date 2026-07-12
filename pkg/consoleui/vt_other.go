//go:build !windows

package consoleui

// enableVT: non-Windows terminals have handled ANSI since before this game
// existed.
func enableVT() bool {
	return true
}
