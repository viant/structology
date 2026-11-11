package tagutil

import "fmt"

// NameCandidate identifies a declared leaf after the configured wire transform.
// Callers resolve values/exclusions only after this declaration-only decision.
type NameCandidate struct {
	Name     string
	Depth    int
	Explicit bool
}

func NameWinners(fields []NameCandidate) (map[int]bool, error) {
	groups := map[string][]int{}
	for i, f := range fields {
		groups[f.Name] = append(groups[f.Name], i)
	}
	keep := map[int]bool{}
	for name, indices := range groups {
		depth := int(^uint(0) >> 1)
		for _, i := range indices {
			if fields[i].Depth < depth {
				depth = fields[i].Depth
			}
		}
		var candidates, tagged []int
		for _, i := range indices {
			if fields[i].Depth == depth {
				candidates = append(candidates, i)
				if fields[i].Explicit {
					tagged = append(tagged, i)
				}
			}
		}
		if len(tagged) > 0 {
			candidates = tagged
		}
		if len(candidates) != 1 {
			return nil, fmt.Errorf("JSON name collision %q", name)
		}
		keep[candidates[0]] = true
	}
	return keep, nil
}
