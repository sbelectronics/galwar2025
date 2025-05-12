package initgame

import (
	"github.com/sbelectronics/galwar/pkg/port"
	"github.com/sbelectronics/galwar/pkg/sector"
	"github.com/sbelectronics/galwar/pkg/universe"
	"math/rand"
)

func AddPortToSector(sectorNum int) {
	i := len(port.Ports.Ports)
	p := port.Port{
		Name:   PortNames[i],
		Sector: sectorNum,
	}
	port.Ports.Ports = append(port.Ports.Ports, &p)
}

func InitSectors(numsec int) {
	for i := 0; i <= numsec; i++ {
		sec := sector.Sector{
			Number: i,
			Warps:  []int{},
		}
		if i > 1 {
			sec.AddWarp(i - 1)
		}
		if i < numsec {
			sec.AddWarp(i + 1)
		}
		sector.Sectors = append(sector.Sectors, sec)
	}

	for i := 2; i <= 9; i++ {
		sector.Sectors[1].AddWarp(i)
		sector.Sectors[i].AddWarp(1)
	}

	AddPortToSector(1) // Sol

	for a := 11; a <= 425*numsec/2000; a++ {
		for {
			b := 1 + rand.Intn(numsec-1)
			portsThisSector := universe.Universe.GetObjectsInSector(b, "Port")
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
		sector.Sectors[firstSec].AddWarp(secondSec)
		sector.Sectors[secondSec].AddWarp(firstSec)
	}

	for a := 1; a <= 250; a++ {
		b := 1 + rand.Intn(numsec-1)
		j := rand.Intn(2)
		c := sector.Sectors[b].Warps[j]
		g := 1 + rand.Intn(numsec-1)
		// TODO: the relinking thing...
		_ = c
		_ = g
	}
}
