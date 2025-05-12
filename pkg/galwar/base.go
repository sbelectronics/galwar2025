package galwar

type Commodity struct {
	Name      string
	ShortName string
	Prod      int
	Quantity  int
	BuyPrice  float64
	SellPrice float64
	Sell      bool
}

var TradeGoods = []Commodity{
	{Name: "Ore", ShortName: "Ore", BuyPrice: 8, SellPrice: 5},
	{Name: "Organics", ShortName: "Org", BuyPrice: 14, SellPrice: 10},
	{Name: "Equipment", ShortName: "Equ", BuyPrice: 25, SellPrice: 20},
}

func (c *Commodity) GetBuyPrice(quantity int) int {
	return int(c.BuyPrice * float64(quantity))
}

func (c *Commodity) GetSellPrice(quantity int) int {
	return int(c.SellPrice * float64(quantity))
}
