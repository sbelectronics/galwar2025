package galwar

import (
	"gopkg.in/yaml.v2"
	"os"
)

type UniverseType struct {
	Ports        *PortList
	Players      *PlayerList
	Battlegroups *BattlegroupList
	Sectors      *[]Sector
	filename     string
}

func (u *UniverseType) SetFilename(filename string) {
	u.filename = filename
}

func (u *UniverseType) FileExist() bool {
	if _, err := os.Stat(u.filename); err == nil {
		return true
	}
	return false
}

func (u *UniverseType) Save() error {
	data, err := yaml.Marshal(u)
	if err != nil {
		return err
	}

	err = os.WriteFile(u.filename, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (u *UniverseType) Load() error {
	data, err := os.ReadFile(u.filename)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, u)
	if err != nil {
		return err
	}

	return nil
}

func (u *UniverseType) GetObjectsInSector(sector int, kind string) []ObjectInterface {
	objects := []ObjectInterface{}

	// Be deterministic about the order we display things
	objLists := []ObjectListInterface{u.Ports, u.Players, u.Battlegroups}
	for _, objList := range objLists {
		if objList == nil {
			continue
		}
		objItems := objList.GetObjectsInSector(sector)
		for _, obj := range objItems {
			if (kind == "") || (obj.GetType() == kind) {
				objects = append(objects, obj)
			}
		}
	}

	return objects
}

// These Register functions seem kinda boneheaded. What I intended to do was to register
// Ports, Player, Planets, etc., as ObjectListInterface, but when I went to serialize them
// this was an issue.

func (u *UniverseType) RegisterPorts(ports *PortList) {
	u.Ports = ports
}

func (u *UniverseType) RegisterPlayers(players *PlayerList) {
	u.Players = players
}

func (u *UniverseType) RegisterBattlegroups(battlegroups *BattlegroupList) {
	u.Battlegroups = battlegroups
}

func (u *UniverseType) RegisterSectors(sectors *[]Sector) {
	u.Sectors = sectors
}

var Universe UniverseType
