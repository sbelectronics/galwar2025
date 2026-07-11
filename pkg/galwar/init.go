package galwar

import (
	"math/rand"
)

func (u *UniverseType) AddPortToSector(sectorNum int) {
	i := len(u.Ports.Ports)
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
	default:
		for _, tg := range TradeGoods {
			if !tg.SellAtPorts {
				continue
			}
			cm := tg.Commodity
			cm.Prod = 50 + rand.Intn(400)
			cm.Quantity = cm.Prod * 10
			cm.BuyPrice = float64(tg.BuyPrice) * (float64(rand.Intn(10)) + 100.0) / 100.0   // up to 10% difference
			cm.SellPrice = float64(tg.SellPrice) * (float64(rand.Intn(10)) + 100.0) / 100.0 // up to 10% difference
			p.Inventory = append(p.Inventory, &cm)
		}
		if len(p.Inventory) > 0 {
			toSell := rand.Intn(len(p.Inventory))
			p.Inventory[toSell].Sell = true
		}
	}

	u.Ports.Ports = append(u.Ports.Ports, p)
}

// randSec returns a random valid sector number, 1..numsec inclusive.
func randSec(numsec int) int {
	return 1 + rand.Intn(numsec)
}

// Generate creates a brand-new universe of numsec sectors. The layout
// algorithm follows the original Pascal generator.
func (u *UniverseType) Generate(numsec int) {
	for i := 0; i <= numsec; i++ {
		sec := Sector{
			Number: i,
			Warps:  []int{},
		}
		if i > 1 {
			sec.AddWarp(i - 1)
		}
		if (i >= 1) && (i < numsec) {
			sec.AddWarp(i + 1)
		}
		u.Sectors = append(u.Sectors, sec)
	}

	for i := 2; i <= 9; i++ {
		u.Sectors[1].AddWarp(i)
		u.Sectors[i].AddWarp(1)
	}

	u.AddPortToSector(1) // Sol

	u.AddPortToSector(3) // for reproducibility

	for a := 11; a <= 425*numsec/2000; a++ {
		for {
			b := randSec(numsec)
			portsThisSector := u.GetObjectsInSector(b, "Port")
			if len(portsThisSector) == 0 {
				u.AddPortToSector(b)
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
		u.Sectors[firstSec].AddWarp(secondSec)
		u.Sectors[secondSec].AddWarp(firstSec)
	}

	for a := 1; a <= 250; a++ {
		b := randSec(numsec) // pick a sector to relink
		if len(u.Sectors[b].Warps) < 2 {
			// relinking can shrink warp counts (re-adding an existing warp
			// is a no-op); don't index Warps[1] on a one-warp sector
			continue
		}
		j := rand.Intn(2) // pick one of the first two links
		c := u.Sectors[b].Warps[j]
		g := randSec(numsec) // pick a sector to relink to

		u.Sectors[b].RemoveWarp(c) // remove the old link
		if g != b {
			u.Sectors[b].AddWarp(g)
			u.Sectors[g].AddWarp(b)
		}

		g = randSec(numsec)
		u.Sectors[c].RemoveWarp(b) // remove the old link
		if g != c {
			u.Sectors[c].AddWarp(g)
			u.Sectors[g].AddWarp(c)
		}
	}

	// Make dead ends

	for a := 1; a <= numsec*20/2000; a++ {
		secnum := 20 + randSec(numsec-20) // pick a sector after sector 20

		if len(u.Sectors[secnum].Warps) == 0 {
			// how did this happen?
			continue
		}

		warpToKeep := rand.Intn(len(u.Sectors[secnum].Warps))
		destToKeep := u.Sectors[secnum].Warps[warpToKeep]

		// dest to keep is the destination we will keep

		// remove everything else
		for len(u.Sectors[secnum].Warps) > 0 {
			destToRemove := u.Sectors[secnum].Warps[0]
			u.Sectors[secnum].RemoveWarp(destToRemove)
			u.Sectors[destToRemove].RemoveWarp(secnum)
		}

		// Put the one we wanted to keep back in
		u.Sectors[secnum].AddWarp(destToKeep)
		u.Sectors[destToKeep].AddWarp(secnum)
	}

	// The wonky logic above can strand sectors (the original had an optional
	// CheckWarps relinker for the same reason). Guarantee every sector is
	// reachable from sector 1.
	u.repairConnectivity()
}

// repairConnectivity links any sector unreachable from sector 1 back into
// the connected component with a two-way warp. Returns the number of links
// added.
func (u *UniverseType) repairConnectivity() int {
	numsec := len(u.Sectors) - 1
	reachable := make([]bool, len(u.Sectors))

	bfs := func(start int) {
		queue := []int{start}
		reachable[start] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, w := range u.Sectors[cur].Warps {
				if w >= 1 && w <= numsec && !reachable[w] {
					reachable[w] = true
					queue = append(queue, w)
				}
			}
		}
	}
	bfs(1)

	relinked := 0
	for s := 2; s <= numsec; s++ {
		if reachable[s] {
			continue
		}
		for {
			t := randSec(numsec)
			if reachable[t] {
				u.Sectors[s].AddWarp(t)
				u.Sectors[t].AddWarp(s)
				break
			}
		}
		relinked++
		bfs(s) // everything in s's component is now reachable too
	}
	return relinked
}
