package ops

import (
	"github.com/sbelectronics/galwar/interfaces"
)

// TradeBuy: Buy goods from a port to a player

func TradeBuy(name string, port InventoryInterface, player InventoryInterface, quantity integer) error
{
	// TODO: trade lock
	seller.AdjustQuantity(name, -quantity)
	if buyer.GetMoney() < price*quantity {
		return fmt.Errorf("not enough money")
	}
	buyer.AdjustMoney(-price * quantity)
	buyer.AdjustQuantity(name, quantity)
	return nil
}

// TradeSell: Sell goods from a player to a port

func TradeSell(name string, port InventoryInterface, player InventoryInterface, quantity integer) error
{
	// TODO: trade lock
	// Note: Even here we adjust the seller's quantity by -quantity, because we're actually reducing
	// the amount of goods the seller wants to purchase.
	seller.AdjustQuantity(name, -quantity)
	if buyer.GetQuantity(name) < quantity {
		return fmt.Errorf("not enough quantity")
	}
	buyer.AdjustMoney(price * quantity)
	buyer.AdjustQuantity(name, -quantity)
	return nil
}

func GetObjectsInSector(kind string)
{
	objects := []interfaces.ObjectInterface{}

	

	for _, objList := range(Universe.objLists) ) {
		objItems := objList.GetObjectsInSector(i)
		for _, obj := range objItems {
			if obj.GetType() == obj {
				objects = append(objects, port)
			}
		}
	}

	return objects
}
