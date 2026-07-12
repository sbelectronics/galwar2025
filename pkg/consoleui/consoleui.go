package consoleui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

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
	// the original showed rule violations in light red throughout
	c.printf("%s%s%s\n", LightRed, ErrText(err), Reset)
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

func (c *ConsoleUI) PromptInt(prompt string) int {
	for !c.Terminated {
		c.printf("%s", prompt)
		input := c.GetInput()

		result, err := strconv.Atoi(input)
		if err == nil {
			return result
		}
	}
	return 0
}

func (c *ConsoleUI) PromptIntDefault(prompt string, def int) int {
	for !c.Terminated {
		c.printf("%s", prompt)
		input := c.GetInput()

		if input == "" {
			return def
		}

		result, err := strconv.Atoi(input)
		if err == nil {
			return result
		}
	}
	return def
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

func (c *ConsoleUI) DisplaySector(secnum int) {
	now := time.Now()
	c.Universe.Do(func() {
		c.printf("%sSector: %d%s\n", LightRed, secnum, Reset)

		objs := c.Universe.GetVisibleObjectsInSector(secnum, "", now)
		for _, obj := range objs {
			if obj == c.Player {
				// don't show yourself
				continue
			}
			color := objectColor(obj.GetType())
			extra := obj.GetNameExtra()
			if extra != "" {
				c.printf("%s%s: %s, %s%s\n", color, obj.GetType(), obj.GetName(), extra, Reset)
			} else {
				c.printf("%s%s: %s%s\n", color, obj.GetType(), obj.GetName(), Reset)
			}
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
		"        [D] Drop Mine          [C] Computer        [L] Land on planet       ",
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
	c.printf("%s\n", HelpLine("Implemented: A, C, D, F, H, I, J, L, M, P, Q, S, W, Y  (plus [PASS], [REPORT], [SYSOP])"))
}

func (c *ConsoleUI) ExecuteMove() {
	warps := c.getWarps(c.Player.Sector)
	c.printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(warps), ", "))

	secnum := c.PromptInt("\nMove to what sector? ")
	if c.Terminated {
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
	target := c.PromptInt("Which sector? ")
	if c.Terminated {
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
	n := c.PromptInt(fmt.Sprintf("You have %d pulsar bombs. Drop how many? ", have))
	if c.Terminated || n < 1 {
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
	now := time.Now()
	c.Universe.Do(func() {
		for _, obj := range c.Universe.GetVisibleObjectsInSector(c.Player.Sector, galwar.TYPE_PLAYER, now) {
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
	choice := c.PromptInt("\nAttack which ship (0=abort) ? ")
	if c.Terminated || choice < 1 || choice > len(targets) {
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
	commit := c.PromptIntDefault(fmt.Sprintf("How many fighters do you wish to use [%d] ? ", fighters), fighters)
	if c.Terminated {
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
	sec := c.PromptInt("\nWhat sector do you wish to go to ? ")
	if c.Terminated {
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

func (c *ConsoleUI) DockSolPort(port *galwar.Port) {
	choices := map[string]*galwar.Commodity{}

	c.Universe.Do(func() {
		// commerce report: yellow title, green headers, white rows
		// (TWARS.PAS port procedure)
		c.printf("%sCommerce Report For %s: %s%s\n", Yellow, port.GetName(), time.Now().Format("2006-01-02 15:04:05"), Reset)
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

		qty := c.PromptInt(fmt.Sprintf("\nYou can afford %d %s. How many do you want? ", canAfford, commodity.Name))
		if qty <= 0 {
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

	if port.Goods == galwar.Sol {
		c.DockSolPort(port)
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
		c.printf("%sCommerce Report For %s: %s%s\n", Yellow, port.GetName(), time.Now().Format("2006-01-02 15:04:05"), Reset)
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
			input := c.PromptIntDefault(fmt.Sprintf("How many holds of %s do you want to sell [%d] ? ", item.name, buyAllow), buyAllow)
			if c.Terminated {
				return
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
			input := c.PromptIntDefault(fmt.Sprintf("How many holds of %s do you want to buy [%d] ? ", item.name, sellAllow), sellAllow)
			if c.Terminated {
				return
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

func (c *ConsoleUI) ExecuteInfo() {
	// info screen: cyan labels, white values (TWARS.PAS:148-198)
	c.Universe.Do(func() {
		c.printf("\n")
		c.printf("%s           Name: %s%s\n", Cyan, White, c.Player.GetName())
		c.printf("%s        Credits: %s%d\n", Cyan, White, c.Player.GetMoney())
		c.printf("%s          Cargo:%s", Cyan, White)
		for _, cm := range c.Player.Inventory {
			if cm.IsCargo() {
				c.printf(" %s: %d", cm.GetShortName(), cm.Quantity)
			}
		}
		c.printf("\n")
		for _, cm := range c.Player.Inventory {
			if !cm.IsCargo() {
				c.printf("%s%15s: %s%d\n", Cyan, cm.Name, White, cm.Quantity)
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

	amount := c.PromptInt(fmt.Sprintf("You have %d total %s. How many do you want to defend this sector? ", total, kind))
	if c.Terminated {
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
		c.printf("%sPlanet report For %s: %s%s\n", Yellow, planet.GetName(), time.Now().Format("2006-01-02 15:04:05"), Reset)
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

	amount := c.PromptIntDefault(fmt.Sprintf("Take how much %s [%d] ? ", commodityName, wanted), wanted)
	if c.Terminated {
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

	amount := c.PromptIntDefault(fmt.Sprintf("You have %d %s available, how many to leave on planet? ", total, commodityName), 0)
	if c.Terminated {
		return
	}

	err = c.Universe.DoErr(func() error {
		return c.Universe.TransferSet(c.Player, c.Player.Sector, commodityName, amount)
	})
	if err != nil {
		c.PrintError(err)
	}
}

func (c *ConsoleUI) ExecuteLand() {
	if err := c.Universe.DoErr(func() error {
		return c.Universe.CheckSystem(c.Player, galwar.SysThrusters)
	}); err != nil {
		c.PrintError(err)
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
// sub-reports. Currently offers player rankings.
func (c *ConsoleUI) ExecuteComputer() {
	for !c.Terminated {
		cmd := strings.ToLower(c.PromptString("\n" + Yellow + "Computer (?=Help) ? " + Reset))
		switch cmd {
		case "l":
			c.ShowRankings()
		case "q", "":
			return
		case "?":
			c.printf("%s[L]%s Rank the greatest warlords\n", Cyan, Reset)
			c.printf("%s[Q]%s Quit the computer\n", Cyan, Reset)
		}
	}
}

func (c *ConsoleUI) ShowRankings() {
	now := time.Now()
	c.Universe.Do(func() {
		ranks := c.Universe.RankedPlayers(now)
		c.printf("\n%s   Rank  Trader                          Net Worth\n", Green)
		c.printf("   ====  ==============================  ============%s\n", White)
		for i, r := range ranks {
			if i >= 20 {
				break
			}
			tag := ""
			if r.Dormant {
				tag = " (dormant)"
			}
			c.printf("   %4d  %-30s %12d%s\n", i+1, r.Name, r.Value, tag)
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
	case "c":
		c.ExecuteComputer()
	case "d":
		c.ExecuteBattleGroup(galwar.MINES)
	case "f":
		c.ExecuteBattleGroup(galwar.FIGHTERS)
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
