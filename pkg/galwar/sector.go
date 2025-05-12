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

func (s *Sector) HasWarp(warp int) bool {
	for _, w := range s.Warps {
		if w == warp {
			return true
		}
	}
	return false
}

func (s *Sector) ShortestPathTo(target int) []int {
	visited := make(map[int]bool)
	distances := make(map[int]int)
	previous := make(map[int]int)

	for _, sector := range Sectors {
		distances[sector.Number] = int(^uint(0) >> 1) // Set to max int
	}
	distances[s.Number] = 0

	current := s.Number
	for current != target {
		visited[current] = true

		// Update distances for neighbors
		for _, neighbor := range Sectors[current].Warps {
			if visited[neighbor] {
				continue
			}
			newDist := distances[current] + 1
			if newDist < distances[neighbor] {
				distances[neighbor] = newDist
				previous[neighbor] = current
			}
		}

		// Find the next unvisited sector with the smallest distance
		minDist := int(^uint(0) >> 1)
		next := -1
		for _, sector := range Sectors {
			if !visited[sector.Number] && distances[sector.Number] < minDist {
				minDist = distances[sector.Number]
				next = sector.Number
			}
		}

		if next == -1 {
			return nil // No path exists
		}
		current = next
	}

	// Reconstruct the path
	path := []int{}
	for at := target; at != s.Number; at = previous[at] {
		path = append([]int{at}, path...)
	}
	path = append([]int{s.Number}, path...)
	return path
}
