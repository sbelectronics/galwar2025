package consoleui

import (
	"fmt"
	"github.com/sbelectronics/galwar/pkg/galwar"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ConsoleUI struct {
	Player     *galwar.Player
	Terminated bool
	wg         *sync.WaitGroup
	input      string
}

func NewConsoleUI(player *galwar.Player) *ConsoleUI {
	return &ConsoleUI{
		Player: player,
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

func (c *ConsoleUI) GetInput() string {
	scanned := false
	if c.input == "" {
		fmt.Scanln(&c.input)
		c.input = strings.ToLower(c.input)
		scanned = true
	}

	if idx := strings.Index(c.input, ";"); idx != -1 {
		result := c.input[:idx]
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
	for {
		fmt.Printf("%s", prompt)
		input := c.GetInput()
		if input == "y" || input == "Y" {
			return true
		} else if input == "n" || input == "N" || input == "" {
			return false
		}
	}
}

func (c *ConsoleUI) PromptInt(prompt string) int {
	for {
		fmt.Printf("%s", prompt)
		input := c.GetInput()

		result, err := strconv.Atoi(input)
		if err == nil {
			return result
		}
	}
}

func (c *ConsoleUI) PromptIntDefault(prompt string, def int) int {
	for {
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
}

func (c *ConsoleUI) GetWarpStrings(sector galwar.SectorInterface) []string {
	warpStrings := []string{}
	for _, warp := range sector.GetWarps() {
		warpStrings = append(warpStrings, fmt.Sprintf("%d", warp))
	}
	return warpStrings
}

func (c *ConsoleUI) DisplaySector(sector galwar.SectorInterface) {
	fmt.Printf("Sector: %d\n", sector.GetNumber())

	objs := galwar.Universe.GetObjectsInSector(sector.GetNumber(), "")
	for _, obj := range objs {
		if obj == c.Player {
			// don't show yourself
			continue
		}
		extra := obj.GetNameExtra()
		if extra != "" {
			fmt.Printf("%s: %s, %s\n", obj.GetType(), obj.GetName(), obj.GetNameExtra())
		} else {
			fmt.Printf("%s: %s\n", obj.GetType(), obj.GetName())
		}
	}

	fmt.Printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(sector), ", "))
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
	sector := &galwar.Sectors[c.Player.Sector]
	fmt.Printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(sector), ", "))

	secnum := c.PromptInt("\nMove to what sector? ")

	if !sector.HasWarp(secnum) {
		fmt.Printf("You cannot go to that sector!\n")
		return
	}
	c.Player.MoveTo(secnum)
}

func (c *ConsoleUI) ExecuteScan() {
	sector := &galwar.Sectors[c.Player.Sector]

	fmt.Printf("\n")
	fmt.Printf("[-------------------------------------]\n")

	for _, warp := range sector.GetWarps() {
		adjSector := &galwar.Sectors[warp]

		c.DisplaySector(adjSector)

		fmt.Printf("\n")
	}

	fmt.Printf("[-------------------------------------]\n")
}

func (c *ConsoleUI) ExecuteAutopilot() {
	sec := c.PromptInt("\nWhat sector do you wish to go to ? ")

	if sec == c.Player.Sector {
		fmt.Printf("You are in that sector!\n")
		return
	}

	path := galwar.Sectors[c.Player.Sector].ShortestPathTo(sec)

	pathStrings := []string{}
	for _, pathSec := range path {
		pathStrings = append(pathStrings, fmt.Sprintf("%d", pathSec))
	}

	fmt.Printf("The shortest path from sector %d to sector %d is: %s\n", c.Player.Sector, sec, strings.Join(pathStrings, ","))

	commit := c.PromptBool("\nEnter course into autopilot(Y/N) ?")
	if commit {
		pathStrings = []string{}
		for _, pathSec := range path {
			pathStrings = append(pathStrings, fmt.Sprintf("m;%d", pathSec))
		}
		c.input = strings.Join(pathStrings, ";")
	}
}

func (c *ConsoleUI) DockSolPort(port *galwar.Port) {
	fmt.Printf("Commerce Report For %s: %s\n", port.GetName(), time.Now().Format("2006-01-02 15:04:05"))
	fmt.Print("\n")

	fmt.Printf("##  Item name               Cost      Can Afford\n")
	fmt.Printf("--  ----------------------  --------  ----------\n")

	choices := map[string]*galwar.Commodity{}
	for i, cm := range port.Inventory {
		canAfford := int(math.Floor(float64(c.Player.GetMoney()) / cm.SellPrice))
		fmt.Printf("%2d  %-22s %9d %11d\n", i+1, cm.Name, int(cm.GetPrice()), canAfford)
		choices[fmt.Sprintf("%d", i+1)] = cm
	}

	for {
		input := c.PromptString("\nEnter number to buy or <Q> to quit > ")
		if input == "q" {
			return
		}
		commodity, exists := choices[input]
		if !exists {
			fmt.Printf("Invalid choice. Please try again.\n")
			continue
		}
		canAfford := int(math.Floor(float64(c.Player.GetMoney()) / commodity.SellPrice))
		qty := c.PromptInt(fmt.Sprintf("\nYou can afford %d %s. How many do you want? ", canAfford, commodity.Name))
		if qty < 0 {
			break
		}
		if qty > canAfford {
			fmt.Printf("You cannot afford that many.\n")
			break
		}
		galwar.TradeBuyNoLimit(commodity, c.Player, qty)
	}
}

func (c *ConsoleUI) DockPort() {
	ports := galwar.Universe.GetObjectsInSector(c.Player.Sector, galwar.TYPE_PORT)
	if len(ports) == 0 {
		fmt.Printf("No ports in this sector\n")
		return
	}
	port, ok := ports[0].(*galwar.Port)
	if !ok {
		// This should never happen, but just in case
		fmt.Printf("Error: Object in sector is not a Port\n")
		return
	}

	if port.Goods == galwar.Sol {
		c.DockSolPort(port)
		return
	}

	fmt.Printf("Commerce Report For %s: %s\n", port.GetName(), time.Now().Format("2006-01-02 15:04:05"))
	fmt.Print("\n")
	fmt.Printf(" Items     Status    Price  # units  In holds\n")
	fmt.Printf(" =====     ======    =====  =======  ========\n")

	for _, cm := range port.Inventory {
		fmt.Printf("%-10s %-9s %5.2f %8d %9d\n", cm.Name, cm.GetBuySell(), cm.GetPrice(), cm.Quantity, c.Player.GetQuantity(cm.Name))
	}

	for _, cm := range port.Inventory {
		if !cm.Sell {
			for {
				buyAllow := min(c.Player.GetQuantity(cm.Name), cm.Quantity)
				fmt.Printf("\nWe are buying up to %d of %s. You have %d in your holds.\n", cm.Quantity, cm.Name, c.Player.GetQuantity(cm.Name))
				input := c.PromptIntDefault(fmt.Sprintf("How many holds of %s do you want to sell [%d] ? ", cm.Name, buyAllow), buyAllow)
				err := galwar.TradeSell(cm.Name, port, c.Player, input)
				if err == nil {
					break
				}
				c.PrintError(err)
			}
		}
	}

	for _, cm := range port.Inventory {
		if cm.Sell {
			for {
				sellAllow := min(c.Player.GetFreeHolds(), cm.Quantity, int(math.Floor(float64(c.Player.GetMoney())/cm.SellPrice)))
				fmt.Printf("\nWe are selling up to %d of %s. You have %d in your holds.\n", cm.Quantity, cm.Name, c.Player.GetQuantity(cm.Name))
				input := c.PromptIntDefault(fmt.Sprintf("How many holds of %s do you want to buy [%d] ? ", cm.Name, sellAllow), sellAllow)
				if input > c.Player.GetFreeHolds() {
					fmt.Printf("You don't have enough free holds.\n")
					continue
				}
				err := galwar.TradeBuy(cm.Name, port, c.Player, input)
				if err == nil {
					break
				}
				c.PrintError(err)
			}
		}
	}
}

func (c *ConsoleUI) ExecuteInfo() {
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
}

func (c *ConsoleUI) ExecuteBattleGroup(kind string) {
	total := c.Player.GetQuantity(kind)
	bg, err := galwar.Battlegroups.GetBattlegroup(c.Player, c.Player.Sector, false)
	if err != nil {
		c.PrintError(err)
		return
	}
	if bg != nil {
		total += bg.GetQuantity(kind)
	}
	amount := c.PromptInt(fmt.Sprintf("You have %d total %s. How many do you want to defend this sector? ", total, kind))
	err = galwar.Battlegroups.AdjustBattlegroup(c.Player, c.Player.Sector, kind, amount)
	if err != nil {
		c.PrintError(err)
		return
	}
}

func (c *ConsoleUI) ExecuteGenesis() {
	name := c.PromptString("Enter the name of your new planet: ")
	err := galwar.Planets.UseGenesisDevice(c.Player, c.Player.Sector, name)
	if err != nil {
		c.PrintError(err)
		return
	}
}

func (c *ConsoleUI) PlanetReport(planet *galwar.Planet) {
	fmt.Printf("Planet report For %s: %s\n", planet.GetName(), time.Now().Format("2006-01-02 15:04:05"))
	fmt.Print("\n")
	fmt.Printf(" Items      Prod     # units  In holds\n")
	fmt.Printf(" =====     ======    =======  ========\n")

	for _, cm := range planet.Inventory {
		fmt.Printf("%-10s %6d %10d %9d\n", cm.Name, cm.Prod, cm.Quantity, c.Player.GetQuantity(cm.Name))
	}
}

func (c *ConsoleUI) ExecutePlanetTakeCargo(commodityName string) {
	planet, err := galwar.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST)
	if err != nil {
		c.PrintError(err)
		return
	}
	wanted := min(c.Player.GetFreeHolds(), planet.GetQuantity(commodityName))
	amount := c.PromptIntDefault(fmt.Sprintf("Take how much %s [%d] ? ", commodityName, wanted), wanted)
	err = galwar.Planets.TransferOut(c.Player, c.Player.Sector, commodityName, amount)
	if err != nil {
		c.PrintError(err)
		return
	}
}

func (c *ConsoleUI) ExecutePlanetPutCargo() {
	input := c.PromptBool("\nTransfer your cargo to planet (Y/N) ? ")
	if !input {
		return
	}

	err := galwar.Planets.TransferIn(c.Player, c.Player.Sector)
	if err != nil {
		c.PrintError(err)
		return
	}
}

func (c *ConsoleUI) ExecutePlanetTransfer(commodityName string) {
	planet, err := galwar.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST)
	if err != nil {
		c.PrintError(err)
		return
	}
	total := c.Player.GetQuantity(commodityName) + planet.GetQuantity(commodityName)
	amount := c.PromptIntDefault(fmt.Sprintf("You have %d %s available, how many to leave on planet? ", total, commodityName), 0)
	err = galwar.Planets.TransferSet(c.Player, c.Player.Sector, commodityName, amount)
	if err != nil {
		c.PrintError(err)
		return
	}
}

func (c *ConsoleUI) ExecuteLand() {
	first := true
	for {
		planet, err := galwar.Planets.GetPlanet(c.Player, c.Player.Sector, galwar.MUST_EXIST)
		if err != nil {
			c.PrintError(err)
			return
		}

		if first {
			c.PlanetReport(planet)
			first = false
		}
		command := c.PromptString("\nPlanet Command (?=Help) ? ")

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
	command := c.PromptString("\nMain Command (?=Help) ? ")
	switch command {
	case "?":
		c.ExecuteHelp()
	case "d":
		c.ExecuteBattleGroup("Mines")
	case "f":
		c.ExecuteBattleGroup("Fighters")
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
	defer c.wg.Done()
	for !c.Terminated {
		c.DisplaySector(&galwar.Sectors[c.Player.Sector])
		c.ExecuteCommand()
		fmt.Print("\n")
	}
}

func (c *ConsoleUI) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	c.wg = wg
	go c.Run()
}
