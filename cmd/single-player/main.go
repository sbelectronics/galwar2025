package main

import (
	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
	"sync"
)

func main() {
	var wg sync.WaitGroup

	galwar.InitSectors(2000)
	player := galwar.NewPlayer("Defs Sacre")
	ui := consoleui.NewConsoleUI(player)
	ui.Start(&wg)

	// Wait for all goroutines to complete
	wg.Wait()
}
