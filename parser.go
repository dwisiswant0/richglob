package richglob

import (
	"runtime"
	"unicode/utf8"
)

type parser struct {
	input string
	pos   int
	cfg   config
}

func (p *parser) parseUntil(stop byte) (*sequence, error) {
	seq := &sequence{asciiOnly: true}
	literalRunes := make([]rune, 0, 8)
	flushLiteral := func() {
		if len(literalRunes) == 0 {
			return
		}

		lit := make([]rune, len(literalRunes))
		copy(lit, literalRunes)

		seq.nodes = append(seq.nodes, node{kind: nodeLiteral, lit: lit})
		literalRunes = literalRunes[:0]
	}

	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if stop != 0 && ch == stop {
			break
		}

		if stop == ')' && ch == '|' {
			break
		}

		switch ch {
		case '[':
			flushLiteral()

			class, err := p.parseClass()
			if err != nil {
				return nil, err
			}

			if seq.asciiOnly && !class.isASCIIOnly() {
				seq.asciiOnly = false
			}

			seq.nodes = append(seq.nodes, node{kind: nodeClass, class: class})
		case '?', '*', '+', '@', '!':
			if p.cfg.extglob && p.pos+1 < len(p.input) && p.input[p.pos+1] == '(' {
				flushLiteral()

				ext, err := p.parseExt(ch)
				if err != nil {
					return nil, err
				}

				seq.nodes = append(seq.nodes, node{kind: nodeExt, ext: ext})
				seq.hasExt = true

				continue
			}

			switch ch {
			case '?':
				flushLiteral()
				p.pos++
				seq.nodes = append(seq.nodes, node{kind: nodeAny})
			case '*':
				flushLiteral()
				p.pos++
				seq.nodes = append(seq.nodes, node{kind: nodeStar})
			default:
				lit, err := p.parseLiteralRune()
				if err != nil {
					return nil, err
				}

				if seq.asciiOnly && lit >= utf8.RuneSelf {
					seq.asciiOnly = false
				}

				literalRunes = append(literalRunes, lit)
			}
		case '\\':
			lit, err := p.parseLiteralRune()
			if err != nil {
				return nil, err
			}

			if seq.asciiOnly && lit >= utf8.RuneSelf {
				seq.asciiOnly = false
			}

			literalRunes = append(literalRunes, lit)
		default:
			lit, err := p.parseLiteralRune()
			if err != nil {
				return nil, err
			}

			if seq.asciiOnly && lit >= utf8.RuneSelf {
				seq.asciiOnly = false
			}

			literalRunes = append(literalRunes, lit)
		}
	}

	flushLiteral()

	return seq, nil
}

func (p *parser) parseExt(op byte) (*extGroup, error) {
	p.pos += 2
	group := &extGroup{op: op}

	for {
		alt, err := p.parseUntil(')')
		if err != nil {
			return nil, err
		}

		group.alts = append(group.alts, alt)

		if p.pos >= len(p.input) {
			return nil, ErrBadPattern
		}

		if p.input[p.pos] == ')' {
			p.pos++
			break
		}

		if p.input[p.pos] != '|' {
			return nil, ErrBadPattern
		}

		p.pos++
	}

	return group, nil
}

func (class *charClass) isASCIIOnly() bool {
	for _, current := range class.ranges {
		if current.lo >= utf8.RuneSelf || current.hi >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

func (p *parser) parseClass() (*charClass, error) {
	p.pos++

	if p.pos >= len(p.input) {
		return nil, ErrBadPattern
	}

	class := &charClass{}

	if p.input[p.pos] == '^' {
		class.negated = true
		p.pos++
	}

	count := 0

	for {
		if p.pos >= len(p.input) {
			return nil, ErrBadPattern
		}

		if p.input[p.pos] == ']' && count > 0 {
			p.pos++
			break
		}

		lo, err := p.parseClassRune()
		if err != nil {
			return nil, err
		}

		hi := lo

		if p.pos < len(p.input) && p.input[p.pos] == '-' {
			p.pos++

			hi, err = p.parseClassRune()
			if err != nil {
				return nil, err
			}
		}

		class.ranges = append(class.ranges, charRange{lo: lo, hi: hi})
		count++
	}

	return class, nil
}

func (p *parser) parseLiteralRune() (rune, error) {
	if p.pos >= len(p.input) {
		return 0, ErrBadPattern
	}

	if p.input[p.pos] == '\\' && runtime.GOOS != "windows" {
		p.pos++
		if p.pos >= len(p.input) {
			return 0, ErrBadPattern
		}
	}

	r, size := utf8.DecodeRuneInString(p.input[p.pos:])
	if r == utf8.RuneError && size == 1 {
		return 0, ErrBadPattern
	}

	p.pos += size

	return r, nil
}

func (p *parser) parseClassRune() (rune, error) {
	if p.pos >= len(p.input) || p.input[p.pos] == '-' || p.input[p.pos] == ']' {
		return 0, ErrBadPattern
	}

	if p.input[p.pos] == '\\' && runtime.GOOS != "windows" {
		p.pos++
		if p.pos >= len(p.input) {
			return 0, ErrBadPattern
		}
	}

	r, size := utf8.DecodeRuneInString(p.input[p.pos:])
	if r == utf8.RuneError && size == 1 {
		return 0, ErrBadPattern
	}

	p.pos += size

	return r, nil
}
