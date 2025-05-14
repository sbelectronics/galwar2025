package galwar

import (
	"github.com/google/uuid"
)

type PlayerId string

type Player struct {
	Id    PlayerId
	Email string
	ObjectBase
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
		Id:    PlayerId(uuid.New().String()),
		Email: email,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: 1,
		},
		InventoryBase: InventoryBase{
			Inventory: []Commodity{},
			Money:     1000,
		},
	}

	for _, tg := range TradeGoods {
		cm := Commodity{
			Name:     tg.Name,
			Quantity: tg.Starting,
		}
		p.Inventory = append(p.Inventory, cm)
	}

	Players.Players = append(Players.Players, p)

	return p
}

func (p *Player) GetNameExtra() string {
	return ""
}

func (p *Player) GetType() string {
	return "Player"
}

func (p *Player) GetFreeHolds() int {
	freeHolds := p.GetQuantity("Cargo Holds")
	for _, c := range p.Inventory {
		freeHolds -= c.GetHoldsUsed()
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
