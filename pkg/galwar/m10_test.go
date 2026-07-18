package galwar

import (
	"strings"
	"testing"
)

// bgUniverse makes a player at sector 50 with a known two-hop route 50->59->60
// so battle-group tests don't depend on the random generator.
func bgUniverse(t *testing.T) (*UniverseType, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Commander", "c@example.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	p.MoveTo(50)
	// carve a deterministic path 50 -> 59 -> 60
	clearWarps(u, 50)
	clearWarps(u, 59)
	clearWarps(u, 60)
	u.Sectors[50].AddWarp(59)
	u.Sectors[59].AddWarp(50)
	u.Sectors[59].AddWarp(60)
	u.Sectors[60].AddWarp(59)
	return u, p
}

func TestBattleGroupValidation(t *testing.T) {
	u, p := bgUniverse(t)
	p.SetQuantity(FIGHTERS, 1000)

	if _, err := u.LaunchBattleGroup(p, 99999, 100); err == nil {
		t.Errorf("launched to a nonexistent sector")
	}
	if _, err := u.LaunchBattleGroup(p, 50, 100); err == nil {
		t.Errorf("launched to own sector")
	}
	if _, err := u.LaunchBattleGroup(p, 60, 0); err == nil {
		t.Errorf("launched with zero ships")
	}
	if _, err := u.LaunchBattleGroup(p, 60, 99999); err == nil {
		t.Errorf("launched with more ships than owned")
	}
	// a damaged Battle-Group Computer blocks the launch
	p.DamageSystem(SysBGComputer, 5)
	if _, err := u.LaunchBattleGroup(p, 60, 100); err == nil {
		t.Errorf("launched with a damaged Battle-Group Computer")
	}
}

func TestBattleGroupSurvivorsReturn(t *testing.T) {
	u, p := bgUniverse(t)
	p.SetQuantity(FIGHTERS, 500)
	turns := p.GetQuantity(TURNS)

	// clear run to an undefended sector: all ships come home, 1 turn spent
	if _, err := u.LaunchBattleGroup(p, 60, 300); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if got := p.GetQuantity(FIGHTERS); got != 500 {
		t.Errorf("fighters after unopposed raid = %d; want 500 (all survivors return)", got)
	}
	if p.GetQuantity(TURNS) != turns-1 {
		t.Errorf("launch did not cost a turn")
	}
}

func TestBattleGroupBreaksGarrison(t *testing.T) {
	u, p := bgUniverse(t)
	// an enemy garrison of 50 fighters at the destination (its owner sits
	// elsewhere, so the fleet fights only the garrison, not the player)
	enemy, _ := u.RegisterPlayer("Defender", "d@example.com", "")
	enemy.SetQuantity(FIGHTERS, 200)
	if err := u.AdjustBattlegroup(enemy, 60, FIGHTERS, 50); err != nil {
		t.Fatalf("garrison: %v", err)
	}

	p.SetQuantity(FIGHTERS, 10000)
	report, err := u.LaunchBattleGroup(p, 60, 5000)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	// the garrison is wiped
	bg, _ := u.GetBattlegroup(enemy, 60, false)
	if bg != nil && bg.GetQuantity(FIGHTERS) > 0 {
		t.Errorf("garrison survived a 5000-ship battle group")
	}
	// the fleet took some losses but most returned
	got := p.GetQuantity(FIGHTERS)
	if got >= 10000 {
		t.Errorf("battle group took no losses breaking a 50-fighter garrison")
	}
	if got <= 5000 {
		t.Errorf("battle group lost more than it committed: %d", got)
	}
	if !strings.Contains(strings.Join(report, "\n"), "destroyed 50 fighters") {
		t.Errorf("garrison-kill not reported: %v", report)
	}
	// the garrison owner has news
	if news := u.TakeNews(enemy.Id); len(news) == 0 {
		t.Errorf("garrison owner got no news of the attack")
	}
}

func TestBattleGroupKillsPlayerRemotely(t *testing.T) {
	u, p := bgUniverse(t)
	victim, _ := u.RegisterPlayer("Victim", "v@example.com", "")
	victim.EverMoved = true // an established player (never-moved ships are hidden from recon)
	victim.MoveTo(60)       // sitting at the far end of the route
	victim.SetQuantity(FIGHTERS, 100)
	victim.SetQuantity(EMWARP, 0)

	p.SetQuantity(FIGHTERS, 100000)
	if _, err := u.LaunchBattleGroup(p, 60, 100000); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if !victim.IsDead() {
		t.Errorf("battle group did not kill the outmatched player at the destination")
	}
	news := u.TakeNews(victim.Id)
	if !strings.Contains(strings.Join(news, "\n"), "killed by Commander's battle group") {
		t.Errorf("victim not notified of the remote kill: %v", news)
	}
}

func TestScoutDoesNotAttackPlayers(t *testing.T) {
	u, p := bgUniverse(t)
	bystander, _ := u.RegisterPlayer("Bystander", "b@example.com", "")
	bystander.EverMoved = true // established: visible to the scout's recon
	bystander.MoveTo(60)
	bystander.SetQuantity(FIGHTERS, 100)

	p.SetQuantity(FIGHTERS, 50)
	report, err := u.LaunchBattleGroup(p, 60, 1) // a 1-ship scout
	if err != nil {
		t.Fatalf("scout: %v", err)
	}
	if bystander.GetQuantity(FIGHTERS) != 100 {
		t.Errorf("scout attacked a player (should only recon): %d", bystander.GetQuantity(FIGHTERS))
	}
	// but it recons: the bystander is reported
	if !strings.Contains(strings.Join(report, "\n"), "Bystander with 100 fighters") {
		t.Errorf("scout did not recon the player: %v", report)
	}
	// scout returns
	if p.GetQuantity(FIGHTERS) != 50 {
		t.Errorf("scout did not return: %d", p.GetQuantity(FIGHTERS))
	}
}

func TestBattleGroupMinesIgnoreSmallFleet(t *testing.T) {
	u, p := bgUniverse(t)
	// minefield at the destination, its owner elsewhere
	miner, _ := u.RegisterPlayer("Miner", "m@example.com", "")
	miner.SetQuantity(MINES, 10)
	if err := u.AdjustBattlegroup(miner, 60, MINES, 10); err != nil {
		t.Fatalf("minefield: %v", err)
	}

	// a 100-ship fleet is under the 150 threshold - mines don't trigger
	p.SetQuantity(FIGHTERS, 100)
	report, err := u.LaunchBattleGroup(p, 60, 100)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	if p.GetQuantity(FIGHTERS) != 100 {
		t.Errorf("mines hit a sub-150 fleet: %d survivors", p.GetQuantity(FIGHTERS))
	}
	bg, _ := u.GetBattlegroup(miner, 60, false)
	if bg == nil || bg.GetQuantity(MINES) != 10 {
		t.Errorf("mines were expended against a sub-150 fleet")
	}
	if !strings.Contains(strings.Join(report, "\n"), "did not go off") {
		t.Errorf("no 'did not go off' message: %v", report)
	}

	// a 200-ship fleet triggers them
	p.SetQuantity(FIGHTERS, 200)
	if _, err := u.LaunchBattleGroup(p, 60, 200); err != nil {
		t.Fatalf("launch2: %v", err)
	}
	if p.GetQuantity(FIGHTERS) >= 200 {
		t.Errorf("mines did not hit a 200-ship fleet")
	}
}
