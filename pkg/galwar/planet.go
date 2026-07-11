package galwar

import (
	"fmt"
	"strings"
)

type Planet struct {
	Owner PlayerId
	ObjectBase
	InventoryBase

	universe *UniverseType // back-reference for owner lookup; set by wire()/NewPlanet, not serialized
}

type PlanetList struct {
	Planets []*Planet
}

func (u *UniverseType) NewPlanet(owner PlayerId, sector int, name string) *Planet {
	p := &Planet{
		Owner: owner,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: sector,
		},
		InventoryBase: InventoryBase{},
		universe:      u,
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

	u.Planets.AddPlanet(p)

	return p
}

func (p *Planet) GetOwnerPlayer() *Player {
	if p.universe != nil {
		if player := p.universe.Players.GetById(p.Owner); player != nil {
			return player
		}
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

// GetPlanet finds the caller's planet in a sector. It examines every planet
// in the sector before deciding, rather than judging by the first one found.
// If flags does not include MUST_EXIST and no planet exists, returns
// (nil, nil).

func (p *PlanetList) GetPlanet(player *Player, sector int, flags int) (*Planet, error) {
	var mine, foreign *Planet
	for _, planet := range p.Planets {
		if planet.Sector != sector {
			continue
		}
		if planet.Owner == player.Id {
			mine = planet
		} else if foreign == nil {
			foreign = planet
		}
	}
	if flags&MUST_NOT_EXIST != 0 {
		if mine != nil {
			return nil, NewGameError(ErrAlreadyExists, "You already own a planet in this sector.")
		}
		if foreign != nil {
			// one planet per sector, as in the original
			return nil, NewGameError(ErrAlreadyExists, "There is already a planet in this sector.")
		}
		return nil, nil
	}
	if mine != nil {
		return mine, nil
	}
	if foreign != nil {
		return nil, NewGameError(ErrNotOwner, "There is a Planet in this sector, but you don't own it.")
	}
	if flags&MUST_EXIST != 0 {
		return nil, NewGameError(ErrNotFound, "No planet found in this sector.")
	}
	return nil, nil
}

func (u *UniverseType) UseGenesisDevice(player *Player, sector int, name string) error {
	if sector <= 10 {
		return NewGameError(ErrFedRestricted, "Galactic law prevents anyone except the federation from creating planets in sectors 1 through 10.")
	}
	_, err := u.Planets.GetPlanet(player, sector, MUST_NOT_EXIST)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return NewGameError(ErrInvalidName, "You must provide a name for the planet.")
	}
	if player.GetQuantity(GENESIS) < 1 {
		return NewGameError(ErrNotEnoughQuantity, "You don't have a Genesis Device.")
	}
	player.AdjustQuantity(GENESIS, -1)
	_ = u.NewPlanet(player.Id, sector, name)
	u.MarkDirty()
	return nil
}

// TransferSet sets the planet-side quantity of a commodity, moving the
// difference to or from the player.

func (u *UniverseType) TransferSet(player *Player, sector int, commodityName string, amount int) error {
	if amount < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't leave a negative quantity.")
	}
	planet, err := u.Planets.GetPlanet(player, sector, MUST_EXIST)
	if err != nil {
		return err
	}
	total := planet.GetQuantity(commodityName) + player.GetQuantity(commodityName)
	if amount > total {
		return NewGameError(ErrNotEnoughQuantity, fmt.Sprintf("Sorry, but you don't have that many %s!", commodityName))
	}
	planet.SetQuantity(commodityName, amount)
	player.SetQuantity(commodityName, total-amount)
	u.MarkDirty()
	return nil
}

func (u *UniverseType) TransferOut(player *Player, sector int, commodityName string, amount int) error {
	if amount < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't take a negative quantity.")
	}
	planet, err := u.Planets.GetPlanet(player, sector, MUST_EXIST)
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
	u.MarkDirty()
	return nil
}

func (u *UniverseType) TransferIn(player *Player, sector int) error {
	planet, err := u.Planets.GetPlanet(player, sector, MUST_EXIST)
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
	u.MarkDirty()
	return nil
}

func (p *PlanetList) AddPlanet(planet *Planet) {
	p.Planets = append(p.Planets, planet)
}

func (p *PlanetList) RemovePlanet(planet *Planet) {
	for i, pl := range p.Planets {
		if pl == planet {
			p.Planets = append(p.Planets[:i], p.Planets[i+1:]...)
			break
		}
	}
}
