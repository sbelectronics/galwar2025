package galwar

import ()

// ObjectBase - Base object structure for things that implment ObjectInterface
// These are things that are located in a sector.
type ObjectBase struct {
	Name   string
	Sector int
}

// InventoryBase - Base inventory structure for things that implement InventoryInterface
// These are things that have an inventory of commodities and money.
type InventoryBase struct {
	Inventory []*Commodity
	Money     int
}

func (o *ObjectBase) GetName() string {
	return o.Name
}

func (o *ObjectBase) GetSector() int {
	return o.Sector
}

func (o *ObjectBase) MoveTo(sector int) {
	o.Sector = sector
}

func (p *InventoryBase) HasInventory() bool {
	for _, cm := range p.Inventory {
		if cm.Quantity > 0 {
			return true
		}
	}
	return false
}

func (p *InventoryBase) GetCommodities() []*Commodity {
	return p.Inventory
}

func (p *InventoryBase) GetCommodity(name string) *Commodity {
	for _, cm := range p.Inventory {
		if cm.Name == name {
			return cm
		}
	}
	return nil
}

func (p *InventoryBase) GetQuantity(name string) int {
	cm := p.GetCommodity(name)
	if cm != nil {
		return cm.Quantity
	}
	return 0
}

func (p *InventoryBase) AdjustQuantity(name string, amount int) {
	cm := p.GetCommodity(name)
	if cm != nil {
		cm.Quantity += amount
		return
	}
	if amount > 0 {
		p.SetQuantity(name, amount)
	}
}

func (p *InventoryBase) SetQuantity(name string, amount int) {
	cm := p.GetCommodity(name)
	if cm != nil {
		cm.Quantity = amount
		return
	}
	if amount > 0 {
		cm := Commodity{Name: name, Quantity: amount} // DANGER - may miss other fields
		p.Inventory = append(p.Inventory, &cm)
	}
}

func (p *InventoryBase) GetMoney() int {
	return p.Money
}

func (p *InventoryBase) AdjustMoney(amount int) {
	p.Money += amount
}
