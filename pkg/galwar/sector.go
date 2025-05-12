package galwar

import "sort"

type Sector struct {
	Number int
	Warps  []int
}

var Sectors []Sector

func (s *Sector) GetName() string {
	return "Sector " + string(s.Number)
}

func (s *Sector) GetWarps() []int {
	return s.Warps
}

func (s *Sector) GetNumber() int {
	return s.Number
}

func (s *Sector) AddWarp(warp int) {
	for _, w := range s.Warps {
		if w == warp {
			return
		}
	}
	s.Warps = append(s.Warps, warp)
	sort.Ints(s.Warps)
}
