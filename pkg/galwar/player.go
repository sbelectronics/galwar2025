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

func NewPlayer(name string, email string) *Player {
	p := &Player{
		Id:    PlayerId(uuid.New().String()),
		Email: email,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: 1,
		},
		InventoryBase: InventoryBase{
			Money: 35000,
		},
	}

	for _, tg := range TradeGoods {
		cm := Commodity{
			Name:     tg.Name,
			Quantity: tg.Starting,
		}
		p.Inventory = append(p.Inventory, &cm)
	}

	Players.Players = append(Players.Players, p)

	return p
}

func (p *Player) GetNameExtra() string {
	return ""
}

func (p *Player) GetType() string {
	return TYPE_PLAYER
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

func (p *PlayerList) GetByEmail(email string) *Player {
	for _, p := range p.Players {
		if p.Email == email {
			return p
		}
	}
	return nil
}

func (p *PlayerList) GetById(id PlayerId) *Player {
	for _, p := range p.Players {
		if p.Id == id {
			return p
		}
	}
	return nil
}

var Players = PlayerList{}

func init() {
	Universe.RegisterPlayers(&Players)
}
