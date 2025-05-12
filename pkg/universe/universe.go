package universe

import (
	"github.com/sbelectronics/galwar/pkg/interfaces"
)

type UniverseType struct {
	ObjLists []interfaces.ObjectListInterface
}

func (u *UniverseType) GetObjectsInSector(sector int, kind string) []interfaces.ObjectInterface {
	objects := []interfaces.ObjectInterface{}

	for _, objList := range u.ObjLists {
		objItems := objList.GetObjectsInSector(sector)
		for _, obj := range objItems {
			if obj.GetType() == kind {
				objects = append(objects, obj)
			}
		}
	}

	return objects
}

func (u *UniverseType) Register(objList interfaces.ObjectListInterface) {
	u.ObjLists = append(u.ObjLists, objList)
}

var Universe UniverseType
