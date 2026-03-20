package richglob

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Separator is the operating system-specific path separator used by patterns.
const Separator = os.PathSeparator

type compiledPath struct {
	volume           string
	absolute         bool
	dirOnly          bool
	hasMeta          bool
	globstars        int
	prefixLen        int
	segments         []compiledSegment
	literalPathValue string
	walkBaseValue    string
}

type compiledSegment struct {
	seq        *sequence
	literal    string
	meta       bool
	globstar   bool
	leadingDot bool
}

type charClass struct {
	negated bool
	ranges  []charRange
}

type charRange struct {
	lo rune
	hi rune
}

// Match reports whether name matches pattern.
//
// Pattern syntax is similar to [filepath.Match], with optional extensions
// enabled through [Option] values.
func Match(pattern, name string, opts ...Option) (bool, error) {
	cfg := buildConfig(opts)

	compiled, err := compileMatchPattern(pattern, cfg)
	if err != nil {
		return false, err
	}

	return compiled.matchName(name, cfg), nil
}

func compileMatchPattern(pattern string, cfg config) (*compiledPath, error) {
	cacheKey := makeCompileCacheKey(compileCacheMatch, pattern, cfg)
	if compiled, ok := globalCompiledPatternCache.Get(cacheKey); ok {
		return compiled, nil
	}

	parts := splitMatchPath(pattern)
	segments := make([]compiledSegment, 0, len(parts.segments))
	globstars := 0

	for _, raw := range parts.segments {
		segment, err := compileSegment(raw, cfg)
		if err != nil {
			return nil, err
		}

		if segment.globstar {
			globstars++
		}

		segments = append(segments, segment)
	}

	compiled := &compiledPath{volume: parts.volume, absolute: parts.absolute, hasMeta: hasMetaSegments(segments), globstars: globstars, segments: segments}
	globalCompiledPatternCache.Add(cacheKey, compiled)

	return compiled, nil
}

func compileGlobPattern(pattern string, cfg config) (*compiledPath, error) {
	cacheKey := makeCompileCacheKey(compileCacheGlob, pattern, cfg)
	if compiled, ok := globalCompiledPatternCache.Get(cacheKey); ok {
		return compiled, nil
	}

	parts := splitGlobPath(pattern)
	segments := make([]compiledSegment, 0, len(parts.segments))
	globstars := 0
	prefixLen := 0
	walkBase := compilePathBase(parts.volume, parts.absolute)

	for _, raw := range parts.segments {
		segment, err := compileSegment(raw, cfg)
		if err != nil {
			return nil, err
		}

		if segment.globstar {
			globstars++
		}

		if prefixLen == len(segments) && !segment.meta && !segment.globstar {
			prefixLen++
			walkBase = joinGlobPath(walkBase, segment.literal)
		}

		segments = append(segments, segment)
	}

	compiled := &compiledPath{
		volume:           parts.volume,
		absolute:         parts.absolute,
		dirOnly:          parts.dirOnly,
		hasMeta:          hasMetaSegments(segments),
		globstars:        globstars,
		prefixLen:        prefixLen,
		segments:         segments,
		literalPathValue: buildLiteralPath(parts.volume, parts.absolute, parts.dirOnly, segments),
		walkBaseValue:    walkBase,
	}
	globalCompiledPatternCache.Add(cacheKey, compiled)

	return compiled, nil
}

func compileSegment(raw string, cfg config) (compiledSegment, error) {
	if cfg.globstar && raw == "**" {
		return compiledSegment{globstar: true, meta: true}, nil
	}

	seq, err := parseSeq(raw, cfg)
	if err != nil {
		return compiledSegment{}, err
	}

	literal, isLiteral := seq.literalVal()

	return compiledSegment{seq: seq, literal: literal, meta: !isLiteral, leadingDot: seq.matchesLeadingDotExplicitly(cfg)}, nil
}

func (compiled *compiledPath) matchName(name string, cfg config) bool {
	if matched, ok := compiled.matchSingleName(name, cfg); ok {
		return matched
	}

	if matched, ok := compiled.matchPathString(name, cfg); ok {
		return matched
	}

	parts := splitMatchPath(name)
	if !sameVolume(compiled.volume, parts.volume) || compiled.absolute != parts.absolute {
		return false
	}

	if compiled.globstars == 0 {
		return matchPathSegmentsNoGlobstar(compiled.segments, parts.segments, cfg)
	}

	return matchPathSegments(compiled.segments, parts.segments, cfg)
}

func (compiled *compiledPath) matchPathString(name string, cfg config) (bool, bool) {
	volume := filepath.VolumeName(name)
	rest := name[len(volume):]

	absolute := len(rest) > 0 && os.IsPathSeparator(rest[0])
	if absolute {
		rest = rest[1:]
	}

	if !sameVolume(compiled.volume, volume) || compiled.absolute != absolute {
		return false, true
	}

	if len(rest) > 0 && rest[len(rest)-1] == Separator {
		return false, false
	}

	var matched, ok bool
	if compiled.globstars == 0 {
		matched, ok = matchPathSegmentsStringNoGlobstar(compiled.segments, rest, cfg)
	} else {
		matched, ok = matchPathSegmentsString(compiled.segments, rest, cfg)
	}
	if !ok {
		return false, false
	}

	return matched, true
}

func (compiled *compiledPath) matchSingleName(name string, cfg config) (bool, bool) {
	if compiled.absolute || compiled.volume != "" || len(compiled.segments) != 1 {
		return false, false
	}

	if filepath.VolumeName(name) != "" || strings.ContainsRune(name, Separator) {
		return false, false
	}

	segment := compiled.segments[0]
	if segment.globstar {
		return false, true
	}

	if isSpecialDotSegment(name) && cfg.skipSpecialDots() {
		if segment.meta || !matchLiteralString(segment.literal, name, cfg) {
			return false, true
		}
	}

	if hasLeadingDot(name) && !isSpecialDotSegment(name) && cfg.usesBashPathnameRules() && !cfg.includeHiddenEntries() && !segment.canMatchLeadingDot() {
		return false, true
	}

	if segment.meta {
		return segment.seq.matchString(name, cfg), true
	}

	return matchLiteralString(segment.literal, name, cfg), true
}

func matchPathSegments(pattern []compiledSegment, name []string, cfg config) bool {
	var walk func(int, int) bool

	walk = func(pi, ni int) bool {
		if pi == len(pattern) {
			return ni == len(name)
		}

		if pattern[pi].globstar {
			if walk(pi+1, ni) {
				return true
			}

			for next := ni; next < len(name); next++ {
				if cfg.globSkipDots && isSpecialDotSegment(name[next]) {
					continue
				}

				if walk(pi, next+1) {
					return true
				}
			}

			return false
		}

		if ni >= len(name) {
			return false
		}

		if isSpecialDotSegment(name[ni]) && cfg.skipSpecialDots() {
			if pattern[pi].meta || pattern[pi].literal != name[ni] {
				return false
			}
		}

		if hasLeadingDot(name[ni]) && !isSpecialDotSegment(name[ni]) && cfg.usesBashPathnameRules() && !cfg.includeHiddenEntries() && !pattern[pi].canMatchLeadingDot() {
			return false
		}

		if pattern[pi].meta {
			if !pattern[pi].seq.matchString(name[ni], cfg) {
				return false
			}
		} else if !matchLiteralString(pattern[pi].literal, name[ni], cfg) {
			return false
		}

		return walk(pi+1, ni+1)
	}

	return walk(0, 0)
}

func matchPathSegmentsNoGlobstar(pattern []compiledSegment, name []string, cfg config) bool {
	if len(pattern) != len(name) {
		return false
	}

	skipDots := cfg.skipSpecialDots()
	usesBashRules := cfg.usesBashPathnameRules()
	includeHidden := cfg.includeHiddenEntries()

	for idx, segment := range name {
		compiled := pattern[idx]

		if isSpecialDotSegment(segment) && skipDots {
			if compiled.meta || compiled.literal != segment {
				return false
			}
		}

		if hasLeadingDot(segment) && !isSpecialDotSegment(segment) && usesBashRules && !includeHidden && !compiled.canMatchLeadingDot() {
			return false
		}

		if compiled.meta {
			if !compiled.seq.matchString(segment, cfg) {
				return false
			}
		} else if !matchLiteralString(compiled.literal, segment, cfg) {
			return false
		}
	}

	return true
}

func matchPathSegmentsString(pattern []compiledSegment, name string, cfg config) (bool, bool) {
	var walk func(int, int) (bool, bool)

	walk = func(pi, pos int) (bool, bool) {
		if pi == len(pattern) {
			return pos == len(name), true
		}

		if pattern[pi].globstar {
			if matched, ok := walk(pi+1, pos); matched || !ok {
				return matched, ok
			}

			nextPos := pos
			for {
				segment, after, ok := nextPathSegment(name, nextPos)
				if !ok {
					return false, nextPos == len(name)
				}

				if cfg.globSkipDots && isSpecialDotSegment(segment) {
					nextPos = after
					continue
				}

				if matched, ok := walk(pi, after); matched || !ok {
					return matched, ok
				}

				nextPos = after
			}
		}

		segment, nextPos, ok := nextPathSegment(name, pos)
		if !ok {
			return false, pos == len(name)
		}

		if isSpecialDotSegment(segment) && cfg.skipSpecialDots() {
			if pattern[pi].meta || pattern[pi].literal != segment {
				return false, true
			}
		}

		if hasLeadingDot(segment) && !isSpecialDotSegment(segment) && cfg.usesBashPathnameRules() && !cfg.includeHiddenEntries() && !pattern[pi].canMatchLeadingDot() {
			return false, true
		}

		if pattern[pi].meta {
			if !pattern[pi].seq.matchString(segment, cfg) {
				return false, true
			}
		} else if !matchLiteralString(pattern[pi].literal, segment, cfg) {
			return false, true
		}

		return walk(pi+1, nextPos)
	}

	return walk(0, 0)
}

func matchPathSegmentsStringNoGlobstar(pattern []compiledSegment, name string, cfg config) (bool, bool) {
	skipDots := cfg.skipSpecialDots()
	usesBashRules := cfg.usesBashPathnameRules()
	includeHidden := cfg.includeHiddenEntries()
	pos := 0

	for _, compiled := range pattern {
		if pos >= len(name) {
			return false, true
		}

		if name[pos] == Separator {
			return false, false
		}

		next := strings.IndexByte(name[pos:], Separator)
		segment := name[pos:]
		if next >= 0 {
			end := pos + next
			segment = name[pos:end]
			pos = end + 1
		} else {
			pos = len(name)
		}

		if isSpecialDotSegment(segment) && skipDots {
			if compiled.meta || compiled.literal != segment {
				return false, true
			}
		}

		if hasLeadingDot(segment) && !isSpecialDotSegment(segment) && usesBashRules && !includeHidden && !compiled.canMatchLeadingDot() {
			return false, true
		}

		if compiled.meta {
			if !compiled.seq.matchString(segment, cfg) {
				return false, true
			}
		} else if !matchLiteralString(compiled.literal, segment, cfg) {
			return false, true
		}
	}

	return pos == len(name), true
}

func parseSeq(input string, cfg config) (*sequence, error) {
	p := parser{input: input, cfg: cfg}

	seq, err := p.parseUntil(0)
	if err != nil {
		return nil, err
	}

	if p.pos != len(p.input) {
		return nil, ErrBadPattern
	}

	seq.initFastMatcher()

	return seq, nil
}

func (segment compiledSegment) canMatchLeadingDot() bool {
	if segment.globstar {
		return false
	}

	if !segment.meta {
		return strings.HasPrefix(segment.literal, ".")
	}

	return segment.leadingDot
}

func (class *charClass) matchRune(r rune, cfg config) bool {
	matched := false
	for _, current := range class.ranges {
		if runeInRange(r, current, cfg) {
			matched = true
			break
		}
	}

	if class.negated {
		return !matched
	}

	return matched
}

func appendUniqInt(values []int, value int) []int {
	if slices.Contains(values, value) {
		return values
	}

	return append(values, value)
}

func runesEq(left, right rune, cfg config) bool {
	if cfg.noCase {
		return foldRune(left, cfg) == foldRune(right, cfg)
	}

	return left == right
}

func matchLiteralString(literal, input string, cfg config) bool {
	if !cfg.noCase {
		return literal == input
	}

	if len(literal) == len(input) && isASCIIString(literal) && isASCIIString(input) {
		for idx := 0; idx < len(literal); idx++ {
			if foldASCIIRune(rune(literal[idx]), cfg) != foldASCIIRune(rune(input[idx]), cfg) {
				return false
			}
		}

		return true
	}

	for len(literal) != 0 && len(input) != 0 {
		left, leftSize := utf8.DecodeRuneInString(literal)
		right, rightSize := utf8.DecodeRuneInString(input)

		if !runesEq(left, right, cfg) {
			return false
		}

		literal = literal[leftSize:]
		input = input[rightSize:]
	}

	return len(literal) == 0 && len(input) == 0
}

func runeInRange(candidate rune, current charRange, cfg config) bool {
	if cfg.asciiRanges && candidate <= unicode.MaxASCII && current.lo <= unicode.MaxASCII && current.hi <= unicode.MaxASCII {
		left := foldASCIIRune(current.lo, cfg)
		right := foldASCIIRune(current.hi, cfg)
		value := foldASCIIRune(candidate, cfg)

		if left > right {
			left, right = right, left
		}

		return left <= value && value <= right
	}

	left := foldRune(current.lo, cfg)
	right := foldRune(current.hi, cfg)
	value := foldRune(candidate, cfg)

	if left > right {
		left, right = right, left
	}

	return left <= value && value <= right
}

func foldRune(r rune, cfg config) rune {
	if !cfg.noCase {
		return r
	}

	return unicode.ToLower(r)
}

func foldASCIIRune(r rune, cfg config) rune {
	if !cfg.noCase {
		return r
	}

	if 'A' <= r && r <= 'Z' {
		return r + ('a' - 'A')
	}

	return r
}

type pathParts struct {
	volume   string
	absolute bool
	dirOnly  bool
	segments []string
}

func nextPathSegment(path string, pos int) (string, int, bool) {
	if pos >= len(path) {
		return "", pos, false
	}

	if path[pos] == Separator {
		return "", pos, false
	}

	if next := strings.IndexByte(path[pos:], Separator); next >= 0 {
		end := pos + next

		return path[pos:end], end + 1, true
	}

	return path[pos:], len(path), true
}

func splitMatchPath(path string) pathParts {
	cacheKey := pathPartsCacheKey{mode: pathPartsCacheMatch, path: path}
	if parts, ok := globalPathPartsCache.Get(cacheKey); ok {
		return parts
	}

	volume := filepath.VolumeName(path)
	rest := path[len(volume):]

	absolute := len(rest) > 0 && os.IsPathSeparator(rest[0])
	if absolute {
		rest = rest[1:]
	}

	segments := strings.Split(rest, string(Separator))
	if rest == "" {
		segments = nil
	}

	parts := pathParts{volume: volume, absolute: absolute, segments: segments}
	globalPathPartsCache.Add(cacheKey, parts)

	return parts
}

func splitGlobPath(path string) pathParts {
	cacheKey := pathPartsCacheKey{mode: pathPartsCacheGlob, path: path}
	if parts, ok := globalPathPartsCache.Get(cacheKey); ok {
		return parts
	}

	parts := splitMatchPath(path)
	if len(parts.segments) > 0 && parts.segments[len(parts.segments)-1] == "" {
		parts.dirOnly = true
		parts.segments = parts.segments[:len(parts.segments)-1]
	}

	globalPathPartsCache.Add(cacheKey, parts)

	return parts
}

func sameVolume(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}

	return left == right
}

func compilePathBase(volume string, absolute bool) string {
	if absolute {
		return volume + string(Separator)
	}

	if volume != "" {
		return volume
	}

	return "."
}

func buildLiteralPath(volume string, absolute, dirOnly bool, segments []compiledSegment) string {
	path := compilePathBase(volume, absolute)
	for _, segment := range segments {
		path = joinGlobPath(path, segment.literal)
	}

	if dirOnly && !strings.HasSuffix(path, string(Separator)) {
		path += string(Separator)
	}

	return path
}

func isSpecialDotSegment(name string) bool {
	return name == "." || name == ".."
}

func hasLeadingDot(name string) bool {
	return strings.HasPrefix(name, ".")
}

func (compiled *compiledPath) literalPath() string {
	if compiled.literalPathValue == "" {
		return buildLiteralPath(compiled.volume, compiled.absolute, compiled.dirOnly, compiled.segments)
	}

	return compiled.literalPathValue
}
