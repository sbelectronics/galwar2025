package galwar

import (
	"testing"
	"time"
)

// M13: the Interstel bank.

func bankUniverse(t *testing.T) (*UniverseType, *Player) {
	t.Helper()
	u := NewUniverse()
	u.Generate(50)
	u.SeedDefaultConfig()
	p, err := u.RegisterPlayer("Rich Trader", "rt@x.com", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	u.TouchLastSeen(p, time.Now().Unix())
	return u, p
}

func TestBankDepositWithdraw(t *testing.T) {
	u, p := bankUniverse(t)
	start := p.GetMoney() // 35000

	if err := u.BankDeposit(p, 0); err == nil {
		t.Errorf("zero deposit accepted")
	}
	if err := u.BankDeposit(p, start+1); err == nil {
		t.Errorf("overdraft deposit accepted")
	}
	if err := u.BankDeposit(p, 10000); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if p.GetMoney() != start-10000 || p.BankBalance != 10000 {
		t.Errorf("after deposit: money=%d balance=%d", p.GetMoney(), p.BankBalance)
	}

	if err := u.BankWithdraw(p, 10001); err == nil {
		t.Errorf("over-balance withdrawal accepted")
	}
	if err := u.BankWithdraw(p, 4000); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if p.GetMoney() != start-6000 || p.BankBalance != 6000 {
		t.Errorf("after withdraw: money=%d balance=%d", p.GetMoney(), p.BankBalance)
	}
}

func TestBankInterestAndCap(t *testing.T) {
	u, p := bankUniverse(t)
	now := time.Now()

	p.BankBalance = 100000
	u.creditBankInterest(now)
	if p.BankBalance != 101000 { // +1%
		t.Errorf("interest: balance=%d, want 101000", p.BankBalance)
	}

	// above the cap, only the first bank_interest_cap credits earn
	p.BankBalance = 2000000
	u.creditBankInterest(now)
	if p.BankBalance != 2010000 { // +1% of 1,000,000
		t.Errorf("capped interest: balance=%d, want 2010000", p.BankBalance)
	}
}

func TestBankInterestSkipsDormant(t *testing.T) {
	u, p := bankUniverse(t)
	now := time.Now()
	p.BankBalance = 100000
	u.TouchLastSeen(p, now.Add(-10*24*time.Hour).Unix()) // dormant
	u.creditBankInterest(now)
	if p.BankBalance != 100000 {
		t.Errorf("dormant account compounded: %d", p.BankBalance)
	}
}

func TestBankSurvivesDeath(t *testing.T) {
	u, p := bankUniverse(t)
	now := time.Now()
	if err := u.BankDeposit(p, 20000); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	u.KillPlayer(p, now.Unix())
	if p.BankBalance != 20000 {
		t.Errorf("death touched the bank balance: %d", p.BankBalance)
	}
	// reconstruction the next day: fresh ship, same account
	msg, rebuilt := u.ReconstructIfDue(p, now.Add(48*time.Hour))
	if !rebuilt {
		t.Fatalf("not rebuilt: %q", msg)
	}
	if p.BankBalance != 20000 {
		t.Errorf("reconstruction touched the bank balance: %d", p.BankBalance)
	}
	if p.GetMoney() != u.ConfigInt("starting_credits", 35000) {
		t.Errorf("ship credits not reset to the starting kit: %d", p.GetMoney())
	}
}

func TestBankForfeitedOnExpiry(t *testing.T) {
	u, p := bankUniverse(t)
	now := time.Now()
	p.BankBalance = 50000
	u.TouchLastSeen(p, now.Add(-40*24*time.Hour).Unix())
	if !u.IsExpired(p, now) {
		t.Fatalf("player not expired after 40 days")
	}
	u.ExpirePlayer(p, now.Unix())
	if p.BankBalance != 0 {
		t.Errorf("expiry did not forfeit the bank balance: %d", p.BankBalance)
	}
}

func TestBankCountsInPlayerValue(t *testing.T) {
	u, p := bankUniverse(t)
	before := u.PlayerValue(p)
	if err := u.BankDeposit(p, 30000); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	// moving credits from ship to bank must not change total value...
	if got := u.PlayerValue(p); got != before {
		t.Errorf("deposit changed player value: %d -> %d", before, got)
	}
	// ...and interest must raise it
	u.creditBankInterest(time.Now())
	if got := u.PlayerValue(p); got <= before {
		t.Errorf("interest did not raise player value: %d -> %d", before, got)
	}
}

func TestEnsureInterstel(t *testing.T) {
	u, _ := bankUniverse(t)
	count := 0
	var bank *Port
	for _, port := range u.Ports.Ports {
		if port.Goods == Interstel {
			count++
			bank = port
		}
	}
	if count != 1 {
		t.Fatalf("generated universe has %d Interstel ports, want 1", count)
	}
	if bank.Sector < 2 || bank.Sector > 10 {
		t.Errorf("Interstel at sector %d; want a low (Federation) sector", bank.Sector)
	}
	if !bank.IsService() {
		t.Errorf("Interstel is not a service port (would charge docking turns)")
	}
	// idempotent
	u.ensureInterstel()
	count = 0
	for _, port := range u.Ports.Ports {
		if port.Goods == Interstel {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ensureInterstel is not idempotent: %d ports", count)
	}
}

func TestBankInterestViaMaintenance(t *testing.T) {
	u, p := bankUniverse(t)
	p.BankBalance = 100000
	if ran := u.RunDailyMaintenance(time.Now()); !ran {
		t.Fatalf("maintenance did not run on a fresh universe")
	}
	if p.BankBalance != 101000 {
		t.Errorf("maintenance interest: balance=%d, want 101000", p.BankBalance)
	}
}
