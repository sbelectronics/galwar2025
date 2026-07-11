package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
)

func main() {
	u := galwar.NewUniverse()
	u.SetFilename("universe.yaml")

	if !u.FileExist() {
		u.Generate(2000)
		if err := u.Save(); err != nil {
			fmt.Printf("Error saving universe: %v\n", err)
			return
		}
	} else {
		if err := u.Load(); err != nil {
			fmt.Printf("Error loading universe: %v\n", err)
			return
		}
	}

	u.Start()

	var player *galwar.Player
	u.Do(func() {
		player = u.Players.GetByEmail("theplayer@gmail.com")
		if player == nil {
			player = u.NewPlayer("Defs Sacre", "theplayer@gmail.com")
		}
	})

	ui := consoleui.NewConsoleUI(u, player)
	done := make(chan struct{})
	go func() {
		ui.Run()
		close(done)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	select {
	case <-done:
	case <-sig:
		fmt.Printf("\nInterrupted - saving universe...\n")
	}

	if err := u.DoErr(u.Save); err != nil {
		fmt.Printf("Error saving universe: %v\n", err)
	}
}
