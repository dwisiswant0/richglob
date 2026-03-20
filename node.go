package richglob

type nodeKind int

const (
	nodeLiteral nodeKind = iota
	nodeAny
	nodeStar
	nodeClass
	nodeExt
)

type node struct {
	kind  nodeKind
	lit   []rune
	class *charClass
	ext   *extGroup
}

func matchNodes(nodes []node, nodeIndex int, input []rune, pos int, cfg config) []int {
	if nodeIndex == len(nodes) {
		return []int{pos}
	}

	positions := nodes[nodeIndex].match(input, pos, cfg)
	if len(positions) == 0 {
		return nil
	}

	results := make([]int, 0, len(positions))
	for _, next := range positions {
		for _, end := range matchNodes(nodes, nodeIndex+1, input, next, cfg) {
			results = appendUniqInt(results, end)
		}
	}

	return results
}

func (item node) match(input []rune, pos int, cfg config) []int {
	switch item.kind {
	case nodeLiteral:
		if pos+len(item.lit) > len(input) {
			return nil
		}

		for idx, r := range item.lit {
			if !runesEq(r, input[pos+idx], cfg) {
				return nil
			}
		}

		return []int{pos + len(item.lit)}
	case nodeAny:
		if pos >= len(input) {
			return nil
		}

		return []int{pos + 1}
	case nodeStar:
		results := make([]int, 0, len(input)-pos+1)
		for next := pos; next <= len(input); next++ {
			results = append(results, next)
		}

		return results
	case nodeClass:
		if pos >= len(input) {
			return nil
		}

		if item.class.matchRune(input[pos], cfg) {
			return []int{pos + 1}
		}

		return nil
	case nodeExt:
		return item.ext.match(input, pos, cfg)
	default:
		return nil
	}
}

func (item node) matchesLeadingDotExplicitly(cfg config) bool {
	switch item.kind {
	case nodeLiteral:
		return len(item.lit) > 0 && item.lit[0] == '.'
	case nodeClass:
		return item.class.matchRune('.', cfg)
	case nodeExt:
		for _, alt := range item.ext.alts {
			if alt.matchesLeadingDotExplicitly(cfg) {
				return true
			}
		}

		return false
	default:
		return false
	}
}
