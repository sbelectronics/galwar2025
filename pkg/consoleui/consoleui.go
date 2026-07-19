package consoleui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// ConsoleUI is a session that drives the game engine from any Terminal
// (local console, WebSocket, telnet). It follows the actor rules: gather
// input from the player first, then submit one complete operation to the
// universe via Do/DoErr. Prompts never happen while holding the universe.

type ConsoleUI struct {
	Universe   *galwar.UniverseType
	Player     *galwar.Player
	Term       Terminal
	Terminated bool
	input      string

	// OnError, if set, is called with every engine error shown to the player.
	// The interactive front-ends leave it nil; the bot simulation uses it to
	// classify and record errors without screen-scraping the transcript.
	OnError func(err error)
}

func NewConsoleUI(universe *galwar.UniverseType, player *galwar.Player, term Terminal) *ConsoleUI {
	return &ConsoleUI{
		Universe: universe,
		Player:   player,
		Term:     term,
	}
}

func (c *ConsoleUI) printf(format string, args ...any) {
	c.Term.Printf(format, args...)
}

func (c *ConsoleUI) PrintError(err error) {
	if c.OnError != nil {
		c.OnError(err)
	}
	// the original showed rule violations in light red throughout
	c.printf("%s%s%s\n", LightRed, ErrText(err), Reset)
	// an error aborts anything queued behind it: an autopilot course (or any
	// ';' chain) whose step fails would otherwise keep replaying from the
	// wrong sector, spraying "You cannot go to that sector!" at the player
	if c.input != "" {
		c.input = ""
		c.printf("(remaining queued commands discarded)\n")
	}
}

// GetInput returns the next input token. Whole lines are read (so names may
// contain spaces); ';' separates chained commands, which is how the
// autopilot queues its course. Case is preserved - command dispatch
// lowercases where it compares. On EOF/disconnect the session terminates
// instead of spinning.
func (c *ConsoleUI) GetInput() string {
	scanned := false
	if c.input == "" {
		line, err := c.Term.ReadLine()
		if err != nil {
			c.Terminated = true
			return ""
		}
		c.input = strings.TrimSpace(line)
		scanned = true
	}

	if idx := strings.Index(c.input, ";"); idx != -1 {
		result := strings.TrimSpace(c.input[:idx])
		c.input = c.input[idx+1:]
		c.printf("%s\n", result)
		return result
	}

	result := c.input
	c.input = ""

	if !scanned {
		c.printf("%s\n", result)
	}

	return result
}

func (c *ConsoleUI) PromptString(prompt string) string {
	c.printf("%s", prompt)
	return c.GetInput()
}

func (c *ConsoleUI) PromptBool(prompt string) bool {
	for !c.Terminated {
		c.printf("%s", prompt)
		input := strings.ToLower(c.GetInput())
		if input == "y" {
			return true
		} else if input == "n" || input == "" {
			return false
		}
	}
	return false
}

// PromptInt reads a number. It returns ok=false to abort the current command
// when the player types "q" (or a bare Enter), or on disconnect - so a stray
// keystroke never spins the prompt forever and never logs the player out.
// Non-numeric input gets a hint and re-prompts.
func (c *ConsoleUI) PromptInt(prompt string) (int, bool) {
	for !c.Terminated {
		c.printf("%s", prompt)
		input := strings.TrimSpace(c.GetInput())
		if input == "" || strings.EqualFold(input, "q") {
			return 0, false
		}
		if result, err := strconv.Atoi(input); err == nil {
			return result, true
		}
		c.printf("Please enter a number, or Q to cancel.\n")
	}
	return 0, false
}

// PromptIntDefault is PromptInt with a default: a bare Enter takes the default
// (ok=true), while "q" aborts (ok=false).
func (c *ConsoleUI) PromptIntDefault(prompt string, def int) (int, bool) {
	for !c.Terminated {
		c.printf("%s", prompt)
		input := strings.TrimSpace(c.GetInput())
		if input == "" {
			return def, true
		}
		if strings.EqualFold(input, "q") {
			return 0, false
		}
		if result, err := strconv.Atoi(input); err == nil {
			return result, true
		}
		c.printf("Please enter a number, <Enter> for %d, or Q to cancel.\n", def)
	}
	return def, false
}

func (c *ConsoleUI) GetWarpStrings(warps []int) []string {
	warpStrings := []string{}
	for _, warp := range warps {
		warpStrings = append(warpStrings, fmt.Sprintf("%d", warp))
	}
	return warpStrings
}

// getWarps snapshots a sector's warp list from inside the universe actor.
func (c *ConsoleUI) getWarps(secnum int) []int {
	var warps []int
	c.Universe.Do(func() {
		warps = append(warps, c.Universe.Sectors[secnum].GetWarps()...)
	})
	return warps
}

// objectColor is the original display_sector palette (TWLIB1.PAS:917-1035):
// ports green, planets light gray, other ships light cyan, sector forces
// light red.
func objectColor(kind string) string {
	switch kind {
	case galwar.TYPE_PORT:
		return Green
	case galwar.TYPE_PLANET:
		return LightGray
	case galwar.TYPE_PLAYER:
		return LightCyan
	case galwar.TYPE_BATTLEGROUP:
		return LightRed
	}
	return LightGray
}

// fedShipCap bounds the ship list in a Federation-space sector display:
// sectors 1-10 collect newly-registered and sheltering ships, and the busiest
// screen in the game (Sol) must not open with a wall of parked starters. The
// highest-ranked ships are shown; the remainder is summarized as a count.
// Sectors 11+ are never capped - a fleet pileup out there is combat
// intelligence the player is entitled to see in full.
const fedShipCap = 5

func (c *ConsoleUI) printObject(obj galwar.ObjectInterface) {
	color := objectColor(obj.GetType())
	extra := obj.GetNameExtra()
	if extra != "" {
		c.printf("%s%s: %s, %s%s\n", color, obj.GetType(), obj.GetName(), extra, Reset)
	} else {
		c.printf("%s%s: %s%s\n", color, obj.GetType(), obj.GetName(), Reset)
	}
}

func (c *ConsoleUI) DisplaySector(secnum int) {
	now := galwar.Now()
	c.Universe.Do(func() {
		c.printf("%sSector: %d%s\n", LightRed, secnum, Reset)

		objs := c.Universe.GetVisibleObjectsInSector(secnum, "", c.Player, now)

		// in Federation space, cap the ship list at the top fedShipCap by
		// value; ports, planets, and defense forces are never capped
		var ships []*galwar.Player
		for _, obj := range objs {
			if p, ok := obj.(*galwar.Player); ok && p != c.Player {
				ships = append(ships, p)
			}
		}
		capped := secnum <= 10 && len(ships) > fedShipCap
		if capped {
			vals := map[*galwar.Player]int{}
			for _, p := range ships {
				vals[p] = c.Universe.PlayerValue(p)
			}
			sort.SliceStable(ships, func(i, j int) bool { return vals[ships[i]] > vals[ships[j]] })
		}

		shownShipBlock := false
		for _, obj := range objs {
			if obj == c.Player {
				// don't show yourself
				continue
			}
			if _, ok := obj.(*galwar.Player); ok && capped {
				// the capped ship block prints once, where ships appear in the
				// normal display order
				if !shownShipBlock {
					shownShipBlock = true
					for _, p := range ships[:fedShipCap] {
						c.printObject(p)
					}
					c.printf("%s(%d players not displayed)%s\n", LightCyan, len(ships)-fedShipCap, Reset)
				}
				continue
			}
			c.printObject(obj)
		}

		warps := c.Universe.Sectors[secnum].GetWarps()
		c.printf("%sWarps lead to: %s%s\n", Green, strings.Join(c.GetWarpStrings(warps), ", "), Reset)
	})
}

func (c *ConsoleUI) ExecuteHelp() {
	lines := []string{
		"             [COMBAT]             [TACTICAL]          [MOVEMENT]            ",
		"",
		"        [A] Attack Player      [S] Sensor Scan     [M] Move to Sector       ",
		"        [D] Drop Mine          [C] Computer        [L] Land/Invade planet   ",
		"        [F] Take/Leave fgtrs   [I] Your info       [P] Dock at port         ",
		"        [G] Launch Group       [B] Use Device      [Y] Engage Autopilot     ",
		"                               [H] Damage Control  [R] Starbase Transporter ",
		"",
		"             [HELP]               [MISC]              [PLANETS]             ",
		"",
		"        [?] This menu          [V] Record Macro    [J] Create Planet        ",
		"        [Z] Instructions       [W] Plasma device   [O] <Removed>            ",
		"                               [Q] Quit to bbs     [U] Starbase maint       ",
		"                               [T] Team Menu                                ",
	}
	for _, line := range lines {
		if line == "" {
			c.printf("\n")
			continue
		}
		c.printf("%s\n", HelpLine(line))
	}
	c.printf("\n")
	c.printf("%s\n", HelpLine("Implemented: A, B, C, D, F, G, H, I, J, L, M, P, Q, S, W, Y, Z  (plus [PASS], [REPORT], [SYSOP])"))
}

func (c *ConsoleUI) ExecuteMove() {
	warps := c.getWarps(c.Player.Sector)
	c.printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(warps), ", "))

	secnum, ok := c.PromptInt("\nMove to what sector? ")
	if !ok {
		return
	}

	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.MovePlayer(c.Player, secnum)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printReport(report)
}

// printReport shows a combat narration in the original's battle red.
func (c *ConsoleUI) printReport(report []string) {
	for _, line := range report {
		c.printf("%s%s%s\n", LightRed, line, Reset)
	}
}

// ExecutePlasma is the W command: use a plasma device to add or remove a
// two-way warp link from the current sector.
func (c *ConsoleUI) ExecutePlasma() {
	var have int
	c.Universe.Do(func() { have = c.Player.GetQuantity(galwar.PLASMA) })
	if have < 1 {
		c.printf("You have no plasma devices!\n")
		return
	}
	c.printf("%sPlasma device (%d available)%s\n", LightCyan, have, Reset)
	choice := strings.ToLower(c.PromptString("(A)dd a warp link, (R)emove one, or <Enter> to abort? "))
	var action galwar.PlasmaAction
	switch choice {
	case "a":
		action = galwar.PlasmaAdd
	case "r":
		action = galwar.PlasmaRemove
	default:
		return
	}
	target, ok := c.PromptInt("Which sector? ")
	if !ok {
		return
	}
	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.UsePlasma(c.Player, action, target)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	for _, line := range report {
		c.printf("%s%s%s\n", LightCyan, line, Reset)
	}
}

// ExecutePulsar drops pulsar bombs on the planet in the current sector
// (which the player must own). Reached from the Land menu.
func (c *ConsoleUI) ExecutePulsar() {
	var have int
	c.Universe.Do(func() { have = c.Player.GetQuantity(galwar.PULSAR) })
	if have < 1 {
		c.printf("You have no pulsar bombs!\n")
		return
	}
	n, ok := c.PromptInt(fmt.Sprintf("You have %d pulsar bombs. Drop how many? ", have))
	if !ok || n < 1 {
		return
	}
	if !c.PromptBool(fmt.Sprintf("Really bomb your own planet with %d pulsar bombs (Y/N)[N] ? ", n)) {
		return
	}
	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.UsePulsar(c.Player, n)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printReport(report)
}

// ExecuteAttack is the original's A command (GWMISC.PAS:269): pick a target
// in your sector, commit fighters, and the whole battle - exchange,
// counter-attack, salvage, boobytraps - resolves in one atomic command.
// The defender doesn't need to be online; they read about it in the news.
func (c *ConsoleUI) ExecuteAttack() {
	type candidate struct {
		id   galwar.PlayerId
		name string
	}
	var targets []candidate
	now := galwar.Now()
	c.Universe.Do(func() {
		for _, obj := range c.Universe.GetVisibleObjectsInSector(c.Player.Sector, galwar.TYPE_PLAYER, c.Player, now) {
			p, ok := obj.(*galwar.Player)
			if !ok || p == c.Player || p.IsDead() {
				continue
			}
			targets = append(targets, candidate{id: p.Id, name: p.GetName()})
		}
	})
	if len(targets) == 0 {
		c.printf("There is nobody here to attack.\n")
		return
	}

	c.printf("\n")
	for i, t := range targets {
		c.printf("%s[%d]%s %s\n", Cyan, i+1, Reset, t.name)
	}
	choice, ok := c.PromptInt("\nAttack which ship (0=abort) ? ")
	if !ok || choice < 1 || choice > len(targets) {
		return
	}
	target := targets[choice-1]

	if !c.PromptBool(fmt.Sprintf("Attack %s (Y/N)[N] ? ", target.name)) {
		return
	}

	var fighters int
	c.Universe.Do(func() {
		fighters = c.Player.GetQuantity(galwar.FIGHTERS)
	})
	commit, ok := c.PromptIntDefault(fmt.Sprintf("How many fighters do you wish to use [%d] ? ", fighters), fighters)
	if !ok {
		return
	}

	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.AttackPlayer(c.Player, target.id, commit)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printReport(report)
}

// ExecuteDamageControl is the original's H command: the six ship systems
// and their damage in turns. Damage heals one point per turn spent, or all
// at once (for a price) at Sol.
func (c *ConsoleUI) ExecuteDamageControl() {
	c.Universe.Do(func() {
		total := c.Player.TotalSystemDamage()
		if total == 0 {
			c.printf("%sAll ship systems are operational.%s\n", LightGreen, Reset)
			return
		}
		c.printf("\n%s System                 Damage (turns)\n", Green)
		c.printf(" ======================  ==============%s\n", White)
		for i, name := range galwar.SystemNames {
			if c.Player.Systems[i] > 0 {
				c.printf(" %-22s  %d\n", name, c.Player.Systems[i])
			}
		}
		c.printf("%s\n%sDamage heals 1 point per turn spent; Sol repairs everything for %d credits per point.%s\n",
			Reset, LightCyan, c.Universe.ConfigInt("cost_of_repair", 250), Reset)
	})
}

// ExecuteLaunchGroup is the [G] command: send a battle group (strike fleet)
// to a remote sector. It fights through the whole route; survivors return.
func (c *ConsoleUI) ExecuteLaunchGroup() {
	if err := c.Universe.DoErr(func() error {
		return c.Universe.CheckSystem(c.Player, galwar.SysBGComputer)
	}); err != nil {
		c.PrintError(err)
		return
	}
	target, ok := c.PromptInt("\nSend a battle group to what sector? ")
	if !ok {
		return
	}
	var have int
	c.Universe.Do(func() { have = c.Player.GetQuantity(galwar.FIGHTERS) })
	ships, ok := c.PromptInt(fmt.Sprintf("How many ships do you send (1 = scout; you have %d) ? ", have))
	if !ok || ships < 1 {
		return
	}
	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.LaunchBattleGroup(c.Player, target, ships)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printReport(report)
}

// ExecuteUseDevice is the [B] Use Device command. The only activatable device
// so far is the Pulsar Tube, which launches pulsar bombs at a planet in your
// sector from orbit (500/bomb, any planet).
func (c *ConsoleUI) ExecuteUseDevice() {
	var tubes, bombs, planets int
	c.Universe.Do(func() {
		tubes = c.Player.GetQuantity(galwar.PULSARTUBE)
		bombs = c.Player.GetQuantity(galwar.PULSAR)
		planets = len(c.Universe.GetObjectsInSector(c.Player.Sector, galwar.TYPE_PLANET))
	})
	if tubes < 1 {
		c.printf("You have no usable devices.\n")
		return
	}
	// (when more activatable devices exist, this becomes a menu)
	if planets == 0 {
		c.printf("There is no planet here to bomb.\n")
		return
	}
	if bombs < 1 {
		c.printf("You have no pulsar bombs to launch.\n")
		return
	}
	n, ok := c.PromptInt(fmt.Sprintf("Pulsar Tube: launch how many of your %d pulsar bombs? ", bombs))
	if !ok || n < 1 {
		return
	}
	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.UsePulsarTube(c.Player, n)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printReport(report)
}

func (c *ConsoleUI) ExecuteScan() {
	if err := c.Universe.DoErr(func() error {
		return c.Universe.CheckSystem(c.Player, galwar.SysSensors)
	}); err != nil {
		c.PrintError(err)
		return
	}
	warps := c.getWarps(c.Player.Sector)

	c.printf("\n")
	c.printf("[-------------------------------------]\n")

	for _, warp := range warps {
		c.DisplaySector(warp)

		c.printf("\n")
	}

	c.printf("[-------------------------------------]\n")
}

func (c *ConsoleUI) ExecuteAutopilot() {
	sec, ok := c.PromptInt("\nWhat sector do you wish to go to ? ")
	if !ok {
		return
	}

	if sec == c.Player.Sector {
		c.printf("You are in that sector!\n")
		return
	}

	var path []int
	c.Universe.Do(func() {
		path = c.Universe.ShortestPathTo(c.Player.Sector, sec)
	})

	if path == nil {
		c.printf("There is no route from sector %d to sector %d!\n", c.Player.Sector, sec)
		return
	}

	pathStrings := []string{}
	for _, pathSec := range path {
		pathStrings = append(pathStrings, fmt.Sprintf("%d", pathSec))
	}

	c.printf("The shortest path from sector %d to sector %d is: %s\n", c.Player.Sector, sec, strings.Join(pathStrings, ","))

	commit := c.PromptBool("\nEnter course into autopilot(Y/N) ?")
	if commit {
		pathStrings = []string{}
		for _, pathSec := range path[1:] {
			pathStrings = append(pathStrings, fmt.Sprintf("m;%d", pathSec))
		}
		c.input = strings.Join(pathStrings, ";")
	}
}

// DockServicePort is the fixed-price buy menu for special ports (Sol, Amazing
// Devices): list each item with its price and let the player buy any number.
func (c *ConsoleUI) DockServicePort(port *galwar.Port) {
	choices := map[string]*galwar.Commodity{}

	c.Universe.Do(func() {
		// commerce report: yellow title, green headers, white rows
		// (TWARS.PAS port procedure)
		c.printf("%sCommerce Report For %s: %s%s\n", Yellow, port.GetName(), galwar.Now().Format("2006-01-02 15:04:05"), Reset)
		c.printf("\n")

		c.printf("%s##  Item name               Cost      Can Afford\n", Green)
		c.printf("--  ----------------------  --------  ----------%s\n", White)

		for i, cm := range port.Inventory {
			canAfford := int(math.Floor(float64(c.Player.GetMoney()) / cm.EffectiveSellPrice()))
			c.printf("%2d  %-22s %9d %11d\n", i+1, cm.Name, int(cm.GetPrice()), canAfford)
			choices[fmt.Sprintf("%d", i+1)] = cm
		}
		repairCost := c.Universe.ConfigInt("cost_of_repair", 250)
		c.printf(" R  %-22s %9d %s(per point of damage; you have %d)%s\n", "Ship Repair", repairCost, Cyan, c.Player.TotalSystemDamage(), White)
		c.printf("%s", Reset)
	})

	for !c.Terminated {
		input := strings.ToLower(c.PromptString("\nEnter number to buy, <R> to repair, or <Q> to quit > "))
		if input == "q" {
			return
		}
		if input == "r" {
			err := c.Universe.DoErr(func() error {
				return c.Universe.SolRepair(c.Player)
			})
			if err != nil {
				c.PrintError(err)
			} else {
				c.printf("%sAll ship systems repaired.%s\n", LightGreen, Reset)
			}
			continue
		}
		commodity, exists := choices[input]
		if !exists {
			c.printf("Invalid choice. Please try again.\n")
			continue
		}

		var canAfford int
		c.Universe.Do(func() {
			canAfford = int(math.Floor(float64(c.Player.GetMoney()) / commodity.EffectiveSellPrice()))
		})

		qty, ok := c.PromptInt(fmt.Sprintf("\nYou can afford %d %s. How many do you want? ", canAfford, commodity.Name))
		if !ok || qty <= 0 {
			continue
		}

		err := c.Universe.DoErr(func() error {
			return c.Universe.TradeBuyNoLimit(commodity, c.Player, qty)
		})
		if err != nil {
			c.PrintError(err)
		}
	}
}

func (c *ConsoleUI) DockPort() {
	var port *galwar.Port
	c.Universe.Do(func() {
		ports := c.Universe.GetObjectsInSector(c.Player.Sector, galwar.TYPE_PORT)
		if len(ports) > 0 {
			port, _ = ports[0].(*galwar.Port)
		}
	})
	if port == nil {
		c.printf("No ports in this sector\n")
		return
	}

	// docking charges a turn (Sol excepted) and refreshes the port's stock
	if err := c.Universe.DoErr(func() error {
		return c.Universe.Dock(c.Player, port)
	}); err != nil {
		c.PrintError(err)
		return
	}

	if port.Goods == galwar.Interstel {
		c.DockBank()
		return
	}
	if port.Goods == galwar.Sol || port.Goods == galwar.AmazingDevices {
		c.DockServicePort(port)
		return
	}

	// Snapshot the commerce report and the trading order.
	type tradeItem struct {
		name string
		sell bool
	}
	items := []tradeItem{}

	c.Universe.Do(func() {
		// commerce report: yellow title, green headers, white rows
		// (TWARS.PAS port procedure)
		c.printf("%sCommerce Report For %s: %s%s\n", Yellow, port.GetName(), galwar.Now().Format("2006-01-02 15:04:05"), Reset)
		c.printf("\n")
		c.printf("%s Items     Status    Price  # units  In holds\n", Green)
		c.printf(" =====     ======    =====  =======  ========%s\n", White)

		for _, cm := range port.Inventory {
			c.printf("%-10s %-9s %5.2f %8d %9d\n", cm.Name, cm.GetBuySell(), cm.GetPrice(), galwar.ScaleUp(c.Player, cm.Quantity), c.Player.GetQuantity(cm.Name))
			items = append(items, tradeItem{name: cm.Name, sell: cm.Sell})
		}
		c.printf("%s", Reset)
	})

	for _, item := range items {
		if item.sell {
			continue
		}
		for !c.Terminated {
			// killed mid-dock? the run loop delivers the news and death
			// notice; just stop trading (the engine already refuses, but
			// don't loop forever re-prompting a ghost)
			if c.isDead() {
				return
			}
			var portWants, inHolds int
			c.Universe.Do(func() {
				portWants = galwar.ScaleUp(c.Player, port.GetQuantity(item.name))
				inHolds = c.Player.GetQuantity(item.name)
			})
			buyAllow := min(inHolds, portWants)
			c.printf("\nWe are buying up to %d of %s. You have %d in your holds.\n", portWants, item.name, inHolds)
			input, ok := c.PromptIntDefault(fmt.Sprintf("How many holds of %s do you want to sell [%d] ? ", item.name, buyAllow), buyAllow)
			if c.Terminated {
				return
			}
			if !ok {
				break // Q: skip this good, move to the next
			}
			err := c.Universe.DoErr(func() error {
				return c.Universe.TradeSell(item.name, port, c.Player, input)
			})
			if err == nil {
				break
			}
			c.PrintError(err)
		}
	}

	for _, item := range items {
		if !item.sell {
			continue
		}
		for !c.Terminated {
			if c.isDead() {
				return
			}
			var portHas, inHolds, sellAllow int
			c.Universe.Do(func() {
				cm := port.GetCommodity(item.name)
				portHas = galwar.ScaleUp(c.Player, cm.Quantity)
				inHolds = c.Player.GetQuantity(item.name)
				sellAllow = min(c.Player.GetFreeHolds(), portHas, int(math.Floor(float64(c.Player.GetMoney())/cm.EffectiveSellPrice())))
			})
			c.printf("\nWe are selling up to %d of %s. You have %d in your holds.\n", portHas, item.name, inHolds)
			input, ok := c.PromptIntDefault(fmt.Sprintf("How many holds of %s do you want to buy [%d] ? ", item.name, sellAllow), sellAllow)
			if c.Terminated {
				return
			}
			if !ok {
				break // Q: skip this good, move to the next
			}
			err := c.Universe.DoErr(func() error {
				return c.Universe.TradeBuy(item.name, port, c.Player, input)
			})
			if err == nil {
				break
			}
			c.PrintError(err)
		}
	}
}

// DockBank is the Interstel port's menu: deposit, withdraw, and the account
// statement. The account survives ship destruction (bank.go), which is the
// entire sales pitch, so the teller says so.
func (c *ConsoleUI) DockBank() {
	first := true
	for !c.Terminated {
		if c.isDead() {
			return
		}
		var balance, money, pct, capAmt int
		c.Universe.Do(func() {
			balance = c.Player.BankBalance
			money = c.Player.GetMoney()
			pct = c.Universe.ConfigInt("bank_interest_pct", 1)
			capAmt = c.Universe.ConfigInt("bank_interest_cap", 1000000)
		})
		if first {
			first = false
			c.printf("%sInterstel Galactic Banking: %s%s\n", Yellow, galwar.Now().Format("2006-01-02 15:04:05"), Reset)
			c.printf("\n%sDeposits earn %d%% interest nightly on the first %d credits,\nand your account survives the destruction of your ship.%s\n", LightCyan, pct, capAmt, Reset)
		}
		c.printf("\n%s  Account balance: %s%d\n", Cyan, White, balance)
		c.printf("%s   Credits aboard: %s%d%s\n", Cyan, White, money, Reset)

		input := strings.ToLower(c.PromptString("\n(D)eposit, (W)ithdraw, or (Q)uit ? "))
		switch input {
		case "d":
			amount, ok := c.PromptIntDefault(fmt.Sprintf("Deposit how many credits [%d] ? ", money), money)
			if !ok || amount < 1 {
				continue
			}
			if err := c.Universe.DoErr(func() error {
				return c.Universe.BankDeposit(c.Player, amount)
			}); err != nil {
				c.PrintError(err)
			}
		case "w":
			amount, ok := c.PromptIntDefault(fmt.Sprintf("Withdraw how many credits [%d] ? ", balance), balance)
			if !ok || amount < 1 {
				continue
			}
			if err := c.Universe.DoErr(func() error {
				return c.Universe.BankWithdraw(c.Player, amount)
			}); err != nil {
				c.PrintError(err)
			}
		case "q", "":
			return
		}
	}
}

func (c *ConsoleUI) ExecuteInfo() {
	// info screen: cyan labels, white values (TWARS.PAS:148-198)
	c.Universe.Do(func() {
		// right-align every label to the longest one in play, so a
		// late-catalog device ("Anti-Cloaking Device") can't overflow the column
		width := len("Bank Balance")
		for _, cm := range c.Player.Inventory {
			if !cm.IsCargo() && len(cm.Name) > width {
				width = len(cm.Name)
			}
		}
		row := func(label string, val any) {
			c.printf("%s%*s: %s%v\n", Cyan, width, label, White, val)
		}
		c.printf("\n")
		row("Name", c.Player.GetName())
		row("Credits", c.Player.GetMoney())
		if c.Player.BankBalance > 0 {
			row("Bank Balance", c.Player.BankBalance)
		}
		if c.Player.BankedTurns > 0 {
			row("Banked Turns", c.Player.BankedTurns)
		}
		c.printf("%s%*s:%s", Cyan, width, "Cargo", White)
		for _, cm := range c.Player.Inventory {
			if cm.IsCargo() {
				c.printf(" %s: %d", cm.GetShortName(), cm.Quantity)
			}
		}
		c.printf("\n")
		for _, cm := range c.Player.Inventory {
			if !cm.IsCargo() {
				row(cm.Name, cm.Quantity)
			}
		}
		c.printf("%s", Reset)
	})
}

func (c *ConsoleUI) ExecuteBattleGroup(kind string) {
	var total int
	err := c.Universe.DoErr(func() error {
		bg, err := c.Universe.GetBattlegroup(c.Player, c.Player.Sector, false)
		if err != nil {
			return err
		}
		total = c.Player.GetQuantity(kind)
		if bg != nil {
			total += bg.GetQuantity(kind)
		}
		return nil
	})
	if err != nil {
		c.PrintError(err)
		return
	}

	amount, ok := c.PromptInt(fmt.Sprintf("You have %d total %s. How many do you want to defend this sector? ", total, kind))
	if !ok {
		return
	}

	err = c.Universe.DoErr(func() error {
		return c.Universe.AdjustBattlegroup(c.Player, c.Player.Sector, kind, amount)
	})
	if err != nil {
		c.PrintError(err)
	}
}

func (c *ConsoleUI) ExecuteGenesis() {
	name := c.PromptString("Enter the name of your new planet: ")
	if c.Terminated {
		return
	}
	err := c.Universe.DoErr(func() error {
		return c.Universe.UseGenesisDevice(c.Player, c.Player.Sector, name)
	})
	if err != nil {
		c.PrintError(err)
	}
}

// ExecuteSetPassword sets the player's telnet password (hidden command
// "pass", in the spirit of the original's hidden VER/MEM commands).
func (c *ConsoleUI) ExecuteSetPassword() {
	c.printf("Set a password for telnet logins (also used if you connect with a classic client).\n")
	c.printf("New password: ")
	pass, err := ReadPassword(c.Term)
	if err != nil {
		c.Terminated = true
		return
	}
	c.printf("Repeat password: ")
	again, err := ReadPassword(c.Term)
	if err != nil {
		c.Terminated = true
		return
	}
	if pass != again {
		c.printf("Passwords do not match.\n")
		return
	}
	serr := c.Universe.DoErr(func() error {
		return c.Universe.SetTelnetPassword(c.Player, pass)
	})
	if serr != nil {
		c.PrintError(serr)
		return
	}
	c.printf("Password set. You can now log in by telnet with your handle.\n")
}

func (c *ConsoleUI) PlanetReport(planet *galwar.Planet) {
	c.Universe.Do(func() {
		c.printf("%sPlanet report For %s: %s%s\n", Yellow, planet.GetName(), galwar.Now().Format("2006-01-02 15:04:05"), Reset)
		c.printf("\n")
		c.printf("%s Items      Prod     # units  In holds\n", Green)
		c.printf(" =====     ======    =======  ========%s\n", White)

		for _, cm := range planet.Inventory {
			c.printf("%-10s %6d %10d %9d\n", cm.Name, cm.Prod, cm.Quantity, c.Player.GetQuantity(cm.Name))
		}
		c.printf("%s", Reset)
	})
}

func (c *ConsoleUI) ExecutePlanetTakeCargo(commodityName string) {
	var wanted int
	err := c.Universe.DoErr(func() error {
		planet, err := c.Universe.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST)
		if err != nil {
			return err
		}
		wanted = min(c.Player.GetFreeHolds(), planet.GetQuantity(commodityName))
		return nil
	})
	if err != nil {
		c.PrintError(err)
		return
	}

	amount, ok := c.PromptIntDefault(fmt.Sprintf("Take how much %s [%d] ? ", commodityName, wanted), wanted)
	if !ok {
		return
	}

	err = c.Universe.DoErr(func() error {
		return c.Universe.TransferOut(c.Player, c.Player.Sector, commodityName, amount)
	})
	if err != nil {
		c.PrintError(err)
	}
}

func (c *ConsoleUI) ExecutePlanetPutCargo() {
	input := c.PromptBool("\nTransfer your cargo to planet (Y/N) ? ")
	if !input {
		return
	}

	err := c.Universe.DoErr(func() error {
		return c.Universe.TransferIn(c.Player, c.Player.Sector)
	})
	if err != nil {
		c.PrintError(err)
	}
}

func (c *ConsoleUI) ExecutePlanetTransfer(commodityName string) {
	var total int
	err := c.Universe.DoErr(func() error {
		planet, err := c.Universe.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST)
		if err != nil {
			return err
		}
		total = c.Player.GetQuantity(commodityName) + planet.GetQuantity(commodityName)
		return nil
	})
	if err != nil {
		c.PrintError(err)
		return
	}

	amount, ok := c.PromptIntDefault(fmt.Sprintf("You have %d %s available, how many to leave on planet? ", total, commodityName), 0)
	if !ok {
		return
	}

	err = c.Universe.DoErr(func() error {
		return c.Universe.TransferSet(c.Player, c.Player.Sector, commodityName, amount)
	})
	if err != nil {
		c.PrintError(err)
	}
}

// ExecuteInvade attacks a hostile planet in the current sector (the L command
// when the planet isn't yours), faithful to the original's land assault.
func (c *ConsoleUI) ExecuteInvade() {
	var have int
	var planetName, ownerName string
	var scanned bool
	var garrison, mines int
	c.Universe.Do(func() {
		have = c.Player.GetQuantity(galwar.FIGHTERS)
		scanned = c.Player.HasPlanetScanner()
		for _, obj := range c.Universe.GetObjectsInSector(c.Player.Sector, galwar.TYPE_PLANET) {
			if pl, ok := obj.(*galwar.Planet); ok {
				planetName = pl.GetName()
				ownerName = pl.GetOwnerPlayer().GetName()
				garrison = pl.GetQuantity(galwar.FIGHTERS)
				mines = pl.GetQuantity(galwar.MINES)
			}
		}
	})
	c.printf("%s%s is held by %s.%s\n", LightRed, planetName, ownerName, Reset)
	// a Planetary Scanner reveals the defenses before you commit
	if scanned {
		c.printf("%sPlanetary Scanner: %d defending fighters, %d planetary mines.%s\n", LightCyan, garrison, mines, Reset)
	}
	if !c.PromptBool("Invade it (Y/N)[N] ? ") {
		return
	}
	commit, ok := c.PromptIntDefault(fmt.Sprintf("Commit how many of your %d fighters [%d] ? ", have, have), have)
	if !ok {
		return
	}
	var report []string
	err := c.Universe.DoErr(func() error {
		r, err := c.Universe.InvadePlanet(c.Player, commit)
		report = r
		return err
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printReport(report)
}

func (c *ConsoleUI) ExecuteLand() {
	if err := c.Universe.DoErr(func() error {
		return c.Universe.CheckSystem(c.Player, galwar.SysThrusters)
	}); err != nil {
		c.PrintError(err)
		return
	}

	// a planet you don't own is invaded, not landed on
	var owned, foreign bool
	c.Universe.Do(func() {
		if _, err := c.Universe.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST); err == nil {
			owned = true
			return
		}
		foreign = len(c.Universe.GetObjectsInSector(c.Player.Sector, galwar.TYPE_PLANET)) > 0
	})
	if !owned {
		if foreign {
			c.ExecuteInvade()
		} else {
			c.printf("No planet found in this sector.\n")
		}
		return
	}

	first := true
	for !c.Terminated {
		var planet *galwar.Planet
		err := c.Universe.DoErr(func() error {
			p, err := c.Universe.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST)
			planet = p
			return err
		})
		if err != nil {
			c.PrintError(err)
			return
		}

		if first {
			c.PlanetReport(planet)
			first = false
		}
		command := strings.ToLower(c.PromptString("\nPlanet Command (?=Help) ? "))

		switch command {
		case "f":
			c.ExecutePlanetTransfer(galwar.FIGHTERS)
		case "1":
			c.ExecutePlanetTakeCargo(galwar.ORE)
		case "2":
			c.ExecutePlanetTakeCargo(galwar.ORGANICS)
		case "3":
			c.ExecutePlanetTakeCargo(galwar.EQUIPMENT)
		case "t":
			c.ExecutePlanetPutCargo()
		case "b":
			c.ExecutePulsar()
		case "l":
			return
		case "v":
			first = true
		case "?":
			c.printf("[F] Fighter transfer\n")
			c.printf("[1] Take Ore\n")
			c.printf("[2] Take Organics\n")
			c.printf("[3] Take Equipment\n")
			c.printf("[T] Transfer Cargo to Planet\n")
			c.printf("[B] Pulsar-bomb this planet\n")
			c.printf("[L] Leave Planet\n")
			c.printf("[V] View Planet Production\n")
		}
	}
}

// ExecuteComputer is the original's C command: an on-board computer with
// read-only sub-reports (COMPUTER.PAS's computer_menu). A damaged ship computer
// disables it, as in the original.
func (c *ConsoleUI) ExecuteComputer() {
	if err := c.Universe.DoErr(func() error {
		return c.Universe.CheckSystem(c.Player, galwar.SysComputer)
	}); err != nil {
		c.PrintError(err)
		return
	}
	for !c.Terminated {
		cmd := strings.ToLower(c.PromptString("\n" + Yellow + "Computer (?=Help) ? " + Reset))
		switch cmd {
		case "l":
			c.ShowRankings()
		case "e":
			c.ShowForces()
		case "p":
			c.ShowPlanetaryStatus()
		case "f":
			c.FindNearestPort()
		case "u":
			c.ShowUniverseStats()
		case "w":
			c.ShowRecentNews()
		case "q", "":
			return
		case "?":
			c.printf("%s[L]%s Rank the greatest warlords     %s[E]%s Find your forces\n", Cyan, Reset, Cyan, Reset)
			c.printf("%s[P]%s Your planetary status          %s[F]%s Find the nearest port\n", Cyan, Reset, Cyan, Reset)
			c.printf("%s[U]%s Universe specifics             %s[W]%s What happened while you were out\n", Cyan, Reset, Cyan, Reset)
			c.printf("%s[Q]%s Quit the computer\n", Cyan, Reset)
		}
	}
}

// ShowForces is the [E] report: every planet and defense force the player owns.
func (c *ConsoleUI) ShowForces() {
	c.Universe.Do(func() {
		forces := c.Universe.PlayerForces(c.Player)
		if len(forces) == 0 {
			c.printf("%sYou have no planets or sector forces deployed.%s\n", LightCyan, Reset)
			return
		}
		c.printf("\n%s  Asset          Sector   Fighters      Mines  Name\n", Green)
		c.printf("  ============   ======   ========   ========  ====================%s\n", White)
		for _, f := range forces {
			c.printf("  %-12s   %6d   %8d   %8d  %s\n", f.Kind, f.Sector, f.Fighters, f.Mines, f.Name)
		}
		c.printf("%s", Reset)
	})
}

// ShowPlanetaryStatus is the [P] report: production and stock for owned
// planets, without flying to each one.
func (c *ConsoleUI) ShowPlanetaryStatus() {
	c.Universe.Do(func() {
		planets := c.Universe.OwnedPlanets(c.Player)
		if len(planets) == 0 {
			c.printf("%sYou don't own any planets.%s\n", LightCyan, Reset)
			return
		}
		for _, pl := range planets {
			c.printf("\n%s%s (sector %d):%s\n", Yellow, pl.GetName(), pl.Sector, Reset)
			c.printf("%s  Item        Prod     # units%s\n", Green, White)
			for _, cm := range pl.Inventory {
				c.printf("  %-10s %6d %10d\n", cm.Name, cm.Prod, cm.Quantity)
			}
		}
		c.printf("%s", Reset)
	})
}

// FindNearestPort is the [F] report: the closest port, optionally one that
// buys or sells a chosen commodity, with the shortest-path course to it.
func (c *ConsoleUI) FindNearestPort() {
	kind := strings.ToLower(c.PromptString("\nNearest (A)ny port, port (B)uying, or port (S)elling a good [A] ? "))
	var want func(*galwar.Port) bool
	var label string
	if kind == "b" || kind == "s" {
		good := c.chooseCommodity()
		if good == "" {
			return
		}
		if kind == "b" {
			want, label = galwar.PortBuys(good), "buying "+good
		} else {
			want, label = galwar.PortSells(good), "selling "+good
		}
	} else {
		label = "any port"
	}

	var sector, distance int
	var name string
	var found bool
	var path []int
	c.Universe.Do(func() {
		sector, distance, name, found = c.Universe.NearestPort(c.Player.Sector, want)
		if found {
			path = c.Universe.ShortestPathTo(c.Player.Sector, sector)
		}
	})
	if !found {
		c.printf("%sNo reachable port %s was found.%s\n", LightCyan, label, Reset)
		return
	}
	if distance == 0 {
		c.printf("%sYou are already at %s (sector %d).%s\n", LightGreen, name, sector, Reset)
		return
	}
	c.printf("%sNearest port %s: %s in sector %d (%d jumps).%s\n", LightCyan, label, name, sector, distance, Reset)
	pathStrings := []string{}
	for _, s := range path {
		pathStrings = append(pathStrings, fmt.Sprintf("%d", s))
	}
	c.printf("Course: %s\n", strings.Join(pathStrings, ", "))
}

// chooseCommodity prompts for one of the three trade goods.
func (c *ConsoleUI) chooseCommodity() string {
	switch strings.ToLower(c.PromptString("Which good - (O)re, or(G)anics, (E)quipment ? ")) {
	case "o":
		return galwar.ORE
	case "g":
		return galwar.ORGANICS
	case "e":
		return galwar.EQUIPMENT
	}
	return ""
}

// ShowUniverseStats is the [U] report: the state of the galaxy.
func (c *ConsoleUI) ShowUniverseStats() {
	now := galwar.Now()
	c.Universe.Do(func() {
		s := c.Universe.Stats(now)
		c.printf("\n%sUniverse Specifics:%s\n", Yellow, Reset)
		c.printf("%s        Sectors: %s%d\n", Cyan, White, s.Sectors)
		c.printf("%s          Ports: %s%d\n", Cyan, White, s.Ports)
		c.printf("%s        Planets: %s%d\n", Cyan, White, s.Planets)
		c.printf("%s Active traders: %s%d\n", Cyan, White, s.ActiveTraders)
		c.printf("%s  Turns per day: %s%d\n", Cyan, White, s.TurnsPerDay)
		turnPrice := 0
		if def := galwar.FindCommodityDef(galwar.TURNS); def != nil {
			turnPrice = int(def.SellPrice)
		}
		c.printf("%s\n%sSol prices:%s cargo hold %d, fighter %d, turn %d\n", Reset, Cyan, White,
			c.Universe.ConfigInt("cost_of_hold", 500),
			c.Universe.ConfigInt("cost_of_fighter", 98),
			turnPrice)
		c.printf("%s", Reset)
	})
}

// ShowRecentNews is the [W] report: re-show recent news the player may have
// scrolled past.
func (c *ConsoleUI) ShowRecentNews() {
	var news []string
	c.Universe.Do(func() {
		news = c.Universe.RecentNews(c.Player.Id, 20)
	})
	if len(news) == 0 {
		c.printf("%sNothing has happened to you recently.%s\n", LightCyan, Reset)
		return
	}
	PrintNews(c.Term, "Recent transmissions:", news)
}

// rankingsCap bounds the leaderboard display; the rest of the field is
// summarized as a count so the list can't grow into hundreds of rows of
// dormant players. The viewer's own row is always shown, even below the cap.
const rankingsCap = 20

func (c *ConsoleUI) ShowRankings() {
	now := galwar.Now()
	c.Universe.Do(func() {
		ranks := c.Universe.RankedPlayers(now)
		c.printf("\n%s   Rank  Trader                          Net Worth\n", Green)
		c.printf("   ====  ==============================  ============%s\n", White)
		row := func(rank int, r galwar.Ranking) {
			tag := ""
			if r.Dormant {
				tag = " (dormant)"
			}
			c.printf("   %4d  %-30s %12d%s\n", rank, r.Name, r.Value, tag)
		}
		shown := len(ranks)
		if shown > rankingsCap {
			shown = rankingsCap
		}
		for i := 0; i < shown; i++ {
			row(i+1, ranks[i])
		}
		if len(ranks) > rankingsCap {
			c.printf("%s   ( %d more not displayed )%s\n", Cyan, len(ranks)-rankingsCap, White)
			// the viewer always sees where they stand
			for i := rankingsCap; i < len(ranks); i++ {
				if ranks[i].Id == c.Player.Id {
					row(i+1, ranks[i])
					break
				}
			}
		}
		c.printf("%s", Reset)
	})
}

// ExecuteReport lets a player report another for sysop review.
func (c *ConsoleUI) ExecuteReport() {
	target := c.PromptString("Report which trader (handle)? ")
	if c.Terminated || strings.TrimSpace(target) == "" {
		return
	}
	reason := c.PromptString("Reason? ")
	if c.Terminated {
		return
	}
	err := c.Universe.DoErr(func() error {
		return c.Universe.FileReport(c.Player, target, reason)
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printf("%sReport filed. Thank you - a sysop will review it.%s\n", LightGreen, Reset)
}

// ExecuteSysop is the hidden admin menu (the original's E editor command,
// gated by allowremote). Available only to configured admins.
func (c *ConsoleUI) ExecuteSysop() {
	var admin bool
	c.Universe.Do(func() {
		admin = c.Universe.IsAdmin(c.Player)
	})
	if !admin {
		c.printf("%sAccess denied.%s\n", LightRed, Reset)
		return
	}
	for !c.Terminated {
		cmd := strings.ToLower(c.PromptString("\n" + LightMagenta + "Sysop (?=Help) ? " + Reset))
		switch cmd {
		case "l":
			c.sysopListReports()
		case "b":
			c.sysopBan(true)
		case "u":
			c.sysopBan(false)
		case "r":
			c.sysopRename()
		case "a":
			c.sysopAudit()
		case "q", "":
			return
		case "?":
			c.printf("%s[L]%s List open reports   %s[B]%s Ban    %s[U]%s Unban\n", Cyan, Reset, Cyan, Reset, Cyan, Reset)
			c.printf("%s[R]%s Force-rename        %s[A]%s Audit log   %s[Q]%s Quit\n", Cyan, Reset, Cyan, Reset, Cyan, Reset)
		}
	}
}

func (c *ConsoleUI) sysopListReports() {
	c.Universe.Do(func() {
		reports := c.Universe.OpenReports()
		if len(reports) == 0 {
			c.printf("%sNo open reports.%s\n", LightGreen, Reset)
			return
		}
		c.printf("\n%sOpen reports:%s\n", Yellow, Reset)
		for _, r := range reports {
			c.printf("%s  %s reported %s: %s%s\n", LightCyan, r.Reporter, r.Target, r.Reason, Reset)
		}
	})
}

func (c *ConsoleUI) sysopBan(ban bool) {
	verb := "ban"
	if !ban {
		verb = "unban"
	}
	handle := c.PromptString(fmt.Sprintf("Handle to %s? ", verb))
	if c.Terminated || strings.TrimSpace(handle) == "" {
		return
	}
	err := c.Universe.DoErr(func() error {
		return c.Universe.SetBanned(c.Player, handle, ban)
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.Universe.Do(func() { c.Universe.ResolveReports(handle) })
	c.printf("%sDone.%s\n", LightGreen, Reset)
}

func (c *ConsoleUI) sysopRename() {
	handle := c.PromptString("Handle to rename? ")
	if c.Terminated || strings.TrimSpace(handle) == "" {
		return
	}
	newName := c.PromptString("New handle? ")
	if c.Terminated {
		return
	}
	err := c.Universe.DoErr(func() error {
		return c.Universe.ForceRename(c.Player, handle, newName)
	})
	if err != nil {
		c.PrintError(err)
		return
	}
	c.printf("%sRenamed.%s\n", LightGreen, Reset)
}

func (c *ConsoleUI) sysopAudit() {
	c.Universe.Do(func() {
		audit := c.Universe.Audit
		c.printf("\n%sRecent admin/security events:%s\n", Yellow, Reset)
		start := 0
		if len(audit) > 15 {
			start = len(audit) - 15
		}
		for _, a := range audit[start:] {
			c.printf("%s  %s: %s %s%s\n", LightCyan, a.Actor, a.Action, a.Detail, Reset)
		}
	})
}

func (c *ConsoleUI) ExecuteCommand() {
	// yellow prompt, white input echo, as in play_game (TWARS.PAS:1389-1392)
	command := strings.ToLower(c.PromptString("\n" + Yellow + "Main Command (?=Help) ? " + White))
	c.printf("%s", Reset)
	switch command {
	case "?":
		c.ExecuteHelp()
	case "a":
		c.ExecuteAttack()
	case "b":
		c.ExecuteUseDevice()
	case "c":
		c.ExecuteComputer()
	case "d":
		c.ExecuteBattleGroup(galwar.MINES)
	case "f":
		c.ExecuteBattleGroup(galwar.FIGHTERS)
	case "g":
		c.ExecuteLaunchGroup()
	case "h":
		c.ExecuteDamageControl()
	case "j":
		c.ExecuteGenesis()
	case "m":
		c.ExecuteMove()
	case "i":
		c.ExecuteInfo()
	case "l":
		c.ExecuteLand()
	case "p":
		c.DockPort()
	case "pass":
		c.ExecuteSetPassword()
	case "report":
		c.ExecuteReport()
	case "sysop":
		c.ExecuteSysop()
	case "q":
		c.Terminated = true
	case "s":
		c.ExecuteScan()
	case "w":
		c.ExecutePlasma()
	case "y":
		c.ExecuteAutopilot()
	case "z":
		c.ExecuteGuide()
	}
}

// isDead reports whether the player has been killed (by another player's
// command) since the UI last checked. Used to bail out of multi-prompt
// interactions like docking the moment the ship is destroyed.
func (c *ConsoleUI) isDead() bool {
	var dead bool
	c.Universe.Do(func() {
		dead = c.Player.IsDead()
	})
	return dead
}

func (c *ConsoleUI) Run() {
	for !c.Terminated {
		// deliver any news that arrived since the last prompt, so an online
		// player learns about attacks, kills, and revolts on their own
		// rhythm rather than only at next login. This runs on the player's
		// goroutine between commands, so it never garbles half-typed input.
		var dead bool
		var news []string
		c.Universe.Do(func() {
			dead = c.Player.IsDead()
			news = c.Universe.TakeNews(c.Player.Id)
		})
		PrintNews(c.Term, "Incoming transmission:", news)
		if dead {
			c.printf("\n%sYour ship has been destroyed. The Traders Guild will reconstruct you tomorrow.%s\n", LightRed, Reset)
			c.Terminated = true
			break
		}
		c.DisplaySector(c.Player.Sector)
		c.ExecuteCommand()
		c.printf("\n")
	}
}
