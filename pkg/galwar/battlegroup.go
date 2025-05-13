package galwar

import ()

type Battlegroup struct {
	owner  PlayerId
	Name   string
	Sector int
	InventoryBase
}

type BattlegroupList struct {
	Battlegroups []*Battlegroup
}

func (b *Battlegroup) GetName() string {
	return b.Name
}

func (b *Battlegroup) GetNameExtra() string {
	return ""
}

func (b *Battlegroup) GetType() string {
	return "Battlegroup"
}

func (b *Battlegroup) GetSector() int {
	return b.Sector
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

var Battlegroups = BattlegroupList{}

func init() {
	Universe.RegisterBattlegroups(&Battlegroups)
}
