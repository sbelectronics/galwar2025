package galwar

import (
	"math/rand"
)

func AddPortToSector(sectorNum int) {
	i := len(Ports.Ports)
	p := &Port{
		ObjectBase: ObjectBase{
			Name:   PortNames[i],
			Sector: sectorNum,
		},
	}

	// special ports
	switch i {
	case 0:
		p.Goods = Sol
		for _, tg := range TradeGoods {
			if !tg.SellAtSol {
				continue
			}
			cm := tg.Commodity
			cm.Sell = true
			p.Inventory = append(p.Inventory, &cm)
		}
		break
	default:
		toSell := rand.Intn(len(TradeGoods))
		for i, tg := range TradeGoods {
			if !tg.SellAtPorts {
				continue
			}
			cm := tg.Commodity
			cm.Prod = 50 + rand.Intn(400)
			cm.Quantity = cm.Prod * 10
			cm.BuyPrice = float64(tg.BuyPrice) * (float64(rand.Intn(10)) + 100.0) / 100.0   // up to 10% difference
			cm.SellPrice = float64(tg.SellPrice) * (float64(rand.Intn(10)) + 100.0) / 100.0 // up to 10% difference
			cm.Sell = (i == toSell)
			p.Inventory = append(p.Inventory, &cm)
		}
		break
	}

	Ports.Ports = append(Ports.Ports, p)
}

func randSec(numsec int) int {
	return 1 + rand.Intn(numsec-1)
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

	AddPortToSector(3) // for reproducibility

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
			firstSec = randSec(numsec)
			secondSec = randSec(numsec)
			if (firstSec != secondSec) && (firstSec/100 == secondSec/100) {
				break
			}
		}
		Sectors[firstSec].AddWarp(secondSec)
		Sectors[secondSec].AddWarp(firstSec)
	}

	for a := 1; a <= 250; a++ {
		b := randSec(numsec) // pick a sector to relink
		j := rand.Intn(2)    // pick one of the first two links
		c := Sectors[b].Warps[j]
		g := randSec(numsec) // pick a sector to relink to

		Sectors[b].RemoveWarp(c) // remove the old link
		Sectors[b].AddWarp(g)
		Sectors[g].AddWarp(b)

		g = randSec(numsec)
		Sectors[c].RemoveWarp(b) // remove the old link
		Sectors[c].AddWarp(g)
		Sectors[g].AddWarp(c)
	}

	// Make dead ends

	for a := 1; a <= numsec*20/2000; a++ {
		secnum := 20 + randSec(numsec-20) // pick a sector after sector 20

		if len(Sectors[secnum].Warps) == 0 {
			// how did this happen?
			continue
		}

		warpToKeep := rand.Intn(len(Sectors[secnum].Warps))
		destToKeep := Sectors[secnum].Warps[warpToKeep]

		// dest to keep is the destination we will keep

		// remove everything else
		for len(Sectors[secnum].Warps) > 0 {
			destToRemove := Sectors[secnum].Warps[0]
			Sectors[secnum].RemoveWarp(destToRemove)
			Sectors[destToRemove].RemoveWarp(secnum)
		}

		// Put the one we wanted to keep back in
		Sectors[secnum].AddWarp(destToKeep)
		Sectors[destToKeep].AddWarp(secnum)
	}
}
