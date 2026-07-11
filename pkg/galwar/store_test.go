package galwar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

// universesEqual compares two universes by their YAML serialization, which
// covers every exported field and ignores the unexported wiring.
func universesEqual(t *testing.T, a, b *UniverseType) {
	t.Helper()
	ya, err := yaml.Marshal(a)
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	yb, err := yaml.Marshal(b)
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	if string(ya) != string(yb) {
		t.Errorf("universes differ after roundtrip (yaml lengths %d vs %d)", len(ya), len(yb))
	}
}

func makeTestUniverse(t *testing.T) *UniverseType {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	player := u.NewPlayer("Test", "t@example.com")
	player.MoveTo(50)
	player.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(player, 50, "Testworld"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	if err := u.AdjustBattlegroup(player, 60, FIGHTERS, 25); err != nil {
		t.Fatalf("battlegroup: %v", err)
	}
	return u
}

func TestStoreRoundtrip(t *testing.T) {
	u := makeTestUniverse(t)

	dbPath := filepath.Join(t.TempDir(), "galwar.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.SaveUniverse(u.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	u2 := NewUniverse()
	loaded, err := store.LoadUniverse(u2)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded {
		t.Fatalf("load reported empty store")
	}

	universesEqual(t, u, u2)

	// wiring: the loaded planet resolves its owner through the new universe
	if got := u2.Planets.Planets[0].GetOwnerPlayer().GetName(); got != "Test" {
		t.Errorf("planet owner = %q; want Test", got)
	}
	// config survived
	if got := u2.ConfigInt("starting_credits", 0); got != 35000 {
		t.Errorf("config starting_credits = %d; want 35000", got)
	}

	// saving again over existing data must fully replace, not accumulate
	if err := store.SaveUniverse(u2.Snapshot()); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	u3 := NewUniverse()
	if _, err := store.LoadUniverse(u3); err != nil {
		t.Fatalf("re-load: %v", err)
	}
	universesEqual(t, u, u3)
}

func TestLoadUniverseEmpty(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "galwar.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	u := NewUniverse()
	loaded, err := store.LoadUniverse(u)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded {
		t.Errorf("empty store reported as loaded")
	}
}

func TestStoreBackup(t *testing.T) {
	u := makeTestUniverse(t)

	dir := t.TempDir()
	store, err := OpenStore(filepath.Join(dir, "galwar.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.SaveUniverse(u.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	backupPath := filepath.Join(dir, "backup.db")
	if err := store.Backup(backupPath); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	// the backup must be a loadable database
	store2, err := OpenStore(backupPath)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer store2.Close()
	u2 := NewUniverse()
	loaded, err := store2.LoadUniverse(u2)
	if err != nil || !loaded {
		t.Fatalf("load from backup: loaded=%v err=%v", loaded, err)
	}
	universesEqual(t, u, u2)
}

func TestPersisterWriteBehind(t *testing.T) {
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()
	u.Start()

	store, err := OpenStore(filepath.Join(t.TempDir(), "galwar.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	p := NewPersister(u, store)
	p.Interval = 10 * time.Millisecond
	p.Start()

	// a command that marks dirty should reach the database without any
	// explicit save
	var player *Player
	u.Do(func() {
		player = u.NewPlayer("Behind", "wb@example.com")
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		u2 := NewUniverse()
		loaded, err := store.LoadUniverse(u2)
		if err == nil && loaded && u2.Players.GetByEmail("wb@example.com") != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("write-behind flush never reached the database (loaded=%v err=%v)", loaded, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Stop must flush the very latest state even without waiting
	u.Do(func() {
		player.Money = 12345
	})
	u.MarkDirty()
	p.Stop()

	u3 := NewUniverse()
	if _, err := store.LoadUniverse(u3); err != nil {
		t.Fatalf("load after stop: %v", err)
	}
	if got := u3.Players.GetByEmail("wb@example.com").Money; got != 12345 {
		t.Errorf("final flush missed last mutation: money = %d; want 12345", got)
	}
}

func TestConfigAccessors(t *testing.T) {
	u := NewUniverse()
	if got := u.ConfigInt("numsec", 2000); got != 2000 {
		t.Errorf("default not returned on nil config: %d", got)
	}
	u.SetConfig("numsec", "500")
	if got := u.ConfigInt("numsec", 2000); got != 500 {
		t.Errorf("ConfigInt = %d; want 500", got)
	}
	u.SetConfig("numsec", "junk")
	if got := u.ConfigInt("numsec", 2000); got != 2000 {
		t.Errorf("non-numeric value should fall back to default; got %d", got)
	}
	if got := u.ConfigString("missing", "x"); got != "x" {
		t.Errorf("ConfigString default = %q; want x", got)
	}
}
