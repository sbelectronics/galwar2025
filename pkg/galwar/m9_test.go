package galwar

import (
	"strings"
	"testing"
	"time"
)

func devicesUniverse(t *testing.T) (*UniverseType, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Devicer", "d@example.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return u, p
}

func TestAmazingDevicesPort(t *testing.T) {
	u, _ := devicesUniverse(t)

	var dev *Port
	for _, p := range u.Ports.Ports {
		if p.Goods == AmazingDevices {
			dev = p
		}
	}
	if dev == nil {
		t.Fatalf("no Amazing Devices port was generated")
	}
	if dev.Sector < 2 || dev.Sector > 10 {
		t.Errorf("device port in sector %d; expected a low (fed) sector", dev.Sector)
	}
	for _, name := range []string{CLOAK, ANTICLOAK, PULSARTUBE} {
		cm := dev.GetCommodity(name)
		if cm == nil || !cm.Sell {
			t.Errorf("Amazing Devices does not sell %s", name)
		}
	}
	// the devices are NOT at Sol
	sol := u.Ports.Ports[0]
	if sol.Goods != Sol {
		t.Fatalf("port 0 not Sol")
	}
	for _, name := range []string{CLOAK, ANTICLOAK, PULSARTUBE} {
		if sol.GetCommodity(name) != nil {
			t.Errorf("Sol should not sell %s", name)
		}
	}
	// ...but the old specials still are
	for _, name := range []string{PLASMA, PULSAR, EMWARP} {
		if cm := sol.GetCommodity(name); cm == nil || !cm.Sell {
			t.Errorf("Sol no longer sells %s", name)
		}
	}
	if dev.IsService() != true || sol.IsService() != true {
		t.Errorf("service ports not flagged as service")
	}
}

func TestCloakHidesShip(t *testing.T) {
	u, _ := devicesUniverse(t)
	now := time.Now()
	viewer, _ := u.RegisterPlayer("Viewer", "v@example.com", "")
	ghost, _ := u.RegisterPlayer("Ghost", "g@example.com", "")
	// established players (never-moved ships are hidden regardless of cloak;
	// this test is about the cloak)
	viewer.EverMoved = true
	ghost.EverMoved = true
	viewer.MoveTo(50)
	ghost.MoveTo(50)
	u.TouchLastSeen(viewer, now.Unix())
	u.TouchLastSeen(ghost, now.Unix())

	// visible before cloaking
	vis := u.GetVisibleObjectsInSector(50, TYPE_PLAYER, viewer, now)
	if !contains(vis, ghost) {
		t.Fatalf("ghost not visible before cloaking")
	}

	// cloak hides it
	ghost.SetQuantity(CLOAK, 1)
	vis = u.GetVisibleObjectsInSector(50, TYPE_PLAYER, viewer, now)
	if contains(vis, ghost) {
		t.Errorf("cloaked ship still visible")
	}
	// the cloaked player still sees themselves
	self := u.GetVisibleObjectsInSector(50, TYPE_PLAYER, ghost, now)
	if !contains(self, ghost) {
		t.Errorf("cloaked player can't see themselves")
	}

	// an anti-cloak reveals it
	viewer.SetQuantity(ANTICLOAK, 1)
	vis = u.GetVisibleObjectsInSector(50, TYPE_PLAYER, viewer, now)
	if !contains(vis, ghost) {
		t.Errorf("anti-cloak did not reveal the cloaked ship")
	}
}

func TestCloakBlocksAttack(t *testing.T) {
	u, _ := devicesUniverse(t)
	attacker, _ := u.RegisterPlayer("Hunter", "h@example.com", "")
	target, _ := u.RegisterPlayer("Prey", "p@example.com", "")
	attacker.MoveTo(50)
	target.MoveTo(50)
	attacker.SetQuantity(FIGHTERS, 1000)
	target.SetQuantity(CLOAK, 1)

	if _, err := u.AttackPlayer(attacker, target.Id, 100); err == nil {
		t.Errorf("attacked a cloaked target without anti-cloak")
	}
	attacker.SetQuantity(ANTICLOAK, 1)
	if _, err := u.AttackPlayer(attacker, target.Id, 100); err != nil {
		t.Errorf("anti-cloak holder could not attack the cloaked target: %v", err)
	}
}

func TestPulsarTube(t *testing.T) {
	u, attacker := devicesUniverse(t)
	// a defender-owned planet at sector 50
	defender, _ := u.RegisterPlayer("Owner", "o@example.com", "")
	defender.MoveTo(50)
	defender.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(defender, 50, "Target"); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	planet := u.Planets.Planets[0]
	planet.GetCommodity(ORE).Prod = 2000
	planet.GetCommodity(ORGANICS).Prod = 2000
	planet.GetCommodity(EQUIPMENT).Prod = 2000

	attacker.MoveTo(50)
	attacker.SetQuantity(PULSAR, 10)

	// no tube -> refused
	if _, err := u.UsePulsarTube(attacker, 1); err == nil {
		t.Errorf("orbital strike without a Pulsar Tube allowed")
	}

	attacker.SetQuantity(PULSARTUBE, 1)
	turns := attacker.GetQuantity(TURNS)
	if _, err := u.UsePulsarTube(attacker, 1); err != nil {
		t.Fatalf("tube launch: %v", err)
	}
	// 500 per bomb (not 1000)
	if got := planet.GetCommodity(ORE).Prod; got != 1500 {
		t.Errorf("ore prod after 1 tube bomb = %d; want 1500 (500/bomb)", got)
	}
	if attacker.GetQuantity(PULSAR) != 9 {
		t.Errorf("bomb not consumed")
	}
	if attacker.GetQuantity(PULSARTUBE) != 1 {
		t.Errorf("tube consumed (it should be reusable)")
	}
	if attacker.GetQuantity(TURNS) != turns-1 {
		t.Errorf("tube launch did not cost a turn")
	}
	// the owner is notified of the orbital strike
	news := u.TakeNews(defender.Id)
	if !strings.Contains(strings.Join(news, "\n"), "pulsar-bombed from orbit") {
		t.Errorf("owner not notified of orbital strike: %v", news)
	}
}

func TestUpgradeAddsDevicePort(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	// simulate a pre-device-port universe: remove the Amazing Devices port
	kept := u.Ports.Ports[:0]
	for _, p := range u.Ports.Ports {
		if p.Goods != AmazingDevices {
			kept = append(kept, p)
		}
	}
	u.Ports.Ports = kept
	for _, p := range u.Ports.Ports {
		if p.Goods == AmazingDevices {
			t.Fatalf("device port not removed")
		}
	}

	u.upgrade()

	found := false
	for _, p := range u.Ports.Ports {
		if p.Goods == AmazingDevices {
			found = true
			for _, name := range []string{CLOAK, ANTICLOAK, PULSARTUBE} {
				if p.GetCommodity(name) == nil {
					t.Errorf("upgraded device port missing %s", name)
				}
			}
		}
	}
	if !found {
		t.Errorf("upgrade did not create an Amazing Devices port")
	}
}

func contains(objs []ObjectInterface, want ObjectInterface) bool {
	for _, o := range objs {
		if o == want {
			return true
		}
	}
	return false
}
