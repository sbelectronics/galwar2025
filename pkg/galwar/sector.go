package galwar

import (
	"fmt"
	"sort"
)

type Sector struct {
	Number int
	Warps  []int
}

func (s *Sector) GetName() string {
	return fmt.Sprintf("Sector %d", s.Number)
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

func (s *Sector) RemoveWarp(warp int) {
	for i, w := range s.Warps {
		if w == warp {
			s.Warps = append(s.Warps[:i], s.Warps[i+1:]...)
			return
		}
	}
}

func (s *Sector) HasWarp(warp int) bool {
	for _, w := range s.Warps {
		if w == warp {
			return true
		}
	}
	return false
}
