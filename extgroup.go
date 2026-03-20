package richglob

type extGroup struct {
	op   byte
	alts []*sequence
}

func (group *extGroup) match(input []rune, pos int, cfg config) []int {
	switch group.op {
	case '@':
		return group.matchAlts(input, pos, cfg)
	case '?':
		results := []int{pos}
		for _, end := range group.matchAlts(input, pos, cfg) {
			results = appendUniqInt(results, end)
		}
		return results
	case '*':
		return group.repeat(input, pos, cfg, false, map[int]bool{})
	case '+':
		return group.repeat(input, pos, cfg, true, map[int]bool{})
	case '!':
		results := make([]int, 0, len(input)-pos+1)
		for end := pos; end <= len(input); end++ {
			if !group.matchesAnyWhole(input[pos:end], cfg) {
				results = append(results, end)
			}
		}
		return results
	default:
		return nil
	}
}

func (group *extGroup) repeat(input []rune, pos int, cfg config, requireOne bool, visiting map[int]bool) []int {
	results := make([]int, 0)
	if !requireOne {
		results = append(results, pos)
	}

	if visiting[pos] {
		return results
	}

	visiting[pos] = true

	for _, alt := range group.alts {
		for _, end := range alt.matchFrom(input, pos, cfg) {
			results = appendUniqInt(results, end)
			for _, next := range group.repeat(input, end, cfg, false, visiting) {
				results = appendUniqInt(results, next)
			}
		}
	}

	delete(visiting, pos)

	return results
}

func (group *extGroup) matchAlts(input []rune, pos int, cfg config) []int {
	results := make([]int, 0)
	for _, alt := range group.alts {
		for _, end := range alt.matchFrom(input, pos, cfg) {
			results = appendUniqInt(results, end)
		}
	}

	return results
}

func (group *extGroup) matchesAnyWhole(input []rune, cfg config) bool {
	text := string(input)
	for _, alt := range group.alts {
		if alt.matchString(text, cfg) {
			return true
		}
	}

	return false
}
