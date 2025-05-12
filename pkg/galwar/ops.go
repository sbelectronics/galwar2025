package galwar

import (
	"fmt"
)

// TradeBuy: Buy goods from a port to a player

func TradeBuy(name string, port PortInterface, player InventoryInterface, quantity int) error {
	// TODO: trade lock
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return fmt.Errorf("commodity %s not found in port %s", name, port.GetName())
	}
	price := commodity.Price
	port.AdjustQuantity(name, -quantity)
	if player.GetMoney() < price*quantity {
		return fmt.Errorf("not enough money")
	}
	player.AdjustMoney(-price * quantity)
	player.AdjustQuantity(name, quantity)
	return nil
}

// TradeSell: Sell goods from a player to a port

func TradeSell(name string, port PortInterface, player InventoryInterface, quantity int) error {
	// TODO: trade lock
	// Note: Even here we adjust the port's quantity by -quantity, because we're actually reducing
	// the amount of goods the port wants to purchase.
	commodity := port.GetCommodity(name)
	if commodity == nil {
		return fmt.Errorf("commodity %s not found in port %s", name, port.GetName())
	}
	price := commodity.Price
	port.AdjustQuantity(name, -quantity)
	if player.GetQuantity(name) < quantity {
		return fmt.Errorf("not enough quantity")
	}
	player.AdjustMoney(price * quantity)
	player.AdjustQuantity(name, -quantity)
	return nil
}
