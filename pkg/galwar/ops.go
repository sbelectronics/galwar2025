package galwar

import (
	"fmt"
)

// Trade operations follow a strict validate-then-mutate discipline: every
// precondition is checked before any state changes, so a failed trade never
// leaves the port or the player partially modified. When called through the
// universe actor (Universe.Do), each trade is atomic with respect to all
// other commands.

// TradeBuy: Buy goods from a port to a player

func TradeBuy(name string, port PortInterface, player InventoryInterface, quantity int) error {
	if quantity < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't buy a negative quantity.")
	}
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return NewGameError(ErrUnknown, fmt.Sprintf("commodity %s not found in port %s", name, port.GetName()))
	}
	if commodity.Quantity < quantity {
		return NewGameError(ErrNotEnoughQuantity, "We aren't selling that many.")
	}
	totalPrice := commodity.GetSellPrice(quantity)
	if player.GetMoney() < totalPrice {
		return NewGameError(ErrNotEnoughMoney, "You don't have enough credits.")
	}
	if commodity.IsCargo() && quantity > player.GetFreeHolds() {
		return NewGameError(ErrNotEnoughHolds, "You don't have enough free holds.")
	}
	port.AdjustQuantity(name, -quantity)
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
	if commodity.Quantity < quantity {
		return NewGameError(ErrNotEnoughQuantity, "We aren't buying that many.")
	}
	if player.GetQuantity(name) < quantity {
		return NewGameError(ErrNotEnoughQuantity, "You don't have that many to sell.")
	}
	totalPrice := commodity.GetBuyPrice(quantity)
	port.AdjustQuantity(name, -quantity)
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
