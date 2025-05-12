package galwar

import (
	"math/rand"
)

func AddPortToSector(sectorNum int) {
	i := len(Ports.Ports)
	p := Port{
		Name:   PortNames[i],
		Sector: sectorNum,
	}
	Ports.Ports = append(Ports.Ports, &p)
}

func InitSectors(numsec int) {
	for i := 0; i <= numsec; i++ {
		sec := Sector{
			Number: i,
			Warps:  []int{},
		}
		if i > 1 {
			sec.AddWarp(i - 1)
		}
		if i < numsec {
			sec.AddWarp(i + 1)
		}
		Sectors = append(Sectors, sec)
	}

	for i := 2; i <= 9; i++ {
		Sectors[1].AddWarp(i)
		Sectors[i].AddWarp(1)
	}

	AddPortToSector(1) // Sol

	for a := 11; a <= 425*numsec/2000; a++ {
		for {
			b := 1 + rand.Intn(numsec-1)
			portsThisSector := Universe.GetObjectsInSector(b, "Port")
			if len(portsThisSector) == 0 {
				AddPortToSector(b)
				break
			}
		}
	}

	// a bunch of wonky logic from the original code

	for a := 1; a <= numsec/2; a++ {
		var firstSec int
		var secondSec int
		for {
			firstSec = 1 + rand.Intn(numsec-1)
			secondSec = 1 + rand.Intn(numsec-1)
			if (firstSec != secondSec) && (firstSec/100 == secondSec/100) {
				break
			}
		}
		Sectors[firstSec].AddWarp(secondSec)
		Sectors[secondSec].AddWarp(firstSec)
	}

	for a := 1; a <= 250; a++ {
		b := 1 + rand.Intn(numsec-1)
		j := rand.Intn(2)
		c := Sectors[b].Warps[j]
		g := 1 + rand.Intn(numsec-1)
		// TODO: the relinking thing...
		_ = c
		_ = g
	}
}
