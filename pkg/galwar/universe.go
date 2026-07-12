package galwar

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// UniverseType owns all game state. There are deliberately no package-level
// globals: create a universe with NewUniverse, generate or Load it, then
// Start the actor loop and route all access through Do/DoErr (see actor.go).
type UniverseType struct {
	Ports        PortList
	Players      PlayerList
	Battlegroups BattlegroupList
	Planets      PlanetList
	Sectors      []Sector
	Config       map[string]string
	News         []*NewsItem

	filename    string
	tasks       chan *task
	dirtyNotify func()
}

// SetDirtyNotifier registers the persistence hook invoked by MarkDirty.
func (u *UniverseType) SetDirtyNotifier(fn func()) {
	u.dirtyNotify = fn
}

// MarkDirty records that the universe has changed and should be persisted.
// Engine commands call this after a successful mutation. Without a
// registered notifier (tests, generation) it is a no-op.
func (u *UniverseType) MarkDirty() {
	if u.dirtyNotify != nil {
		u.dirtyNotify()
	}
}

func NewUniverse() *UniverseType {
	return &UniverseType{}
}

func (u *UniverseType) SetFilename(filename string) {
	u.filename = filename
}

func (u *UniverseType) FileExist() bool {
	if _, err := os.Stat(u.filename); err == nil {
		return true
	}
	return false
}

// Save writes the universe atomically: marshal to a temp file in the same
// directory, then rename over the target, so a crash mid-write can never
// corrupt the only copy of the world.
func (u *UniverseType) Save() error {
	data, err := yaml.Marshal(u)
	if err != nil {
		return err
	}

	dir := filepath.Dir(u.filename)
	tmp, err := os.CreateTemp(dir, ".universe-*.yaml.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	// Flush to disk before the rename, so a crash immediately afterward
	// can't leave an empty or truncated file behind the new name.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	// CreateTemp makes the file 0600; restore the 0644 the universe file has
	// always had so group/other readability doesn't silently change.
	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, u.filename); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func (u *UniverseType) Load() error {
	data, err := os.ReadFile(u.filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, u)
	if err != nil {
		return err
	}

	u.wire()
	u.upgrade()
	return u.validate()
}

// upgrade brings data saved by older builds up to the current feature set.
// Idempotent; runs on every load.
func (u *UniverseType) upgrade() {
	// players saved before the turn economy get today's allowance once
	// (missing commodity entries are added at zero; Turns is special-cased
	// because a zero grant would strand them)
	for _, p := range u.Players.Players {
		p.ensureSystems()
		for _, tg := range TradeGoods {
			if p.GetCommodity(tg.Name) != nil {
				continue
			}
			quantity := 0
			if tg.Name == TURNS {
				quantity = u.ConfigInt("turns_per_day", 250)
			}
			p.Inventory = append(p.Inventory, &Commodity{Name: tg.Name, Quantity: quantity})
		}
	}

	// Sol ports saved before a good existed learn to sell it
	for _, port := range u.Ports.Ports {
		if port.Goods != Sol {
			continue
		}
		for _, tg := range TradeGoods {
			if !tg.SellAtSol || port.GetCommodity(tg.Name) != nil {
				continue
			}
			cm := tg.Commodity
			cm.Sell = true
			port.Inventory = append(port.Inventory, &cm)
		}
	}

	// planets saved before production existed get the genesis seed rates
	for _, planet := range u.Planets.Planets {
		for _, name := range []string{ORE, ORGANICS, EQUIPMENT} {
			if c := planet.GetCommodity(name); c != nil && c.Prod == 0 {
				c.Prod = FindCommodityDef(name).PlanetProdStarting
			}
		}
	}
}

// wire sets the unexported back-references that objects need to resolve
// things outside themselves (e.g. a planet resolving its owner's name).
// Called after Load; constructors set the same references at creation time.
func (u *UniverseType) wire() {
	for _, p := range u.Planets.Planets {
		p.universe = u
	}
	for _, b := range u.Battlegroups.Battlegroups {
		b.universe = u
	}
}

// validate sanity-checks a loaded universe so bad data fails at load time
// with a useful message instead of panicking mid-game.
func (u *UniverseType) validate() error {
	if len(u.Sectors) < 2 {
		return fmt.Errorf("universe has no sectors")
	}
	for i := range u.Sectors {
		if u.Sectors[i].Number != i {
			return fmt.Errorf("sector at index %d has number %d; sector numbers must match their index", i, u.Sectors[i].Number)
		}
	}
	maxSec := len(u.Sectors) - 1

	for i := range u.Sectors {
		for _, w := range u.Sectors[i].Warps {
			if w < 1 || w > maxSec {
				return fmt.Errorf("sector %d has a warp to invalid sector %d", i, w)
			}
		}
	}

	checkInventory := func(kind, name string, inv []*Commodity) error {
		for _, c := range inv {
			if c == nil {
				return fmt.Errorf("%s %q has a nil commodity entry", kind, name)
			}
			if FindCommodityDef(c.Name) == nil {
				return fmt.Errorf("%s %q has unknown commodity %q", kind, name, c.Name)
			}
		}
		return nil
	}
	checkSector := func(kind, name string, sector int) error {
		if sector < 1 || sector > maxSec {
			return fmt.Errorf("%s %q is in invalid sector %d", kind, name, sector)
		}
		return nil
	}

	for _, p := range u.Ports.Ports {
		if err := checkSector("port", p.Name, p.Sector); err != nil {
			return err
		}
		if err := checkInventory("port", p.Name, p.Inventory); err != nil {
			return err
		}
	}
	for _, p := range u.Planets.Planets {
		if err := checkSector("planet", p.Name, p.Sector); err != nil {
			return err
		}
		if err := checkInventory("planet", p.Name, p.Inventory); err != nil {
			return err
		}
	}
	for _, b := range u.Battlegroups.Battlegroups {
		if err := checkSector("battlegroup", string(b.Owner), b.Sector); err != nil {
			return err
		}
		if err := checkInventory("battlegroup", string(b.Owner), b.Inventory); err != nil {
			return err
		}
	}
	for _, p := range u.Players.Players {
		if err := checkInventory("player", p.Name, p.Inventory); err != nil {
			return err
		}
		// Be forgiving with players: strand them at Sol rather than refusing
		// to load the whole universe. Sector 0 is legitimate off-map parking
		// for dead ships and NPC faction records.
		if p.Sector < 0 || p.Sector > maxSec {
			log.Printf("player %q was in invalid sector %d; moved to sector 1", p.Name, p.Sector)
			p.Sector = 1
		}
	}
	return nil
}

func (u *UniverseType) GetObjectsInSector(sector int, kind string) []ObjectInterface {
	objects := []ObjectInterface{}

	// Be deterministic about the order we display things
	objLists := []ObjectListInterface{&u.Ports, &u.Players, &u.Battlegroups, &u.Planets}
	for _, objList := range objLists {
		objItems := objList.GetObjectsInSector(sector)
		for _, obj := range objItems {
			if (kind == "") || (obj.GetType() == kind) {
				objects = append(objects, obj)
			}
		}
	}

	return objects
}

// Fun Fact: In 1986, My friend Greg wrote the Galwar Autopilot for me.
// In 2025, I just hit CTRL-I and asked Copilot to implement single source
// shortest paths. The following code is verbatim from the AI:
// (2026 update: moved from Sector to UniverseType when the global Sectors
// slice was eliminated; the algorithm is unchanged.)

func (u *UniverseType) ShortestPathTo(from int, target int) []int {
	if from < 1 || from >= len(u.Sectors) || target < 1 || target >= len(u.Sectors) {
		return nil
	}

	visited := make(map[int]bool)
	distances := make(map[int]int)
	previous := make(map[int]int)

	for _, sector := range u.Sectors {
		distances[sector.Number] = int(^uint(0) >> 1) // Set to max int
	}
	distances[from] = 0

	current := from
	for current != target {
		visited[current] = true

		// Update distances for neighbors
		for _, neighbor := range u.Sectors[current].Warps {
			if visited[neighbor] {
				continue
			}
			newDist := distances[current] + 1
			if newDist < distances[neighbor] {
				distances[neighbor] = newDist
				previous[neighbor] = current
			}
		}

		// Find the next unvisited sector with the smallest distance
		minDist := int(^uint(0) >> 1)
		next := -1
		for _, sector := range u.Sectors {
			if !visited[sector.Number] && distances[sector.Number] < minDist {
				minDist = distances[sector.Number]
				next = sector.Number
			}
		}

		if next == -1 {
			return nil // No path exists
		}
		current = next
	}

	// Reconstruct the path
	path := []int{}
	for at := target; at != from; at = previous[at] {
		path = append([]int{at}, path...)
	}
	path = append([]int{from}, path...)
	return path
}
