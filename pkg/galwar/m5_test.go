package galwar

import (
	"strings"
	"testing"
	"time"
)

func combatUniverse(t *testing.T) (*UniverseType, *Player, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	attacker, err := u.RegisterPlayer("Attacker", "a@example.com", "sub-a")
	if err != nil {
		t.Fatalf("register attacker: %v", err)
	}
	defender, err := u.RegisterPlayer("Defender", "d@example.com", "sub-d")
	if err != nil {
		t.Fatalf("register defender: %v", err)
	}
	attacker.MoveTo(50)
	defender.MoveTo(50)
	return u, attacker, defender
}

func TestAttrition(t *testing.T) {
	for i := 0; i < 20; i++ {
		aLoss, dLoss := attrition(100, 100)
		if aLoss > 100 || dLoss > 100 || aLoss < 0 || dLoss < 0 {
			t.Fatalf("losses out of bounds: %d, %d", aLoss, dLoss)
		}
		if aLoss < 100 && dLoss < 100 {
			t.Fatalf("attrition ended with neither side exhausted: %d, %d", aLoss, dLoss)
		}
	}
	// committed pool is the cap
	aLoss, _ := attrition(5, 1000000)
	if aLoss > 5 {
		t.Errorf("attacker lost %d of a 5-fighter commitment", aLoss)
	}
}

func TestAttackValidation(t *testing.T) {
	u, attacker, defender := combatUniverse(t)

	// federation space is a no-combat zone (deviation: the original allowed
	// it with fines; we refuse outright)
	attacker.MoveTo(5)
	defender.MoveTo(5)
	if _, err := u.AttackPlayer(attacker, defender.Id, 100); err == nil {
		t.Errorf("attack allowed in federation space")
	}
	attacker.MoveTo(50)
	defender.MoveTo(50)

	if _, err := u.AttackPlayer(attacker, attacker.Id, 100); err == nil {
		t.Errorf("self-attack allowed")
	}
	if _, err := u.AttackPlayer(attacker, defender.Id, 0); err == nil {
		t.Errorf("zero-fighter attack allowed")
	}
	if _, err := u.AttackPlayer(attacker, defender.Id, 999999); err == nil {
		t.Errorf("attack with more fighters than owned allowed")
	}
	defender.MoveTo(51)
	if _, err := u.AttackPlayer(attacker, defender.Id, 100); err == nil {
		t.Errorf("attack across sectors allowed")
	}
	defender.MoveTo(50)

	// attacks cost a turn (deviation per PLAN 5.3)
	turns := attacker.GetQuantity(TURNS)
	if _, err := u.AttackPlayer(attacker, defender.Id, 10); err != nil {
		t.Fatalf("attack: %v", err)
	}
	if got := attacker.GetQuantity(TURNS); got != turns-1 {
		t.Errorf("attack cost %d turns; want 1", turns-got)
	}
}

func TestAttackKillFlow(t *testing.T) {
	u, attacker, defender := combatUniverse(t)

	// give the defender standing assets to forfeit (garrison first, while
	// they still have fighters to leave behind)
	if err := u.AdjustBattlegroup(defender, 60, FIGHTERS, 50); err != nil {
		t.Fatalf("defender bg: %v", err)
	}

	// then stack the odds so the outcome is certain
	attacker.SetQuantity(FIGHTERS, 100000)
	defender.SetQuantity(FIGHTERS, 5)
	defender.SetQuantity(MINES, 0)
	defender.SetQuantity(GENESIS, 1)
	if err := u.UseGenesisDevice(defender, 50, "Doomed"); err != nil {
		t.Fatalf("defender planet: %v", err)
	}

	holdsBefore := attacker.GetQuantity(HOLDS)
	report, err := u.AttackPlayer(attacker, defender.Id, 100000)
	if err != nil {
		t.Fatalf("attack: %v", err)
	}

	if !defender.IsDead() {
		t.Fatalf("defender survived a 100000 vs 5 attack; report: %v", report)
	}
	if defender.Sector != 0 {
		t.Errorf("dead defender in sector %d; want 0 (off-map)", defender.Sector)
	}
	if defender.TimesDied != 1 {
		t.Errorf("TimesDied = %d; want 1", defender.TimesDied)
	}

	// salvage: 30-99% of the defender's 25 holds
	salvage := attacker.GetQuantity(HOLDS) - holdsBefore
	if salvage < 7 || salvage > 24 {
		t.Errorf("salvage = %d holds; want 7..24", salvage)
	}

	// the defender's battlegroup now belongs to the Renegades
	renegades := u.Players.GetBySub("npc:renegades")
	if renegades == nil {
		t.Fatalf("renegades NPC not created")
	}
	bg, _ := u.GetBattlegroup(renegades, 60, false)
	if bg == nil || bg.GetQuantity(FIGHTERS) != 50 {
		t.Errorf("defender's battlegroup did not pass to the renegades")
	}

	// the planet revolted to the Cabal or the Federation
	planet := u.Planets.Planets[0]
	heir := u.Players.GetById(planet.Owner)
	if heir == nil || !heir.IsNPC() {
		t.Errorf("planet owner after death = %v; want an NPC faction", heir)
	}

	// the defender has news waiting
	news := u.TakeNews(defender.Id)
	joined := strings.Join(news, "\n")
	if !strings.Contains(joined, "KILLED by Attacker") {
		t.Errorf("defender news missing kill notice: %q", joined)
	}
	if !strings.Contains(joined, "revolted") {
		t.Errorf("defender news missing planet revolt: %q", joined)
	}
}

func TestCarriedMinesAvengeTheVictim(t *testing.T) {
	u, attacker, defender := combatUniverse(t)

	// the defender is helpless but booby-trapped: 50 carried mines at
	// 300-499 fighters each will shred a 400-fighter attacker
	attacker.SetQuantity(FIGHTERS, 400)
	defender.SetQuantity(FIGHTERS, 1)
	defender.SetQuantity(MINES, 50)

	report, err := u.AttackPlayer(attacker, defender.Id, 400)
	if err != nil {
		t.Fatalf("attack: %v", err)
	}
	if !defender.IsDead() {
		// 400 vs 1 can technically lose the exchange; then no mines fire
		t.Skipf("defender survived the 400 vs 1 exchange (rare); report: %v", report)
	}
	if !attacker.IsDead() {
		t.Errorf("attacker survived 50 mine detonations; report: %v", report)
	}
}

func TestContestedEntry(t *testing.T) {
	u, mover, owner := combatUniverse(t)

	// owner garrisons sector 60 with 10 fighters; mover has 5000 and wins
	owner.SetQuantity(FIGHTERS, 200)
	if err := u.AdjustBattlegroup(owner, 60, FIGHTERS, 10); err != nil {
		t.Fatalf("garrison: %v", err)
	}
	mover.SetQuantity(FIGHTERS, 5000)
	mover.MoveTo(59)
	u.Sectors[59].AddWarp(60)
	u.Sectors[60].AddWarp(59)

	report, err := u.MovePlayer(mover, 60)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if mover.Sector != 60 {
		t.Errorf("mover did not enter after winning; sector %d", mover.Sector)
	}
	if len(report) == 0 {
		t.Errorf("no combat report for contested entry")
	}
	if mover.GetQuantity(FIGHTERS) >= 5000 {
		t.Errorf("mover lost no fighters in the fight")
	}
	if bg, _ := u.GetBattlegroup(owner, 60, false); bg != nil && bg.GetQuantity(FIGHTERS) > 0 {
		t.Errorf("garrison survived a 5000 vs 10 fight")
	}
	if news := u.TakeNews(owner.Id); len(news) == 0 {
		t.Errorf("garrison owner got no news")
	}
}

func TestMinefieldEntry(t *testing.T) {
	u, mover, owner := combatUniverse(t)

	owner.SetQuantity(MINES, 3)
	if err := u.AdjustBattlegroup(owner, 60, MINES, 3); err != nil {
		t.Fatalf("minefield: %v", err)
	}
	mover.SetQuantity(FIGHTERS, 5000)
	mover.MoveTo(59)
	u.Sectors[59].AddWarp(60)
	u.Sectors[60].AddWarp(59)

	report, err := u.MovePlayer(mover, 60)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	lost := 5000 - mover.GetQuantity(FIGHTERS)
	if lost < 900 || lost > 1497 {
		t.Errorf("3 mines destroyed %d fighters; want 900..1497", lost)
	}
	if mover.TotalSystemDamage() == 0 {
		t.Errorf("mine blasts caused no system damage")
	}
	if len(report) != 3 {
		t.Errorf("report has %d lines; want 3 mine detonations", len(report))
	}
	// minefield consumed, empty battlegroup removed
	if bg, _ := u.GetBattlegroup(owner, 60, false); bg != nil {
		t.Errorf("spent minefield battlegroup not removed")
	}
}

func TestDeathAndReconstruction(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	p, _ := u.RegisterPlayer("Mortal", "m@example.com", "")

	day1 := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)

	u.KillPlayer(p, day1.Unix())

	// same day: still dead
	msg, revived := u.ReconstructIfDue(p, day1.Add(2*time.Hour))
	if revived || !p.IsDead() {
		t.Fatalf("player reconstructed on the day of death")
	}
	if msg == "" {
		t.Errorf("no lockout message for a dead player")
	}

	// next day: first death is free
	msg, revived = u.ReconstructIfDue(p, day2)
	if !revived || p.IsDead() {
		t.Fatalf("player not reconstructed the next day: %q", msg)
	}
	if p.Sector != 1 {
		t.Errorf("reconstructed in sector %d; want 1", p.Sector)
	}
	if p.GetQuantity(HOLDS) != 25 || p.GetQuantity(FIGHTERS) != 200 || p.Money != 35000 {
		t.Errorf("first death penalized: holds=%d fighters=%d credits=%d",
			p.GetQuantity(HOLDS), p.GetQuantity(FIGHTERS), p.Money)
	}

	// second death: deathper = round((100-50)*0.65) = 33 -> 67% kit
	u.KillPlayer(p, day2.Unix())
	_, revived = u.ReconstructIfDue(p, day3)
	if !revived {
		t.Fatalf("second reconstruction failed")
	}
	if got := p.GetQuantity(HOLDS); got != 25*67/100 {
		t.Errorf("holds after 2nd death = %d; want %d", got, 25*67/100)
	}
	if got := p.GetQuantity(FIGHTERS); got != 200*67/100 {
		t.Errorf("fighters after 2nd death = %d; want %d", got, 200*67/100)
	}
	if p.Money != 35000*67/100 {
		t.Errorf("credits after 2nd death = %d; want %d", p.Money, 35000*67/100)
	}
	if got := p.GetQuantity(TURNS); got != 250 {
		t.Errorf("turns after reconstruction = %d; want a fresh 250", got)
	}
}

func TestNewsLifecycle(t *testing.T) {
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()
	p, _ := u.RegisterPlayer("Reader", "r@example.com", "")

	now := time.Now().Unix()
	u.AddNews(p.Id, now, "first")
	u.AddNews(p.Id, now, "second")

	news := u.TakeNews(p.Id)
	if len(news) != 2 || news[0] != "first" || news[1] != "second" {
		t.Fatalf("TakeNews = %v", news)
	}
	if again := u.TakeNews(p.Id); len(again) != 0 {
		t.Errorf("news delivered twice: %v", again)
	}

	// delivered old news is trimmed; undelivered old news survives
	u.AddNews(p.Id, now-10*86400, "ancient undelivered")
	for _, n := range u.News {
		if n.Delivered {
			n.At = now - 10*86400 // age the delivered items past the cutoff
		}
	}
	u.trimNews(now - 86400)
	if len(u.News) != 1 || u.News[0].Msg != "ancient undelivered" {
		t.Errorf("trim kept wrong items: %d items", len(u.News))
	}

	// NPCs never accumulate news
	npc := u.EnsureNPC("renegades")
	u.AddNews(npc.Id, now, "npc mail")
	for _, n := range u.News {
		if n.Player == npc.Id {
			t.Errorf("news queued for an NPC")
		}
	}
}

func TestShipDamageAndRepair(t *testing.T) {
	u := NewUniverse()
	u.Generate(100)
	u.SeedDefaultConfig()
	p, _ := u.RegisterPlayer("Wrench", "w@example.com", "")

	p.DamageSystem(SysEngines, 3)
	if _, err := u.MovePlayer(p, u.Sectors[1].GetWarps()[0]); err == nil {
		t.Errorf("move allowed with damaged engines")
	}
	if err := u.CheckSystem(p, SysSensors); err != nil {
		t.Errorf("undamaged system reported damaged: %v", err)
	}

	// damage caps at MaxShipDamage
	p.DamageSystem(SysCargoBay, 9999)
	if got := p.Systems[SysCargoBay]; got != MaxShipDamage {
		t.Errorf("damage = %d; want cap %d", got, MaxShipDamage)
	}

	// docking is blocked by a damaged cargo bay
	port := testPort(u, 1, "RepairPort")
	if err := u.Dock(p, port); err == nil {
		t.Errorf("dock allowed with damaged cargo bay")
	}

	// spending a turn heals one point per system (passturn)
	engines := p.Systems[SysEngines]
	if err := u.spendTurn(p); err != nil {
		t.Fatalf("spendTurn: %v", err)
	}
	if p.Systems[SysEngines] != engines-1 || p.Systems[SysCargoBay] != MaxShipDamage-1 {
		t.Errorf("healing wrong: engines %d cargo %d", p.Systems[SysEngines], p.Systems[SysCargoBay])
	}

	// Sol repair: exact cost, everything cleared
	total := p.TotalSystemDamage()
	cost := total * 250
	p.Money = cost - 1
	if err := u.SolRepair(p); err == nil {
		t.Errorf("repair allowed without enough credits")
	}
	p.Money = cost
	if err := u.SolRepair(p); err != nil {
		t.Fatalf("repair: %v", err)
	}
	if p.TotalSystemDamage() != 0 || p.Money != 0 {
		t.Errorf("after repair: damage=%d money=%d", p.TotalSystemDamage(), p.Money)
	}
	if err := u.SolRepair(p); err == nil {
		t.Errorf("repair of an undamaged ship allowed")
	}
}

func TestDeadPlayersCannotTrade(t *testing.T) {
	u, attacker, victim := combatUniverse(t)
	// give the victim cargo and money so a trade would otherwise succeed
	victim.SetQuantity(ORE, 10)
	victim.Money = 100000
	port := testPort(u, victim.Sector, "GhostPort")

	// victim is killed while "docked" (port pointer already captured)
	u.KillPlayer(victim, time.Now().Unix())

	moneyBefore := victim.Money
	oreBefore := victim.GetQuantity(ORE)
	portOreBefore := port.GetQuantity(ORE)

	if err := u.TradeSell(ORE, port, victim, 5); err == nil {
		t.Errorf("dead player completed a sell")
	}
	if err := u.TradeBuy(ORGANICS, port, victim, 5); err == nil {
		t.Errorf("dead player completed a buy")
	}
	sol := &Commodity{Name: FIGHTERS, SellPrice: 98, Sell: true}
	if err := u.TradeBuyNoLimit(sol, victim, 5); err == nil {
		t.Errorf("dead player completed a Sol purchase")
	}
	if err := u.Dock(victim, port); err == nil {
		t.Errorf("dead player docked")
	}

	// nothing moved
	if victim.Money != moneyBefore || victim.GetQuantity(ORE) != oreBefore || port.GetQuantity(ORE) != portOreBefore {
		t.Errorf("ghost trade mutated state: money %d->%d, ore %d->%d, port %d->%d",
			moneyBefore, victim.Money, oreBefore, victim.GetQuantity(ORE), portOreBefore, port.GetQuantity(ORE))
	}

	// a live player at the same port trades fine (guard isn't over-broad)
	if err := u.Dock(attacker, port); err != nil {
		t.Fatalf("live dock: %v", err)
	}
}

func TestDeadPlayersAreNotTargets(t *testing.T) {
	u, attacker, defender := combatUniverse(t)
	u.KillPlayer(defender, time.Now().Unix())

	if _, err := u.AttackPlayer(attacker, defender.Id, 10); err == nil {
		t.Errorf("attack on a dead player allowed")
	}
	if _, err := u.MovePlayer(defender, 2); err == nil {
		t.Errorf("dead player allowed to move")
	}
}
