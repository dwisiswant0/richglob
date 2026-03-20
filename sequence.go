package richglob

import (
	"slices"
	"strings"
	"unicode/utf8"
)

type sequence struct {
	nodes     []node
	hasExt    bool
	asciiOnly bool
	fast      seqFastMatcher
}

type seqFastKind uint8

const (
	sequenceFastNone seqFastKind = iota
	sequenceFastStar
	sequenceFastLiteralStarLiteral
)

type seqFastMatcher struct {
	kind   seqFastKind
	prefix string
	suffix string
	minLen int
}

func (seq *sequence) literalVal() (string, bool) {
	if len(seq.nodes) == 0 {
		return "", true
	}

	var builder strings.Builder

	for _, item := range seq.nodes {
		if item.kind != nodeLiteral {
			return "", false
		}

		builder.WriteString(string(item.lit))
	}

	return builder.String(), true
}

func (seq *sequence) matchesLeadingDotExplicitly(cfg config) bool {
	if len(seq.nodes) == 0 {
		return false
	}

	return seq.nodes[0].matchesLeadingDotExplicitly(cfg)
}

func (seq *sequence) matchString(s string, cfg config) bool {
	if !seq.hasExt {
		if !cfg.noCase {
			if matched, ok := seq.matchFastString(s); ok {
				return matched
			}
		}

		if seq.asciiOnly && isASCIIString(s) {
			return seq.matchSimpleASCII(s, cfg)
		}

		runes := []rune(s)

		return seq.matchSimple(runes, cfg)
	}

	runes := []rune(s)
	return slices.Contains(seq.matchFrom(runes, 0, cfg), len(runes))
}

func (seq *sequence) matchFastString(input string) (bool, bool) {
	switch seq.fast.kind {
	case sequenceFastStar:
		return true, true
	case sequenceFastLiteralStarLiteral:
		if len(input) < seq.fast.minLen {
			return false, true
		}

		if seq.fast.prefix != "" && !strings.HasPrefix(input, seq.fast.prefix) {
			return false, true
		}

		if seq.fast.suffix != "" && !strings.HasSuffix(input, seq.fast.suffix) {
			return false, true
		}

		return true, true
	default:
		return false, false
	}
}

func (seq *sequence) initFastMatcher() {
	if seq.hasExt || !seq.asciiOnly {
		return
	}

	if len(seq.nodes) == 1 && seq.nodes[0].kind == nodeStar {
		seq.fast.kind = sequenceFastStar
		return
	}

	starIndex := -1
	for idx, item := range seq.nodes {
		switch item.kind {
		case nodeLiteral:
		case nodeStar:
			if starIndex >= 0 {
				return
			}
			starIndex = idx
		default:
			return
		}
	}

	if starIndex < 0 {
		return
	}

	seq.fast.kind = sequenceFastLiteralStarLiteral
	for _, item := range seq.nodes[:starIndex] {
		seq.fast.prefix += string(item.lit)
		seq.fast.minLen += len(item.lit)
	}

	for _, item := range seq.nodes[starIndex+1:] {
		seq.fast.suffix += string(item.lit)
		seq.fast.minLen += len(item.lit)
	}
}

func (seq *sequence) matchSimpleASCII(input string, cfg config) bool {
	nodePos := 0
	inputPos := 0
	starNodePos := -1
	starInputPos := -1

	for inputPos < len(input) {
		if nodePos < len(seq.nodes) {
			item := seq.nodes[nodePos]

			switch item.kind {
			case nodeLiteral:
				if matchLiteralASCII(item.lit, input, inputPos, cfg) {
					inputPos += len(item.lit)
					nodePos++
					continue
				}
			case nodeAny:
				inputPos++
				nodePos++
				continue
			case nodeClass:
				if item.class.matchRune(rune(input[inputPos]), cfg) {
					inputPos++
					nodePos++
					continue
				}
			case nodeStar:
				starNodePos = nodePos
				starInputPos = inputPos
				nodePos++
				continue
			}
		}

		if starNodePos < 0 {
			return false
		}

		starInputPos++
		inputPos = starInputPos
		nodePos = starNodePos + 1
	}

	for nodePos < len(seq.nodes) && seq.nodes[nodePos].kind == nodeStar {
		nodePos++
	}

	return nodePos == len(seq.nodes)
}

func (seq *sequence) matchSimple(input []rune, cfg config) bool {
	nodePos := 0
	inputPos := 0
	starNodePos := -1
	starInputPos := -1

	for inputPos < len(input) {
		if nodePos < len(seq.nodes) {
			item := seq.nodes[nodePos]

			switch item.kind {
			case nodeLiteral:
				if matchLiteralRunes(item.lit, input, inputPos, cfg) {
					inputPos += len(item.lit)
					nodePos++
					continue
				}
			case nodeAny:
				inputPos++
				nodePos++
				continue
			case nodeClass:
				if item.class.matchRune(input[inputPos], cfg) {
					inputPos++
					nodePos++
					continue
				}
			case nodeStar:
				starNodePos = nodePos
				starInputPos = inputPos
				nodePos++
				continue
			}
		}

		if starNodePos < 0 {
			return false
		}

		starInputPos++
		inputPos = starInputPos
		nodePos = starNodePos + 1
	}

	for nodePos < len(seq.nodes) && seq.nodes[nodePos].kind == nodeStar {
		nodePos++
	}

	return nodePos == len(seq.nodes)
}

func matchLiteralRunes(literal, input []rune, pos int, cfg config) bool {
	if pos+len(literal) > len(input) {
		return false
	}

	for idx, r := range literal {
		if !runesEq(r, input[pos+idx], cfg) {
			return false
		}
	}

	return true
}

func matchLiteralASCII(literal []rune, input string, pos int, cfg config) bool {
	if pos+len(literal) > len(input) {
		return false
	}

	for idx, r := range literal {
		if !runesEq(r, rune(input[pos+idx]), cfg) {
			return false
		}
	}

	return true
}

func isASCIIString(s string) bool {
	for idx := 0; idx < len(s); idx++ {
		if s[idx] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

func (seq *sequence) matchFrom(input []rune, pos int, cfg config) []int {
	return matchNodes(seq.nodes, 0, input, pos, cfg)
}
