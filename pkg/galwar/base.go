package galwar

import (
	"log"
)

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

// GetFreeHolds returns the number of unoccupied cargo holds. Lives on
// InventoryBase (not Player) so the engine can enforce hold limits through
// InventoryInterface.
func (p *InventoryBase) GetFreeHolds() int {
	freeHolds := p.GetQuantity(HOLDS)
	for _, c := range p.Inventory {
		freeHolds -= c.GetHoldsUsed()
	}
	return freeHolds
}

// AdjustQuantity changes a commodity count by a delta. Quantities are clamped
// at zero: callers are required to validate before mutating, so a clamp
// firing indicates a bug in the caller and is logged loudly.
func (p *InventoryBase) AdjustQuantity(name string, amount int) {
	cm := p.GetCommodity(name)
	if cm != nil {
		cm.Quantity += amount
		if cm.Quantity < 0 {
			log.Printf("BUG: quantity of %s went negative (%d); clamped to 0 - caller failed to validate", name, cm.Quantity)
			cm.Quantity = 0
		}
		return
	}
	if amount > 0 {
		p.SetQuantity(name, amount)
	} else if amount < 0 {
		log.Printf("BUG: attempt to remove %d of missing commodity %s ignored - caller failed to validate", -amount, name)
	}
}

func (p *InventoryBase) SetQuantity(name string, amount int) {
	if amount < 0 {
		log.Printf("BUG: SetQuantity(%s, %d) clamped to 0 - caller failed to validate", name, amount)
		amount = 0
	}
	cm := p.GetCommodity(name)
	if cm != nil {
		cm.Quantity = amount
		return
	}
	if amount > 0 {
		cm := Commodity{Name: name, Quantity: amount}
		if def := FindCommodityDef(name); def != nil {
			cm.BuyPrice = def.BuyPrice
			cm.SellPrice = def.SellPrice
		}
		p.Inventory = append(p.Inventory, &cm)
	}
}

func (p *InventoryBase) GetMoney() int {
	return p.Money
}

func (p *InventoryBase) AdjustMoney(amount int) {
	p.Money += amount
	if p.Money < 0 {
		log.Printf("BUG: money went negative (%d); clamped to 0 - caller failed to validate", p.Money)
		p.Money = 0
	}
}
