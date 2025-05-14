package galwar

import (
	"fmt"
)

// TradeBuy: Buy goods from a port to a player

func TradeBuy(name string, port PortInterface, player InventoryInterface, quantity int) error {
	// TODO: trade lock
	if quantity < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't buy negative quantity.")
	}
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return NewGameError(ErrUnknown, fmt.Sprintf("commodity %s not found in port %s", name, port.GetName()))
	}
	if commodity.Quantity < quantity {
		return NewGameError(ErrNotEnoughQuantity, "We aren't selling that many.")
	}
	totalPrice := commodity.GetSellPrice(quantity)
	port.AdjustQuantity(name, -quantity)
	if player.GetMoney() < totalPrice {
		return NewGameError(ErrNotEnoughMoney, "You don't have enough credits.")
	}
	player.AdjustMoney(-totalPrice)
	player.AdjustQuantity(name, quantity)
	return nil
}

// TradeSell: Sell goods from a player to a port

func TradeSell(name string, port PortInterface, player InventoryInterface, quantity int) error {
	// TODO: trade lock
	// Note: Even here we adjust the port's quantity by -quantity, because we're actually reducing
	// the amount of goods the port wants to purchase.
	if quantity < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't sell negative quantity.")
	}
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return NewGameError(ErrUnknown, fmt.Sprintf("commodity %s not found in port %s", name, port.GetName()))
	}
	if commodity.Quantity < quantity {
		return NewGameError(ErrNotEnoughQuantity, "We aren't buying that many.")
	}
	totalPrice := commodity.GetBuyPrice(quantity)
	port.AdjustQuantity(name, -quantity)
	if player.GetQuantity(name) < quantity {
		return NewGameError(ErrNotEnoughMoney, "You don't have enough credits.")
	}
	player.AdjustMoney(totalPrice)
	player.AdjustQuantity(name, -quantity)
	return nil
}

// TradeBuyNoLimit: For things like SolGoods
// No port quanity to check or adjust

func TradeBuyNoLimit(commodity *Commodity, player InventoryInterface, quantity int) error {
	// TODO: trade lock
	totalPrice := commodity.GetSellPrice(quantity)
	if player.GetMoney() < totalPrice {
		return fmt.Errorf("not enough money")
	}
	player.AdjustMoney(-totalPrice)
	player.AdjustQuantity(commodity.Name, quantity)
	return nil
}
