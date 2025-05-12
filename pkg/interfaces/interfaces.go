package interfaces

import "github.com/sbelectronics/galwar/pkg/base"

type SectorInterface interface {
	GetWarps() []int
	GetNumber() int
}

type ObjectInterface interface {
	GetName() string
	GetType() string
	GetSector() int
}

type ObjectListInterface interface {
	GetObjectsInSector(sector int) []ObjectInterface
}

type PortInterface interface {
	GetName() string
	GetCommodities() []base.Commodity
}

type InventoryInterface interface {
	GetQuantity(name string) int
	AdjustQuantity(name string, amount int)
	AdjustMoney(amount int)
}
