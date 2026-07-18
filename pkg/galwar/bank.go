package galwar

import "time"

// The Interstel bank - the descendant of the original's credit account
// (INTERSTE.PAS), pared down to deposits, withdrawals, and nightly interest.
// The account is the one asset that survives the destruction of your ship:
// death wipes the ship, not the ledger. That "how much do I keep liquid?"
// decision is the bank's entire gameplay.
//
// Deviation from the original, deliberate: the original paid 5%/day, which
// was survivable only because players were deleted after 21 idle days and
// universes reset. In an always-on persistent world that's exp(0.05*t) - a
// money printer. Here interest is bank_interest_pct (default 1%) per nightly
// maintenance and accrues only on the first bank_interest_cap credits, so
// parked wealth plateaus instead of exploding. The balance counts toward
// PlayerValue (value.go), so the bank is no shelter from faction targeting
// or the rankings; Tier-2 expiry forfeits it like everything else (death.go).

// BankDeposit moves credits from the ship's hold to the player's Interstel
// account. The UI gates this on being docked at the Interstel port, like the
// Sol purchases.
func (u *UniverseType) BankDeposit(p *Player, amount int) error {
	if p.IsDead() {
		return NewGameError(ErrDead, "You are dead.")
	}
	if amount < 1 {
		return NewGameError(ErrNegativeQuantity, "You must deposit at least one credit.")
	}
	if p.GetMoney() < amount {
		return NewGameError(ErrNotEnoughMoney, "You don't have that many credits aboard.")
	}
	p.AdjustMoney(-amount)
	p.BankBalance += amount
	u.MarkDirty()
	return nil
}

// BankWithdraw moves credits from the account back to the ship.
func (u *UniverseType) BankWithdraw(p *Player, amount int) error {
	if p.IsDead() {
		return NewGameError(ErrDead, "You are dead.")
	}
	if amount < 1 {
		return NewGameError(ErrNegativeQuantity, "You must withdraw at least one credit.")
	}
	if p.BankBalance < amount {
		return NewGameError(ErrNotEnoughMoney, "Your account doesn't hold that many credits.")
	}
	p.BankBalance -= amount
	p.AdjustMoney(amount)
	u.MarkDirty()
	return nil
}

// creditBankInterest applies one night of interest to every account, called
// from RunDailyMaintenance. A dormant player's account coasts without
// compounding - the same Tier-1 philosophy that freezes their planets'
// production growth - so absence isn't an investment strategy. Returns the
// number of accounts credited.
func (u *UniverseType) creditBankInterest(now time.Time) int {
	pct := u.ConfigInt("bank_interest_pct", 1)
	capAmt := u.ConfigInt("bank_interest_cap", 1000000)
	credited := 0
	for _, p := range u.Players.Players {
		if p.IsNPC() || p.BankBalance <= 0 || u.IsDormant(p, now) {
			continue
		}
		base := p.BankBalance
		if base > capAmt {
			base = capAmt
		}
		if gain := base * pct / 100; gain > 0 {
			p.BankBalance += gain
			credited++
		}
	}
	if credited > 0 {
		u.MarkDirty()
	}
	return credited
}
