package galwar

type Commodity struct {
	Name      string
	ShortName string
	Prod      int
	Quantity  int
	BuyPrice  float64
	SellPrice float64
	Sell      bool
	Holds     int
	Starting  int
}

var TradeGoods = []Commodity{
	{Name: "Ore", ShortName: "Ore", BuyPrice: 8, SellPrice: 5, Holds: 1},
	{Name: "Organics", ShortName: "Org", BuyPrice: 14, SellPrice: 10, Holds: 1},
	{Name: "Equipment", ShortName: "Equ", BuyPrice: 25, SellPrice: 20, Holds: 1},
}

var SolGoods = []Commodity{
	{Name: "Cargo Holds", ShortName: "Holds", BuyPrice: 0, SellPrice: 500, Holds: 0, Starting: 25, Sell: true},
	{Name: "Fighters", ShortName: "Fighters", BuyPrice: 0, SellPrice: 98, Holds: 0, Starting: 200, Sell: true},
	{Name: "Mines", ShortName: "Mines", BuyPrice: 0, SellPrice: 15000, Holds: 0, Sell: true},
	{Name: "Genesis Devices", ShortName: "Genesis", BuyPrice: 0, SellPrice: 10000, Holds: 0, Sell: true},
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
