package galwar

import (
	"github.com/google/uuid"
)

type PlayerId string

type Player struct {
	Id     PlayerId
	Email  string
	Name   string
	Sector int
	InventoryBase
}

type PlayerList struct {
	Players []*Player
}

func GetPlayer(email string) *Player {
	for _, p := range Players.Players {
		if p.Email == email {
			return p
		}
	}
	return nil
}

func NewPlayer(name string, email string) *Player {
	p := &Player{
		Id:     PlayerId(uuid.New().String()),
		Email:  email,
		Name:   name,
		Sector: 1,
		InventoryBase: InventoryBase{
			Inventory: []Commodity{},
			Money:     1000,
		},
	}

	for _, goods := range []([]Commodity){TradeGoods, SolGoods} {
		for _, tg := range goods {
			cm := Commodity{
				Name:      tg.Name,
				ShortName: tg.ShortName,
				Holds:     tg.Holds,
				Quantity:  tg.Starting,
			}
			p.Inventory = append(p.Inventory, cm)
		}
	}

	Players.Players = append(Players.Players, p)

	return p
}

func (p *Player) GetName() string {
	return p.Name
}

func (p *Player) GetNameExtra() string {
	return ""
}

func (p *Player) GetType() string {
	return "Player"
}

func (p *Player) GetSector() int {
	return p.Sector
}

func (p *Player) MoveTo(sector int) {
	p.Sector = sector
}

func (p *Player) GetFreeHolds() int {
	freeHolds := p.GetQuantity("Cargo Holds")
	for _, c := range p.Inventory {
		freeHolds -= c.Quantity * c.Holds
	}
	return freeHolds
}

func (p *PlayerList) GetObjectsInSector(sector int) []ObjectInterface {
	var playersInSector []ObjectInterface
	for _, player := range p.Players {
		if player.Sector == sector {
			playersInSector = append(playersInSector, player)
		}
	}
	return playersInSector
}

var Players = PlayerList{}

func init() {
	Universe.RegisterPlayers(&Players)
}
