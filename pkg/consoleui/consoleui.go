package consoleui

import (
	"fmt"
	"github.com/sbelectronics/galwar/pkg/galwar"
	"strconv"
	"strings"
	"sync"
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
	fmt.Printf("\n%s", prompt)
	return c.GetInput()
}

func (c *ConsoleUI) PromptBool(prompt string) bool {
	for {
		fmt.Printf("\n%s", prompt)
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
		fmt.Printf("\n%s", prompt)
		input := c.GetInput()

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
	fmt.Printf("Implemented Commands: M, Q, S, Y\n")
}

func (c *ConsoleUI) ExecuteMove() {
	sector := &galwar.Sectors[c.Player.Sector]
	fmt.Printf("Warps lead to: %s\n", strings.Join(c.GetWarpStrings(sector), ", "))

	secnum := c.PromptInt("Move to what sector? ")

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
	sec := c.PromptInt("What sector do you wish to go to ? ")

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

	commit := c.PromptBool("Enter course into autopilot(Y/N) ?")
	if commit {
		pathStrings = []string{}
		for _, pathSec := range path {
			pathStrings = append(pathStrings, fmt.Sprintf("m;%d", pathSec))
		}
		c.input = strings.Join(pathStrings, ";")
	}
}

func (c *ConsoleUI) ExecuteCommand() {
	command := c.PromptString("Main Command (?=Help) ? ")
	switch command {
	case "?":
		c.ExecuteHelp()
	case "m":
		c.ExecuteMove()
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
