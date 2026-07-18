package consoleui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// These tests drive the command loop and individual handlers through the
// scripted-terminal harness (harness_test.go). They are the payoff the
// harness exists for: exercising ExecuteCommand dispatch and the multi-prompt
// handlers the way a player would, and asserting on the transcript.

// TestRunLoopQuits confirms the main loop shows the sector, dispatches Q, and
// terminates rather than reading past the script into a spin.
func TestRunLoopQuits(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Quitter", "q@x.com", 5, 5000)

	ui, term := session(t, u, p, "q")
	ui.Run()

	if !ui.Terminated {
		t.Errorf("Q did not terminate the session")
	}
	if term.eofs != 0 {
		t.Errorf("loop read past the script %d times (spin risk)", term.eofs)
	}
	// the loop displays the sector before prompting
	term.wants(t, "Main Command", "Sector: 5")
}

// TestRunLoopEndsOnDisconnect confirms an exhausted script (EOF/disconnect)
// ends the loop instead of looping forever.
func TestRunLoopEndsOnDisconnect(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Ghost", "g@x.com", 5, 5000)

	ui, term := session(t, u, p) // empty script -> immediate EOF at the prompt
	ui.Run()

	if !ui.Terminated {
		t.Errorf("disconnect did not terminate the session")
	}
	if term.eofs > 1 {
		t.Errorf("loop kept reading after disconnect: %d EOFs", term.eofs)
	}
}

// TestRunDispatchesInfoAndHelp drives two commands through the real loop and
// checks both handlers rendered before the quit.
func TestRunDispatchesInfoAndHelp(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Browser", "b@x.com", 5, 5000)

	ui, term := session(t, u, p, "i", "?", "q")
	ui.Run()

	term.wants(t, "Name:", "Browser", "Credits:") // [I] info
	term.wants(t, "Implemented:")                 // [?] help legend
}

// TestExecuteInfoShowsIdentity checks the info screen names the player, their
// credits, and their non-cargo systems.
func TestExecuteInfoShowsIdentity(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Reporter", "r@x.com", 5, 12345)

	ui, term := session(t, u, p)
	ui.ExecuteInfo()

	term.wants(t, "Reporter", "12345", "Fighters")

	// the label column must line up: every row's first colon at the same
	// index, including the long device names ("Anti-Cloaking Device")
	col := -1
	for _, line := range strings.Split(term.text(), "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		if col == -1 {
			col = idx
		}
		if idx != col {
			t.Errorf("misaligned label column (%d != %d): %q", idx, col, line)
		}
	}
}

// TestExecuteMoveToNeighbor moves the ship to a real adjacent sector and
// confirms the engine actually relocated it.
func TestExecuteMoveToNeighbor(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Pilot", "p@x.com", 20, 5000)
	p.SetQuantity(galwar.TURNS, 100)

	warps := u.Sectors[20].Warps
	if len(warps) == 0 {
		t.Fatalf("sector 20 has no warps to move to")
	}
	dest := warps[0]

	ui, term := session(t, u, p, strconv.Itoa(dest))
	ui.ExecuteMove()

	if p.Sector != dest {
		t.Errorf("ship at sector %d, expected %d after move\n%s", p.Sector, dest, term.text())
	}
	term.wants(t, "Warps lead to:")
}

// TestExecuteMoveAborts confirms a bare-Enter/Q at the sector prompt cancels
// the move without an error and without spending a turn.
func TestExecuteMoveAborts(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Stayer", "s@x.com", 20, 5000)
	p.SetQuantity(galwar.TURNS, 100)
	before := p.Sector
	turns := p.GetQuantity(galwar.TURNS)

	ui, _ := session(t, u, p, "q")
	ui.ExecuteMove()

	if p.Sector != before {
		t.Errorf("ship moved despite aborting: %d -> %d", before, p.Sector)
	}
	if p.GetQuantity(galwar.TURNS) != turns {
		t.Errorf("aborted move still spent a turn")
	}
}

// TestExecuteScanShowsNeighbors confirms the S command renders each adjacent
// sector inside its frame.
func TestExecuteScanShowsNeighbors(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Scanner", "sc@x.com", 20, 5000)

	ui, term := session(t, u, p)
	ui.ExecuteScan()

	out := term.text()
	if !strings.Contains(out, "[-------------------------------------]") {
		t.Errorf("scan frame missing:\n%s", out)
	}
	for _, w := range u.Sectors[20].Warps {
		if !strings.Contains(out, "Sector: "+strconv.Itoa(w)) {
			t.Errorf("scan did not show neighbor sector %d:\n%s", w, out)
		}
	}
}

// TestAutopilotAbortsQueueOnError replays a live-play find: engage the
// autopilot on a multi-hop course with no turns left. The first queued move
// fails; the rest of the course must be discarded, not replayed from the
// wrong sector ("You cannot go to that sector!" spam).
func TestAutopilotAbortsQueueOnError(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Jetsam", "j@x.com", 20, 5000)
	p.SetQuantity(galwar.TURNS, 0)

	// a destination two-plus hops out, so the course queues at least two moves
	dest := 0
	for s := 1; s <= 60 && dest == 0; s++ {
		if path := u.ShortestPathTo(20, s); len(path) >= 3 {
			dest = s
		}
	}
	if dest == 0 {
		t.Fatalf("no multi-hop destination found from sector 20")
	}

	ui, term := session(t, u, p, "y", strconv.Itoa(dest), "y")
	ui.Run()

	term.wants(t, "turns left", "discarded")
	term.rejects(t, "You cannot go to that sector")
	if p.Sector != 20 {
		t.Errorf("ship moved to %d despite having no turns", p.Sector)
	}
}

// TestBankDepositThenWithdraw exercises the banking dialog end to end.
func TestBankDepositThenWithdraw(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Saver", "sv@x.com", 5, 10000)

	ui, term := session(t, u, p, "d", "4000", "w", "1500", "q")
	ui.DockBank()

	if p.BankBalance != 2500 {
		t.Errorf("bank balance %d, expected 2500 (deposit 4000, withdraw 1500)", p.BankBalance)
	}
	if p.GetMoney() != 7500 {
		t.Errorf("credits aboard %d, expected 7500", p.GetMoney())
	}
	term.wants(t, "Account balance", "Interstel Galactic Banking")
}

// TestBankRejectsOverdraft confirms withdrawing more than the balance is
// refused with the engine's error and leaves the account intact.
func TestBankRejectsOverdraft(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Overdrawn", "od@x.com", 5, 10000)

	ui, _ := session(t, u, p, "d", "1000", "w", "5000", "q")
	ui.DockBank()

	if p.BankBalance != 1000 {
		t.Errorf("balance %d, expected the overdraft to be refused (1000)", p.BankBalance)
	}
}

// TestSetPasswordMatch sets a telnet password through the paired-entry flow.
func TestSetPasswordMatch(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Secured", "se@x.com", 5, 5000)

	ui, term := session(t, u, p, "hunter2", "hunter2")
	ui.ExecuteSetPassword()

	term.wants(t, "Password set")
	if ui.Terminated {
		t.Errorf("setting a password ended the session")
	}
}

// TestSetPasswordMismatch confirms mismatched entries are rejected and no
// password is stored.
func TestSetPasswordMismatch(t *testing.T) {
	u := displayUniverse(t)
	p := addShip(t, u, "Fumbled", "fu@x.com", 5, 5000)

	ui, term := session(t, u, p, "alpha", "beta")
	ui.ExecuteSetPassword()

	term.wants(t, "do not match")
	term.rejects(t, "Password set")
}
