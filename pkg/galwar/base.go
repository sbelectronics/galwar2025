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
}

var TradeGoods = []Commodity{
	{Name: "Ore", ShortName: "Ore", BuyPrice: 8, SellPrice: 5, Holds: 1},
	{Name: "Organics", ShortName: "Org", BuyPrice: 14, SellPrice: 10, Holds: 1},
	{Name: "Equipment", ShortName: "Equ", BuyPrice: 25, SellPrice: 20, Holds: 1},
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
