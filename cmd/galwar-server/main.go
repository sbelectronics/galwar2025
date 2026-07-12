package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
	"github.com/sbelectronics/galwar/pkg/telnetserver"
	"github.com/sbelectronics/galwar/pkg/webserver"
)

func main() {
	dbPath := flag.String("db", "galwar.db", "path to the game database")
	yamlPath := flag.String("yaml", "universe.yaml", "legacy YAML universe (migrated if the database is empty)")
	listen := flag.String("listen", ":8080", "HTTP listen address")
	telnetAddr := flag.String("telnet", ":2323", "telnet listen address (empty to disable)")
	baseURL := flag.String("base-url", "http://localhost:8080", "externally visible base URL (for OAuth redirect and cookies)")
	devAuth := flag.Bool("devauth", false, "enable /auth/dev?user=email login backdoor (DEVELOPMENT ONLY)")
	admin := flag.String("admin", "", "grant sysop rights to this email (added to the admins config on startup)")
	flag.Parse()

	store, err := galwar.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer store.Close()

	u := galwar.NewUniverse()
	loaded, err := store.LoadUniverse(u)
	if err != nil {
		log.Fatalf("loading universe: %v", err)
	}
	if !loaded {
		if _, err := os.Stat(*yamlPath); err == nil {
			u.SetFilename(*yamlPath)
			if err := u.Load(); err != nil {
				log.Fatalf("migrating %s: %v", *yamlPath, err)
			}
			log.Printf("migrated %s into %s; the database is now authoritative", *yamlPath, *dbPath)
		} else {
			numsec := u.ConfigInt("numsec", 2000)
			u.Generate(numsec)
			log.Printf("generated a new %d-sector universe in %s", numsec, *dbPath)
		}
		u.SeedDefaultConfig()
		if err := store.SaveUniverse(u.Snapshot()); err != nil {
			log.Fatalf("saving universe: %v", err)
		}
	}

	u.Start()

	if *admin != "" {
		u.Do(func() {
			existing := u.ConfigString("admins", "")
			if existing == "" {
				u.SetConfig("admins", *admin)
			} else {
				u.SetConfig("admins", existing+","+*admin)
			}
		})
		log.Printf("granted sysop rights to %s", *admin)
	}

	persister := galwar.NewPersister(u, store)
	persister.Start()

	maint := galwar.NewMaintenanceDaemon(u)
	maint.Start()

	web, err := webserver.New(context.Background(), webserver.Config{
		Universe:           u,
		Store:              store,
		BaseURL:            *baseURL,
		GoogleClientID:     os.Getenv("GALWAR_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GALWAR_GOOGLE_CLIENT_SECRET"),
		DevAuth:            *devAuth,
	})
	if err != nil {
		log.Fatalf("initializing web server: %v", err)
	}
	if web != nil && os.Getenv("GALWAR_GOOGLE_CLIENT_ID") == "" && !*devAuth {
		log.Printf("WARNING: no GALWAR_GOOGLE_CLIENT_ID and -devauth off: nobody can log in on the web")
	}

	httpServer := &http.Server{Addr: *listen, Handler: web.Handler()}
	go func() {
		log.Printf("web portal listening on %s (base URL %s)", *listen, *baseURL)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	var telnet *telnetserver.Server
	if *telnetAddr != "" {
		telnet = telnetserver.New(u)
		if err := telnet.Start(*telnetAddr); err != nil {
			log.Fatalf("telnet server: %v", err)
		}
		log.Printf("telnet gateway listening on %s", *telnetAddr)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	fmt.Println("\nshutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// close live sessions first: they send a farewell and let their state
	// settle, and it keeps Shutdown from waiting on any still-tracked request
	web.CloseAllSessions()
	httpServer.Shutdown(ctx)
	if telnet != nil {
		telnet.Stop()
	}
	maint.Stop()
	persister.Stop() // final flush
}
