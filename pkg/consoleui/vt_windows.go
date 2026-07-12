//go:build windows

package consoleui

import (
	"golang.org/x/sys/windows"
)

// enableVT turns on ANSI escape processing in the Windows console.
// Returns false when stdout is not a console (piped/redirected), which
// conveniently disables color for scripted runs.
func enableVT() bool {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if err := windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return false
	}
	return true
}
