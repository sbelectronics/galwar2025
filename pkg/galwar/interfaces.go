package galwar

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
	GetCommodities() []Commodity
	GetQuantity(name string) int
	GetCommodity(name string) *Commodity
	AdjustQuantity(name string, amount int)
	AdjustMoney(amount int)
}

type InventoryInterface interface {
	GetQuantity(name string) int
	GetCommodity(name string) *Commodity
	AdjustQuantity(name string, amount int)
	GetMoney() int
	AdjustMoney(amount int)
}
