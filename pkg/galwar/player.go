package galwar

type Player struct {
	Name      string
	Sector    int
	Holds     int
	Inventory []Commodity
	Money     int
}

func NewPlayer(name string) *Player {
	p := &Player{
		Name:      name,
		Sector:    1,
		Holds:     50,
		Inventory: []Commodity{},
		Money:     1000,
	}

	for _, tg := range TradeGoods {
		cm := Commodity{
			Name:      tg.Name,
			ShortName: tg.ShortName,
			Holds:     tg.Holds,
		}
		p.Inventory = append(p.Inventory, cm)
	}
	return p
}

func (p *Player) GetName() string {
	return p.Name
}

func (p *Player) GetType() string {
	return "Player"
}

func (p *Player) GetSector() int {
	return p.Sector
}

func (p *Player) GetCommodities() []Commodity {
	return p.Inventory
}

func (p *Player) GetQuantity(name string) int {
	for _, c := range p.Inventory {
		if c.Name == name {
			return c.Quantity
		}
	}
	return 0
}

func (p *Player) GetCommodity(name string) *Commodity {
	for _, c := range p.Inventory {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func (p *Player) AdjustQuantity(name string, amount int) {
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

func (p *Player) GetMoney() int {
	return p.Money
}

func (p *Player) AdjustMoney(amount int) {
	p.Money += amount
}

func (p *Player) MoveTo(sector int) {
	p.Sector = sector
}

func (p *Player) GetFreeHolds() int {
	freeHolds := p.Holds
	for _, c := range p.Inventory {
		freeHolds -= c.Quantity * c.Holds
	}
	return freeHolds
}
