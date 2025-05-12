package player

type Player struct {
	Name      string
	Sector    int
	Holds     int
	Inventory []Commodity
	Money     int
}

func (p *Player) GetName() string {
	return p.Name
}

func (p *Player) GetType() string {
	return "Player"
}

func (p *Player) Sector() int {
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

func (p *Player) AdjustQuantity(name string, amount int) {
	for i, c := range p.Inventory {
		if c.Name == name {
			p.Inventory[i].Quantity += amount
			return nil
		}
	}
	if amount > 0 {
		p.Inventory = append(p.Inventory, Commodity{Name: name, Quantity: amount})
	}
}

func (p *Player) AdjustMoney(amount int) {
	p.Money += amount
}
