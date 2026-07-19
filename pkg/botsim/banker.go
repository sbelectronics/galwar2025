package botsim

import (
	"math/rand"
	"strconv"

	"github.com/sbelectronics/galwar/pkg/galwar"
)

// Banker trades like a Trader but sweeps its profits into the Interstel bank,
// keeping only working capital aboard. Over a long run it pushes its balance
// through the 1M interest cap and exercises deposits/withdrawals at scale, plus
// the death-with-a-full-bank case if an Aggressor catches it (PLAN-BOTS.md 3.5).
type Banker struct {
	*Trader
	keepAboard int // working capital to keep on the ship; the rest is banked
}

// NewBanker builds a Banker brain.
func NewBanker(name string, rng *rand.Rand) *Banker {
	t := NewTrader(name, "banker", rng)
	t.reinvestReserve = 1 << 30 // banks instead of buying holds/fighters at Sol
	return &Banker{Trader: t, keepAboard: 15000}
}

// Plan sweeps surplus to the bank when it builds up, tops the ship back up when
// cash-starved, and otherwise trades as the base Trader.
func (b *Banker) Plan(v *View) Action {
	if !v.Self.HasTurns() {
		return pass()
	}
	if bank := v.Mem.NearestServicePort(v.Self.Sector, galwar.Interstel); bank != 0 {
		switch {
		case v.Self.Money > 2*b.keepAboard: // surplus: deposit it
			if v.Self.Sector == bank {
				return b.deposit(v)
			}
			return b.moveToward(v, bank)
		case v.Self.Money < b.keepAboard/2 && v.Self.Bank > 0: // cash-starved: withdraw
			if v.Self.Sector == bank {
				return b.withdraw(v)
			}
			return b.moveToward(v, bank)
		}
	}
	return b.Trader.Plan(v)
}

func (b *Banker) deposit(v *View) Action {
	amount := v.Self.Money - b.keepAboard
	if amount < 1 {
		return b.Trader.Plan(v)
	}
	b.lastDockMoney = -1 // banking moves money without trading: recalibrate
	return act("bank", "p", "d", strconv.Itoa(amount), "q").
		withDetail(map[string]any{"deposit": amount, "balance_before": v.Self.Bank})
}

func (b *Banker) withdraw(v *View) Action {
	amount := b.keepAboard
	if amount > v.Self.Bank {
		amount = v.Self.Bank
	}
	b.lastDockMoney = -1
	return act("bank", "p", "w", strconv.Itoa(amount), "q").
		withDetail(map[string]any{"withdraw": amount, "balance_before": v.Self.Bank})
}
