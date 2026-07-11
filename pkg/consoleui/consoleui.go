package consoleui

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// ConsoleUI is a session that drives the game engine from a terminal.
// It follows the actor rules: gather input from the player first, then
// submit one complete operation to the universe via Do/DoErr. Prompts never
// happen while holding the universe.

type ConsoleUI struct {
	Universe   *galwar.UniverseType
	Player     *galwar.Player
	Terminated bool
	scanner    *bufio.Scanner
	input      string
}

func NewConsoleUI(universe *galwar.UniverseType, player *galwar.Player) *ConsoleUI {
	return &ConsoleUI{
		Universe: universe,
		Player:   player,
		scanner:  bufio.NewScanner(os.Stdin),
	}
}

func (c *ConsoleUI) PrintError(err error) {
	gameErr, ok := err.(*galwar.GameError)
	if ok {
		fmt.Printf("%s\n", gameErr.Message())
		return
	}
	fmt.Printf("Error: %s\n", err.Error())
}

// GetInput returns the next input token. Whole lines are read (so names may
// contain spaces); ';' separates chained commands, which is how the
// autopilot queues its course. Case is preserved - command dispatch
// lowercases where it compares. On EOF the session terminates instead of
// spinning.
func (c *ConsoleUI) GetInput() string {
	scanned := false
	if c.input == "" {
		if !c.scanner.Scan() {
			c.Terminated = true
			return ""
		}
		c.input = strings.TrimSpace(c.scanner.Text())
		scanned = true
	}

	if idx := strings.Index(c.input, ";"); idx != -1 {
		result := strings.TrimSpace(c.input[:idx])
		c.input = c.input[idx+1:]
		fmt.Printf("%s\n", result)
		return result
	}

	result := c.input
	c.input = ""

	if !scanned {
		fmt.Printf("%s\n", result)
	}

	return result
}

func (c *ConsoleUI) PromptString(prompt string) string {
	fmt.Printf("%s", prompt)
	return c.GetInput()
}

func (c *ConsoleUI) PromptBool(prompt string) bool {
	for !c.Terminated {
		fmt.Printf("%s", prompt)
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
		fmt.Printf("%s", prompt)
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
		fmt.Printf("%s", prompt)
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

func (c *ConsoleUI) DisplaySector(secnum int) {
	c.Universe.Do(func() {
		fmt.Printf("Sector: %d\n", secnum)

		objs := c.Universe.GetObjectsInSector(secnum, "")
		for _, obj := range objs {
			if obj == c.Player {
				// don't show yourself
				continue
			}
			extra := obj.GetNameExtra()
			if extra != "" {
				fmt.Printf("%s: %s, %s\n", obj.GetType(), obj.GetName(), extra)
			} else {
				fmt.Printf("%s: %s\n", obj.GetType(), obj.GetName())
			}
		}

		warps := c.Universe.Sectors[secnum].GetWarps()
		fmt.Printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(warps), ", "))
	})
}

func (c *ConsoleUI) ExecuteHelp() {
	fmt.Printf("             [COMBAT]             [TACTICAL]          [MOVEMENT]            \n")
	fmt.Printf("\n")
	fmt.Printf("        [A] Attack Player      [S] Sensor Scan     [M] Move to Sector       \n")
	fmt.Printf("        [D] Drop Mine          [C] Computer        [L] Land on planet       \n")
	fmt.Printf("        [F] Take/Leave fgtrs   [I] Your info       [P] Dock at port         \n")
	fmt.Printf("        [G] Launch Group       [B] Use Device      [Y] Engage Autopilot     \n")
	fmt.Printf("                               [H] Damage Control  [R] Starbase Transporter \n")
	fmt.Printf("\n")
	fmt.Printf("             [HELP]               [MISC]              [PLANETS]             \n")
	fmt.Printf("\n")
	fmt.Printf("        [?] This menu          [V] Record Macro    [J] Create Planet        \n")
	fmt.Printf("        [Z] Instructions       [W] Plasma device   [O] <Removed>            \n")
	fmt.Printf("                               [Q] Quit to bbs     [U] Starbase maint       \n")
	fmt.Printf("                               [T] Team Menu                                \n")
	fmt.Printf("\n")
	fmt.Printf("Implemented Commands: D, F, I, J, L, M, P, Q, S, Y\n")
}

func (c *ConsoleUI) ExecuteMove() {
	warps := c.getWarps(c.Player.Sector)
	fmt.Printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(warps), ", "))

	secnum := c.PromptInt("\nMove to what sector? ")
	if c.Terminated {
		return
	}

	err := c.Universe.DoErr(func() error {
		return c.Universe.MovePlayer(c.Player, secnum)
	})
	if err != nil {
		c.PrintError(err)
	}
}

func (c *ConsoleUI) ExecuteScan() {
	warps := c.getWarps(c.Player.Sector)

	fmt.Printf("\n")
	fmt.Printf("[-------------------------------------]\n")

	for _, warp := range warps {
		c.DisplaySector(warp)

		fmt.Printf("\n")
	}

	fmt.Printf("[-------------------------------------]\n")
}

func (c *ConsoleUI) ExecuteAutopilot() {
	sec := c.PromptInt("\nWhat sector do you wish to go to ? ")
	if c.Terminated {
		return
	}

	if sec == c.Player.Sector {
		fmt.Printf("You are in that sector!\n")
		return
	}

	var path []int
	c.Universe.Do(func() {
		path = c.Universe.ShortestPathTo(c.Player.Sector, sec)
	})

	if path == nil {
		fmt.Printf("There is no route from sector %d to sector %d!\n", c.Player.Sector, sec)
		return
	}

	pathStrings := []string{}
	for _, pathSec := range path {
		pathStrings = append(pathStrings, fmt.Sprintf("%d", pathSec))
	}

	fmt.Printf("The shortest path from sector %d to sector %d is: %s\n", c.Player.Sector, sec, strings.Join(pathStrings, ","))

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
		fmt.Printf("Commerce Report For %s: %s\n", port.GetName(), time.Now().Format("2006-01-02 15:04:05"))
		fmt.Print("\n")

		fmt.Printf("##  Item name               Cost      Can Afford\n")
		fmt.Printf("--  ----------------------  --------  ----------\n")

		for i, cm := range port.Inventory {
			canAfford := int(math.Floor(float64(c.Player.GetMoney()) / cm.EffectiveSellPrice()))
			fmt.Printf("%2d  %-22s %9d %11d\n", i+1, cm.Name, int(cm.GetPrice()), canAfford)
			choices[fmt.Sprintf("%d", i+1)] = cm
		}
	})

	for !c.Terminated {
		input := strings.ToLower(c.PromptString("\nEnter number to buy or <Q> to quit > "))
		if input == "q" {
			return
		}
		commodity, exists := choices[input]
		if !exists {
			fmt.Printf("Invalid choice. Please try again.\n")
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
		fmt.Printf("No ports in this sector\n")
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
		fmt.Printf("Commerce Report For %s: %s\n", port.GetName(), time.Now().Format("2006-01-02 15:04:05"))
		fmt.Print("\n")
		fmt.Printf(" Items     Status    Price  # units  In holds\n")
		fmt.Printf(" =====     ======    =====  =======  ========\n")

		for _, cm := range port.Inventory {
			fmt.Printf("%-10s %-9s %5.2f %8d %9d\n", cm.Name, cm.GetBuySell(), cm.GetPrice(), galwar.ScaleUp(c.Player, cm.Quantity), c.Player.GetQuantity(cm.Name))
			items = append(items, tradeItem{name: cm.Name, sell: cm.Sell})
		}
	})

	for _, item := range items {
		if item.sell {
			continue
		}
		for !c.Terminated {
			var portWants, inHolds int
			c.Universe.Do(func() {
				portWants = galwar.ScaleUp(c.Player, port.GetQuantity(item.name))
				inHolds = c.Player.GetQuantity(item.name)
			})
			buyAllow := min(inHolds, portWants)
			fmt.Printf("\nWe are buying up to %d of %s. You have %d in your holds.\n", portWants, item.name, inHolds)
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
			var portHas, inHolds, sellAllow int
			c.Universe.Do(func() {
				cm := port.GetCommodity(item.name)
				portHas = galwar.ScaleUp(c.Player, cm.Quantity)
				inHolds = c.Player.GetQuantity(item.name)
				sellAllow = min(c.Player.GetFreeHolds(), portHas, int(math.Floor(float64(c.Player.GetMoney())/cm.EffectiveSellPrice())))
			})
			fmt.Printf("\nWe are selling up to %d of %s. You have %d in your holds.\n", portHas, item.name, inHolds)
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
	c.Universe.Do(func() {
		fmt.Print("\n")
		fmt.Printf("           Name: %s\n", c.Player.GetName())
		fmt.Printf("        Credits: %d\n", c.Player.GetMoney())
		fmt.Printf("          Cargo:")
		for _, cm := range c.Player.Inventory {
			if cm.IsCargo() {
				fmt.Printf(" %s: %d", cm.GetShortName(), cm.Quantity)
			}
		}
		fmt.Printf("\n")
		for _, cm := range c.Player.Inventory {
			if !cm.IsCargo() {
				fmt.Printf("%15s: %d\n", cm.Name, cm.Quantity)
			}
		}
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

func (c *ConsoleUI) PlanetReport(planet *galwar.Planet) {
	c.Universe.Do(func() {
		fmt.Printf("Planet report For %s: %s\n", planet.GetName(), time.Now().Format("2006-01-02 15:04:05"))
		fmt.Print("\n")
		fmt.Printf(" Items      Prod     # units  In holds\n")
		fmt.Printf(" =====     ======    =======  ========\n")

		for _, cm := range planet.Inventory {
			fmt.Printf("%-10s %6d %10d %9d\n", cm.Name, cm.Prod, cm.Quantity, c.Player.GetQuantity(cm.Name))
		}
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
		case "l":
			return
		case "v":
			first = true
		case "?":
			fmt.Printf("[F] Fighter transfer\n")
			fmt.Printf("[1] Take Ore\n")
			fmt.Printf("[2] Take Organics\n")
			fmt.Printf("[3] Take Equipment\n")
			fmt.Printf("[T] Transfer Cargo to Planet\n")
			fmt.Printf("[L] Leave Planet\n")
			fmt.Printf("[V] View Planet Production\n")
		}
	}
}

func (c *ConsoleUI) ExecuteCommand() {
	command := strings.ToLower(c.PromptString("\nMain Command (?=Help) ? "))
	switch command {
	case "?":
		c.ExecuteHelp()
	case "d":
		c.ExecuteBattleGroup(galwar.MINES)
	case "f":
		c.ExecuteBattleGroup(galwar.FIGHTERS)
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
	case "q":
		c.Terminated = true
	case "s":
		c.ExecuteScan()
	case "y":
		c.ExecuteAutopilot()
	}
}

func (c *ConsoleUI) Run() {
	for !c.Terminated {
		c.DisplaySector(c.Player.Sector)
		c.ExecuteCommand()
		fmt.Print("\n")
	}
}
