package galwar

import (
	"fmt"
	"strings"
)

type Battlegroup struct {
	Owner PlayerId
	ObjectBase
	InventoryBase

	universe *UniverseType // back-reference for owner lookup; set by wire()/NewBattlegroup, not serialized
}

type BattlegroupList struct {
	Battlegroups []*Battlegroup
}

func (u *UniverseType) NewBattlegroup(owner PlayerId, sector int) *Battlegroup {
	b := &Battlegroup{
		Owner: owner,
		ObjectBase: ObjectBase{
			Name:   "",
			Sector: sector,
		},
		InventoryBase: InventoryBase{},
		universe:      u,
	}

	u.Battlegroups.AddBattlegroup(b)

	return b
}

func (b *Battlegroup) GetOwnerPlayer() *Player {
	if b.universe != nil {
		if player := b.universe.Players.GetById(b.Owner); player != nil {
			return player
		}
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

// GetBattlegroup finds the caller's battlegroup in a sector, examining every
// battlegroup there before deciding. One defense force per sector, as in the
// original: if someone else already defends the sector, you can't.
// If !create and no group exists, will return nil and no error.

func (u *UniverseType) GetBattlegroup(player *Player, sector int, create bool) (*Battlegroup, error) {
	var foreign *Battlegroup
	for _, bg := range u.Battlegroups.Battlegroups {
		if bg.Sector != sector {
			continue
		}
		if bg.Owner == player.Id {
			return bg, nil
		}
		if foreign == nil {
			foreign = bg
		}
	}
	if foreign != nil {
		return nil, NewGameError(ErrNotOwner, "There is a battlegroup in this sector, but you don't own it.")
	}
	if create {
		return u.NewBattlegroup(player.Id, sector), nil
	}
	return nil, nil
}

// AdjustBattlegroup sets the sector-side quantity of fighters or mines,
// moving the difference to or from the player.

func (u *UniverseType) AdjustBattlegroup(player *Player, sector int, kind string, amount int) error {
	if amount < 0 {
		return NewGameError(ErrNegativeQuantity, "You can't leave a negative quantity.")
	}
	if sector <= 10 {
		return NewGameError(ErrFedRestricted, "Galactic law prevents anyone except the federation from leaving fighters or mines in sectors 1 through 10.")
	}
	bg, err := u.GetBattlegroup(player, sector, true)
	if err != nil {
		return err
	}
	total := bg.GetQuantity(kind) + player.GetQuantity(kind)
	if amount > total {
		if !bg.HasInventory() {
			u.Battlegroups.RemoveBattlegroup(bg)
		}
		return NewGameError(ErrNotEnoughQuantity, fmt.Sprintf("You don't have that many %s!", kind))
	}
	bg.SetQuantity(kind, amount)
	player.SetQuantity(kind, total-amount)
	if !bg.HasInventory() {
		u.Battlegroups.RemoveBattlegroup(bg)
	}
	return nil
}

func (b *BattlegroupList) AddBattlegroup(bg *Battlegroup) {
	b.Battlegroups = append(b.Battlegroups, bg)
}

func (b *BattlegroupList) RemoveBattlegroup(bg *Battlegroup) {
	for i, battlegroup := range b.Battlegroups {
		if battlegroup == bg {
			b.Battlegroups = append(b.Battlegroups[:i], b.Battlegroups[i+1:]...)
			break
		}
	}
}
