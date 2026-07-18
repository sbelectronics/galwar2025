package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/sbelectronics/galwar/pkg/consoleui"
	"github.com/sbelectronics/galwar/pkg/galwar"
)

func main() {
	dbPath := flag.String("db", "galwar.db", "path to the game database")
	yamlPath := flag.String("yaml", "universe.yaml", "legacy YAML universe (migrated into the database if it exists and the database is empty)")
	dumpPath := flag.String("dump", "", "write a YAML dump of the universe to this file and exit")
	backupPath := flag.String("backup", "", "write a backup copy of the database to this file and exit")
	flag.Parse()

	store, err := galwar.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		return
	}
	defer store.Close()

	if *backupPath != "" {
		if err := store.Backup(*backupPath); err != nil {
			fmt.Printf("Error backing up database: %v\n", err)
			return
		}
		fmt.Printf("Backed up %s to %s\n", *dbPath, *backupPath)
		return
	}

	u := galwar.NewUniverse()
	loaded, err := store.LoadUniverse(u)
	if err != nil {
		fmt.Printf("Error loading universe from database: %v\n", err)
		return
	}

	if !loaded {
		if _, err := os.Stat(*yamlPath); err == nil {
			// one-time migration from the legacy YAML store
			u.SetFilename(*yamlPath)
			if err := u.Load(); err != nil {
				fmt.Printf("Error migrating %s: %v\n", *yamlPath, err)
				return
			}
			fmt.Printf("Migrated %s into %s; the database is now authoritative.\n", *yamlPath, *dbPath)
		} else {
			numsec := u.ConfigInt("numsec", 2000)
			u.Generate(numsec)
			fmt.Printf("Generated a new %d-sector universe in %s.\n", numsec, *dbPath)
		}
		u.SeedDefaultConfig()
		if err := store.SaveUniverse(u.Snapshot()); err != nil {
			fmt.Printf("Error saving universe to database: %v\n", err)
			return
		}
	}

	if *dumpPath != "" {
		u.SetFilename(*dumpPath)
		if err := u.Save(); err != nil {
			fmt.Printf("Error writing dump: %v\n", err)
			return
		}
		fmt.Printf("Dumped universe to %s\n", *dumpPath)
		return
	}

	u.Start()

	u.Do(u.ApplyModerationExtras)

	persister := galwar.NewPersister(u, store)
	persister.Start()

	maint := galwar.NewMaintenanceDaemon(u)
	maint.Start()

	var player *galwar.Player
	u.Do(func() {
		player = u.Players.GetByEmail("theplayer@gmail.com")
		if player == nil {
			player = u.NewPlayer("Defs Sacre", "theplayer@gmail.com")
		}
	})

	term := consoleui.NewStdioTerminal()
	ui := consoleui.NewConsoleUI(u, player, term)
	done := make(chan struct{})
	go func() {
		if consoleui.SessionStart(u, term, player) {
			ui.Run()
		}
		close(done)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	select {
	case <-done:
	case <-sig:
		fmt.Printf("\nInterrupted - saving universe...\n")
	}

	maint.Stop()
	persister.Stop() // final flush
}
