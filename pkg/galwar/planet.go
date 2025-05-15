package galwar

import (
	"fmt"
)

type Planet struct {
	Owner PlayerId
	ObjectBase
	InventoryBase
}

type PlanetList struct {
	Planets []*Planet
}

func NewPlanet(owner PlayerId, sector int, name string) *Planet {
	p := &Planet{
		Owner: owner,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: sector,
		},
		InventoryBase: InventoryBase{},
	}

	for _, tg := range TradeGoods {
		if !tg.OnPlanet {
			continue
		}
		cm := Commodity{
			Name:     tg.Name,
			Quantity: tg.PlanetStarting,
		}
		p.Inventory = append(p.Inventory, &cm)
	}

	Planets.AddPlanet(p)

	return p
}

func (p *Planet) GetOwnerPlayer() *Player {
	player := Players.GetById(p.Owner)
	if player != nil {
		return player
	}
	return &Player{ObjectBase: ObjectBase{Name: "Unknown"}}
}

func (p *Planet) GetName() string {
	return p.Name
}

func (p *Planet) GetNameExtra() string {
	return fmt.Sprintf("Owned by %s", p.GetOwnerPlayer().GetName())
}

func (p *Planet) GetType() string {
	return TYPE_PLANET
}

func (p *PlanetList) GetObjectsInSector(sector int) []ObjectInterface {
	var planetsInSector []ObjectInterface
	for _, planet := range p.Planets {
		if planet.Sector == sector {
			planetsInSector = append(planetsInSector, planet)
		}
	}
	return planetsInSector
}

// GetPlanet
// If !create and no group exists, will return nil and no error

func (p *PlanetList) GetPlanet(player *Player, sector int, flags int) (*Planet, error) {
	// TODO: planetlock
	objs := p.GetObjectsInSector(sector)
	for _, obj := range objs {
		planet, ok := obj.(*Planet)
		if !ok {
			return nil, NewGameError(ErrInvalidType, "Object is not a Planet")
		}
		if planet.Owner == player.Id {
			if flags&MUST_NOT_EXIST != 0 {
				return nil, NewGameError(ErrAlreadyExists, "You already own a planet in this sector.")
			}
			return planet, nil
		} else {
			return nil, NewGameError(ErrNotOwner, "There is a Planet in this sector, but you don't own it.")
		}
	}
	if flags&MUST_EXIST != 0 {
		return nil, NewGameError(ErrNotFound, "No planet found in this sector.")
	}
	return nil, nil
}

func (p *PlanetList) UseGenesisDevice(player *Player, sector int, name string) error {
	// TODO: planetlock, playerlock
	if sector < 10 {
		return NewGameError(ErrFedRestricted, "Galactic law prevents anyone except the federation from creating planets in sectors 1 through 10.")
	}
	_, err := p.GetPlanet(player, sector, MUST_NOT_EXIST)
	if err != nil {
		return err
	}
	if name == "" {
		return NewGameError(ErrInvalidName, "You must provide a name for the planet.")
	}
	if player.GetQuantity(GENESIS) < 1 {
		return NewGameError(ErrNotEnoughQuantity, "You don't have a Genesis Device.")
	}
	player.AdjustQuantity(GENESIS, -1)
	_ = NewPlanet(player.Id, sector, name)
	return nil
}

func (p *PlanetList) TransferSet(player *Player, sector int, commodityName string, amount int) error {
	// TODO: planetlock, playerlock
	planet, err := p.GetPlanet(player, sector, MUST_EXIST)
	if err != nil {
		return err
	}
	total := planet.GetQuantity(commodityName) + player.GetQuantity(commodityName)
	if amount > total {
		return NewGameError(ErrNotEnoughQuantity, fmt.Sprintf("Sorry, but you don't have that many %s!", commodityName))
	}
	planet.SetQuantity(commodityName, amount)
	player.SetQuantity(commodityName, total-amount)
	return nil
}

func (p *PlanetList) TransferOut(player *Player, sector int, commodityName string, amount int) error {
	// TODO: planetlock, playerlock
	planet, err := p.GetPlanet(player, sector, MUST_EXIST)
	if err != nil {
		return err
	}
	if amount > planet.GetQuantity(commodityName) {
		return NewGameError(ErrNotEnoughQuantity, fmt.Sprintf("Sorry, but you cannot take that much %s!", commodityName))
	}
	if amount > player.GetFreeHolds() {
		return NewGameError(ErrNotEnoughHolds, "Sorry, but you don't have enough free holds.")
	}
	planet.AdjustQuantity(commodityName, -amount)
	player.AdjustQuantity(commodityName, amount)
	return nil
}

func (p *PlanetList) TransferIn(player *Player, sector int) error {
	// TODO: planetlock, playerlock
	planet, err := p.GetPlanet(player, sector, MUST_EXIST)
	if err != nil {
		return err
	}
	for _, c := range player.GetCommodities() {
		tg := c.GetDef()
		if !tg.OnPlanet || !tg.SellAtPorts {
			continue
		}
		if c.Quantity > 0 {
			planet.AdjustQuantity(c.Name, c.Quantity)
			player.SetQuantity(c.Name, 0)
		}
	}
	return nil
}

func (p *PlanetList) AddPlanet(planet *Planet) {
	// TODO: lock
	p.Planets = append(p.Planets, planet)
}

func (p *PlanetList) RemovePlanet(planet *Planet) {
	// TODO: lock
	for i, pl := range p.Planets {
		if pl == planet {
			p.Planets = append(p.Planets[:i], p.Planets[i+1:]...)
			break
		}
	}
}

var Planets = PlanetList{}

func init() {
	Universe.RegisterPlanets(&Planets)
}
