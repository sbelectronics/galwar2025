package main

import (
	"fmt"
	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
	"sync"
)

func main() {
	var wg sync.WaitGroup

	galwar.Universe.SetFilename("universe.yaml")

	if !galwar.Universe.FileExist() {
		galwar.InitSectors(2000)
		galwar.Universe.Save()
	} else {
		err := galwar.Universe.Load()
		if err != nil {
			fmt.Printf("Error loading universe: %v\n", err)
			return
		}
	}

	player := galwar.GetPlayer("theplayer@gmail.com")
	if player == nil {
		player = galwar.NewPlayer("Defs Sacre", "theplayer@gmail.com")
	}

	ui := consoleui.NewConsoleUI(player)
	ui.Start(&wg)

	// Wait for all goroutines to complete
	wg.Wait()

	galwar.Universe.Save()
}
