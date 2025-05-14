package galwar

type Commodity struct {
	Name      string
	Prod      int
	Quantity  int
	BuyPrice  float64
	SellPrice float64
	Sell      bool
}

type CommodityDefinition struct {
	Commodity
	ShortName   string
	Holds       int
	Starting    int
	SellAtPorts bool
	SellAtSol   bool
}

var TradeGoods = []CommodityDefinition{
	{Commodity: Commodity{Name: "Ore", BuyPrice: 8, SellPrice: 5}, ShortName: "Ore", Holds: 1, SellAtPorts: true},
	{Commodity: Commodity{Name: "Organics", BuyPrice: 14, SellPrice: 10}, ShortName: "Org", Holds: 1, SellAtPorts: true},
	{Commodity: Commodity{Name: "Equipment", BuyPrice: 25, SellPrice: 20}, ShortName: "Equ", Holds: 1, SellAtPorts: true},
	{Commodity: Commodity{Name: "Cargo Holds", SellPrice: 500, Sell: true}, ShortName: "Holds", Holds: 0, Starting: 25, SellAtSol: true},
	{Commodity: Commodity{Name: "Fighters", SellPrice: 98, Sell: true}, ShortName: "Fighters", Holds: 0, Starting: 200, SellAtSol: true},
	{Commodity: Commodity{Name: "Mines", SellPrice: 15000, Sell: true}, ShortName: "Mines", Holds: 0, SellAtSol: true},
	{Commodity: Commodity{Name: "Genesis Devices", SellPrice: 10000, Sell: true}, ShortName: "Genesis", Holds: 0, SellAtSol: true},
}

func (c *Commodity) GetDef() *CommodityDefinition {
	for i, def := range TradeGoods {
		if c.Name == def.Name {
			return &TradeGoods[i]
		}
	}
	panic("Fatal error: Commodity definition not found")
	return nil
}

func (c *Commodity) GetShortName() string {
	return c.GetDef().ShortName
}

func (c *Commodity) GetHoldsUsed() int {
	return c.GetDef().Holds * c.Quantity
}

func (c *Commodity) IsCargo() bool {
	return c.GetDef().Holds > 0
}

func (c *Commodity) GetBuyPrice(quantity int) int {
	return int(c.BuyPrice * float64(quantity))
}

func (c *Commodity) GetSellPrice(quantity int) int {
	return int(c.SellPrice * float64(quantity))
}

func (c *Commodity) GetPrice() float64 {
	if c.Sell {
		return c.SellPrice
	}
	return c.BuyPrice

}

func (c Commodity) GetBuySell() string {
	if c.Sell {
		return "Selling"
	} else {
		return "Buying"
	}
}
