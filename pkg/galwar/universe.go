package galwar

type UniverseType struct {
	ObjLists []ObjectListInterface
}

func (u *UniverseType) GetObjectsInSector(sector int, kind string) []ObjectInterface {
	objects := []ObjectInterface{}

	for _, objList := range u.ObjLists {
		objItems := objList.GetObjectsInSector(sector)
		for _, obj := range objItems {
			if (kind == "") || (obj.GetType() == kind) {
				objects = append(objects, obj)
			}
		}
	}

	return objects
}

func (u *UniverseType) Register(objList ObjectListInterface) {
	u.ObjLists = append(u.ObjLists, objList)
}

var Universe UniverseType
