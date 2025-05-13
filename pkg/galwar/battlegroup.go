package galwar

import ()

type Battlegroup struct {
	owner     PlayerId
	Name      string
	Sector    int
	Inventory []Commodity
}

type BattlegroupList struct {
	Battlegroups []*Battlegroup
}

func (b *Battlegroup) GetName() string {
	return b.Name
}

func (b *Battlegroup) GetNameExtra() string {
	return ""
}

func (b *Battlegroup) GetType() string {
	return "Battlegroup"
}

func (b *Battlegroup) GetSector() int {
	return b.Sector
}

func (b *Battlegroup) GetCommodities() []Commodity {
	return b.Inventory
}

func (b *Battlegroup) GetQuantity(name string) int {
	for _, c := range b.Inventory {
		if c.Name == name {
			return c.Quantity
		}
	}
	return 0
}

func (b *Battlegroup) GetCommodity(name string) *Commodity {
	for _, c := range b.Inventory {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func (b *Battlegroup) AdjustQuantity(name string, amount int) {
	for i, c := range b.Inventory {
		if c.Name == name {
			b.Inventory[i].Quantity += amount
			return
		}
	}
	if amount > 0 {
		cm := Commodity{Name: name, Quantity: amount}
		b.Inventory = append(b.Inventory, cm)
	}
}

func (b *Battlegroup) GetMoney() int {
	return 0
}

func (b *Battlegroup) AdjustMoney(amount int) {
	_ = amount
}

func (b *BattlegroupList) GetObjectsInSector(sector int) []ObjectInterface {
	var bgsInSector []ObjectInterface
	for _, bg := range b.Battlegroups {
		if bg.Sector == sector {
			bgsInSector = append(bgsInSector, bg)
		}
	}
	return bgsInSector
}

var Battlegroups = BattlegroupList{}

func init() {
	Universe.RegisterBattlegroups(&Battlegroups)
}
