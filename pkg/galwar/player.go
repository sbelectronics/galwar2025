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

func (u *UniverseType) NewPlayer(name string, email string) *Player {
	p := &Player{
		Id:    PlayerId(uuid.New().String()),
		Email: email,
		ObjectBase: ObjectBase{
			Name:   name,
			Sector: 1,
		},
		InventoryBase: InventoryBase{
			Money: u.ConfigInt("starting_credits", 35000),
		},
	}

	for _, tg := range TradeGoods {
		quantity := tg.Starting
		switch tg.Name {
		case HOLDS:
			quantity = u.ConfigInt("starting_holds", quantity)
		case FIGHTERS:
			quantity = u.ConfigInt("starting_fighters", quantity)
		case TURNS:
			quantity = u.ConfigInt("turns_per_day", quantity)
		}
		cm := Commodity{
			Name:     tg.Name,
			Quantity: quantity,
		}
		p.Inventory = append(p.Inventory, &cm)
	}

	u.Players.Players = append(u.Players.Players, p)
	u.MarkDirty()

	return p
}

func (p *Player) GetNameExtra() string {
	return ""
}

func (p *Player) GetType() string {
	return TYPE_PLAYER
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
