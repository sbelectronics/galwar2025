package galwar

type InventoryBase struct {
	Inventory []Commodity
	Money     int
}

func (p *InventoryBase) GetCommodities() []Commodity {
	return p.Inventory
}

func (p *InventoryBase) GetQuantity(name string) int {
	for _, c := range p.Inventory {
		if c.Name == name {
			return c.Quantity
		}
	}
	return 0
}

func (p *InventoryBase) GetCommodity(name string) *Commodity {
	for _, c := range p.Inventory {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func (p *InventoryBase) AdjustQuantity(name string, amount int) {
	for i, c := range p.Inventory {
		if c.Name == name {
			p.Inventory[i].Quantity += amount
			return
		}
	}
	if amount > 0 {
		cm := Commodity{Name: name, Quantity: amount}
		p.Inventory = append(p.Inventory, cm)
	}
}

func (p *InventoryBase) GetMoney() int {
	return p.Money
}

func (p *InventoryBase) AdjustMoney(amount int) {
	p.Money += amount
}
