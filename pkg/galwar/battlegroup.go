package galwar

import (
	"fmt"
	"strings"
)

type Battlegroup struct {
	Owner PlayerId
	ObjectBase
	InventoryBase
}

type BattlegroupList struct {
	Battlegroups []*Battlegroup
}

func NewBattlegroup(owner PlayerId, sector int) *Battlegroup {
	b := &Battlegroup{
		Owner: owner,
		ObjectBase: ObjectBase{
			Name:   "",
			Sector: sector,
		},
		InventoryBase: InventoryBase{},
	}

	Battlegroups.AddBattlegroup(b)

	return b
}

func (b *Battlegroup) GetOwnerPlayer() *Player {
	player := Players.GetById(b.Owner)
	if player != nil {
		return player
	}
	return &Player{ObjectBase: ObjectBase{Name: "Unknown"}}
}

func (b *Battlegroup) GetName() string {
	itemStrs := []string{}
	for _, c := range b.Inventory {
		if c.Quantity > 0 {
			itemStrs = append(itemStrs, fmt.Sprintf("%d %s", c.Quantity, c.GetShortName()))
		}
	}
	return strings.Join(itemStrs, " and ")
}

func (b *Battlegroup) GetNameExtra() string {
	return fmt.Sprintf("Owned by %s", b.GetOwnerPlayer().GetName())
}

func (b *Battlegroup) GetType() string {
	return TYPE_BATTLEGROUP
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

// GetBattlegroup
// If !create and no group exists, will return nil and no error

func (b *BattlegroupList) GetBattlegroup(player *Player, sector int, create bool) (*Battlegroup, error) {
	// TODO: bglock
	objs := b.GetObjectsInSector(sector)
	for _, obj := range objs {
		bg, ok := obj.(*Battlegroup)
		if !ok {
			return nil, NewGameError(ErrInvalidType, "Object is not a Battlegroup")
		}
		if bg.Owner == player.Id {
			return bg, nil
		} else {
			return nil, NewGameError(ErrNotOwner, "There is a battlegroup in this sector, but you don't own it.")
		}
	}
	if create {
		return NewBattlegroup(player.Id, sector), nil
	}
	return nil, nil
}

func (b *BattlegroupList) AdjustBattlegroup(player *Player, sector int, kind string, amount int) error {
	// TODO: bglock, playerlock
	if sector < 10 {
		return NewGameError(ErrFedRestricted, "Galactic law prevents anyone except the federation from leaving fighters or mines in sectors 1 through 10.")
	}
	bg, err := b.GetBattlegroup(player, sector, true)
	if err != nil {
		return err
	}
	total := bg.GetQuantity(kind) + player.GetQuantity(kind)
	if amount > total {
		if !bg.HasInventory() {
			b.RemoveBattlegroup(bg)
		}
		return NewGameError(ErrNotEnoughQuantity, fmt.Sprintf("You don't have that many %s!", kind))
	}
	bg.SetQuantity(kind, amount)
	player.SetQuantity(kind, total-amount)
	if !bg.HasInventory() {
		b.RemoveBattlegroup(bg)
	}
	return nil
}

func (b *BattlegroupList) AddBattlegroup(bg *Battlegroup) {
	// TODO: lock
	b.Battlegroups = append(b.Battlegroups, bg)
}

func (b *BattlegroupList) RemoveBattlegroup(bg *Battlegroup) {
	// TODO: lock
	for i, battlegroup := range b.Battlegroups {
		if battlegroup == bg {
			b.Battlegroups = append(b.Battlegroups[:i], b.Battlegroups[i+1:]...)
			break
		}
	}
}

var Battlegroups = BattlegroupList{}

func init() {
	Universe.RegisterBattlegroups(&Battlegroups)
}
