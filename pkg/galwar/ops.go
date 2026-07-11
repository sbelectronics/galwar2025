package galwar

import (
	"fmt"
)

// Trade operations follow a strict validate-then-mutate discipline: every
// precondition is checked before any state changes, so a failed trade never
// leaves the port or the player partially modified. When called through the
// universe actor (Universe.Do), each trade is atomic with respect to all
// other commands.

// Volume scaling, per the original's scaleup/scaledown (TWLIB1.PAS:803-828):
// ships with more than 50 holds trade in multiplied blocks, so a big
// freighter can fill up in one visit. The port's stock is quoted to the
// player multiplied by the factor, and each player-unit traded consumes only
// 1/factor port-units.

func ScaleFactor(player InventoryInterface) float64 {
	holds := player.GetQuantity(HOLDS)
	if holds < 50 {
		return 1
	}
	r := float64(holds) / 50
	if r > 275 {
		r = 275
	}
	return r
}

// ScaleUp converts port-units to the player-units quoted to this player.
func ScaleUp(player InventoryInterface, w int) int {
	return int(float64(w) * ScaleFactor(player))
}

// scaleDown converts player-units traded back to port inventory units: the
// smallest delta whose ScaleUp covers the traded amount. Deviation from the
// original's bare trunc (which let fractional factors under-consume port
// stock): rounding goes against the trader. Inverting through ScaleUp itself
// guarantees the result never exceeds a port quantity that already passed a
// ScaleUp(qty) >= w check.
func scaleDown(player InventoryInterface, w int) int {
	if w <= 0 {
		return 0
	}
	d := int(float64(w) / ScaleFactor(player))
	if d < 1 {
		d = 1
	}
	for ScaleUp(player, d) < w {
		d++
	}
	return d
}

// Universe-method forms of the trades below: identical rules, plus marking
// the universe dirty for the write-behind persister. Sessions should call
// these; the free functions remain for direct engine composition and tests.

func (u *UniverseType) TradeBuy(name string, port PortInterface, player InventoryInterface, quantity int) error {
	if err := TradeBuy(name, port, player, quantity); err != nil {
		return err
	}
	u.MarkDirty()
	return nil
}

func (u *UniverseType) TradeSell(name string, port PortInterface, player InventoryInterface, quantity int) error {
	if err := TradeSell(name, port, player, quantity); err != nil {
		return err
	}
	u.MarkDirty()
	return nil
}

func (u *UniverseType) TradeBuyNoLimit(commodity *Commodity, player InventoryInterface, quantity int) error {
	if err := TradeBuyNoLimit(commodity, player, quantity); err != nil {
		return err
	}
	u.MarkDirty()
	return nil
}

// TradeBuy: Buy goods from a port to a player

func TradeBuy(name string, port PortInterface, player InventoryInterface, quantity int) error {
	if quantity < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't buy a negative quantity.")
	}
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return NewGameError(ErrUnknown, fmt.Sprintf("commodity %s not found in port %s", name, port.GetName()))
	}
	if ScaleUp(player, commodity.Quantity) < quantity {
		return NewGameError(ErrNotEnoughQuantity, "We aren't selling that many.")
	}
	totalPrice := commodity.GetSellPrice(quantity)
	if player.GetMoney() < totalPrice {
		return NewGameError(ErrNotEnoughMoney, "You don't have enough credits.")
	}
	if commodity.IsCargo() && quantity > player.GetFreeHolds() {
		return NewGameError(ErrNotEnoughHolds, "You don't have enough free holds.")
	}
	port.AdjustQuantity(name, -scaleDown(player, quantity))
	player.AdjustMoney(-totalPrice)
	player.AdjustQuantity(name, quantity)
	return nil
}

// TradeSell: Sell goods from a player to a port

func TradeSell(name string, port PortInterface, player InventoryInterface, quantity int) error {
	// Note: Even here we adjust the port's quantity by -quantity, because we're actually reducing
	// the amount of goods the port wants to purchase.
	if quantity < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't sell a negative quantity.")
	}
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return NewGameError(ErrUnknown, fmt.Sprintf("commodity %s not found in port %s", name, port.GetName()))
	}
	if ScaleUp(player, commodity.Quantity) < quantity {
		return NewGameError(ErrNotEnoughQuantity, "We aren't buying that many.")
	}
	if player.GetQuantity(name) < quantity {
		return NewGameError(ErrNotEnoughQuantity, "You don't have that many to sell.")
	}
	totalPrice := commodity.GetBuyPrice(quantity)
	port.AdjustQuantity(name, -scaleDown(player, quantity))
	player.AdjustMoney(totalPrice)
	player.AdjustQuantity(name, -quantity)
	return nil
}

// TradeBuyNoLimit: For things like SolGoods
// No port quanity to check or adjust

func TradeBuyNoLimit(commodity *Commodity, player InventoryInterface, quantity int) error {
	if quantity < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't buy a negative quantity.")
	}
	totalPrice := commodity.GetSellPrice(quantity)
	if player.GetMoney() < totalPrice {
		return NewGameError(ErrNotEnoughMoney, "You don't have enough credits.")
	}
	player.AdjustMoney(-totalPrice)
	player.AdjustQuantity(commodity.Name, quantity)
	return nil
}
