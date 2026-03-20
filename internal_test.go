package richglob

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	internalcache "go.dw1.io/richglob/internal/cache"
)

func TestInternalConfigHelpers(t *testing.T) {
	if got := buildConfig(nil); got.sort != SortLexicographic {
		t.Fatalf("default sort = %v, want %v", got.sort, SortLexicographic)
	}
	if !defaultConfig().includeHiddenEntries() {
		t.Fatal("default config should include hidden entries when bash pathname rules are disabled")
	}

	customApplied := false
	cfg := buildConfig([]Option{
		nil,
		WithExtGlob(),
		WithGlobStar(),
		WithDotGlob(),
		WithNoCaseGlob(),
		WithNullGlob(),
		WithFailGlob(),
		WithGlobASCIIRanges(),
		WithGlobSkipDots(false),
		WithGlobIgnore("*.tmp"),
		WithSort(SortNone),
		option(func(cfg *config) {
			customApplied = true
			cfg.globIgnore = append(cfg.globIgnore, "*.bak")
		}),
	})

	if !customApplied {
		t.Fatal("custom option was not applied")
	}
	if !cfg.extglob || !cfg.globstar || !cfg.dotglob || !cfg.noCase || !cfg.nullglob || !cfg.failglob || !cfg.asciiRanges {
		t.Fatalf("buildConfig did not apply builtin options: %#v", cfg)
	}
	if cfg.globSkipDots || !cfg.globSkipDotsSet {
		t.Fatalf("glob skip dots flags = (%v, %v), want (false, true)", cfg.globSkipDots, cfg.globSkipDotsSet)
	}
	if cfg.sort != SortNone {
		t.Fatalf("sort = %v, want %v", cfg.sort, SortNone)
	}
	if !slices.Equal(cfg.globIgnore, []string{"*.tmp", "*.bak"}) {
		t.Fatalf("globIgnore = %#v", cfg.globIgnore)
	}
	if !cfg.usesBashPathnameRules() {
		t.Fatal("usesBashPathnameRules() = false, want true")
	}
	if cfg.skipSpecialDots() {
		t.Fatal("skipSpecialDots() = true, want false when explicitly disabled")
	}
	if !cfg.includeHiddenEntries() {
		t.Fatal("includeHiddenEntries() = false, want true with dotglob/globignore")
	}

	basic := buildConfig([]Option{WithNoCaseGlob()})
	if !basic.usesBashPathnameRules() || !basic.skipSpecialDots() {
		t.Fatalf("basic config helpers returned unexpected values: %#v", basic)
	}
	if basic.includeHiddenEntries() {
		t.Fatal("includeHiddenEntries() = true, want false for bash rules without dotglob")
	}
}

func TestInternalCompiledPatternCache(t *testing.T) {
	cache := internalcache.New[compileCacheKey, *compiledPath](2)
	key1 := compileCacheKey{mode: compileCacheMatch, pattern: "a"}
	key2 := compileCacheKey{mode: compileCacheMatch, pattern: "b"}
	key3 := compileCacheKey{mode: compileCacheMatch, pattern: "c"}
	compiled1 := &compiledPath{segments: []compiledSegment{{literal: "a"}}}
	compiled2 := &compiledPath{segments: []compiledSegment{{literal: "b"}}}
	compiled3 := &compiledPath{segments: []compiledSegment{{literal: "c"}}}

	if got, ok := cache.Get(key1); ok || got != nil {
		t.Fatalf("unexpected cache hit for empty cache: %#v %v", got, ok)
	}

	cache.Add(key1, compiled1)
	cache.Add(key2, compiled2)

	if got, ok := cache.Get(key1); !ok || got != compiled1 {
		t.Fatalf("cache.get(key1) = %#v, %v", got, ok)
	}
	if got, ok := cache.Get(key1); !ok || got != compiled1 {
		t.Fatalf("hot cache get(key1) = %#v, %v", got, ok)
	}

	cache.Add(key3, compiled3)
	if got, ok := cache.Get(key1); ok || got != nil {
		t.Fatalf("evicted key1 still present: %#v %v", got, ok)
	}
	if got, ok := cache.Get(key2); !ok || got != compiled2 {
		t.Fatalf("cache.get(key2) = %#v, %v", got, ok)
	}
	if got, ok := cache.Get(key3); !ok || got != compiled3 {
		t.Fatalf("cache.get(key3) = %#v, %v", got, ok)
	}

	cfg := config{extglob: true, globstar: true, noCase: true, asciiRanges: true}
	cacheKey := makeCompileCacheKey(compileCacheGlob, "**/*.go", cfg)
	if cacheKey.mode != compileCacheGlob || cacheKey.pattern != "**/*.go" || !cacheKey.extglob || !cacheKey.globstar || !cacheKey.noCase || !cacheKey.asciiRanges {
		t.Fatalf("makeCompileCacheKey returned %#v", cacheKey)
	}
}

func TestInternalPathPartsCacheAndSplitters(t *testing.T) {
	cache := internalcache.New[pathPartsCacheKey, pathParts](2)
	key1 := pathPartsCacheKey{mode: pathPartsCacheMatch, path: "a"}
	key2 := pathPartsCacheKey{mode: pathPartsCacheMatch, path: "b"}
	key3 := pathPartsCacheKey{mode: pathPartsCacheMatch, path: "c"}
	parts1 := pathParts{segments: []string{"a"}}
	parts2 := pathParts{segments: []string{"b"}}
	parts3 := pathParts{segments: []string{"c"}}

	cache.Add(key1, parts1)
	cache.Add(key2, parts2)
	if got, ok := cache.Get(key1); !ok || !slices.Equal(got.segments, parts1.segments) {
		t.Fatalf("cache.get(key1) = %#v, %v", got, ok)
	}
	if got, ok := cache.Get(key1); !ok || !slices.Equal(got.segments, parts1.segments) {
		t.Fatalf("hot cache get(key1) = %#v, %v", got, ok)
	}
	cache.Add(key3, parts3)
	if got, ok := cache.Get(key1); ok || got.segments != nil {
		t.Fatalf("evicted key1 still present: %#v %v", got, ok)
	}

	matchPath := filepath.Join("alpha", "beta")
	matchParts := splitMatchPath(matchPath)
	if matchParts.absolute {
		t.Fatalf("splitMatchPath(%q) marked path absolute", matchPath)
	}
	if !slices.Equal(matchParts.segments, []string{"alpha", "beta"}) {
		t.Fatalf("splitMatchPath(%q) = %#v", matchPath, matchParts)
	}

	globPath := matchPath + string(Separator)
	globParts := splitGlobPath(globPath)
	if !globParts.dirOnly || !slices.Equal(globParts.segments, []string{"alpha", "beta"}) {
		t.Fatalf("splitGlobPath(%q) = %#v", globPath, globParts)
	}

	segment, next, ok := nextPathSegment("alpha"+string(Separator)+"beta", 0)
	if !ok || segment != "alpha" || next != len("alpha")+1 {
		t.Fatalf("nextPathSegment returned (%q, %d, %v)", segment, next, ok)
	}
	if segment, next, ok := nextPathSegment(string(Separator)+"alpha", 0); ok || segment != "" || next != 0 {
		t.Fatalf("leading separator returned (%q, %d, %v)", segment, next, ok)
	}

	compiled := &compiledPath{
		absolute:  true,
		dirOnly:   true,
		segments:  []compiledSegment{{literal: "alpha"}, {literal: "beta"}},
		prefixLen: 1,
	}
	expected := string(Separator) + "alpha" + string(Separator) + "beta" + string(Separator)
	if got := compiled.literalPath(); got != expected {
		t.Fatalf("literalPath() = %q, want %q", got, expected)
	}
	base, idx := compiled.globWalkBase()
	if base != string(Separator)+"alpha" || idx != 1 {
		t.Fatalf("globWalkBase() = (%q, %d)", base, idx)
	}

	if !hasMetaSegments([]compiledSegment{{meta: true}}) || !hasMetaSegments([]compiledSegment{{globstar: true}}) {
		t.Fatal("hasMetaSegments() missed meta segment")
	}
	if hasMetaSegments([]compiledSegment{{literal: "plain"}}) {
		t.Fatal("hasMetaSegments() reported meta for plain segment")
	}

	if got := joinGlobPath("", "alpha"); got != "alpha" {
		t.Fatalf("joinGlobPath empty base = %q", got)
	}
	if got := joinGlobPath(".", "alpha"); got != "alpha" {
		t.Fatalf("joinGlobPath dot base = %q", got)
	}
	if got := joinGlobPath(string(Separator), "alpha"); got != string(Separator)+"alpha" {
		t.Fatalf("joinGlobPath root base = %q", got)
	}
	if got := joinGlobPath("alpha"+string(Separator), "beta"); got != "alpha"+string(Separator)+"beta" {
		t.Fatalf("joinGlobPath slash base = %q", got)
	}

	sorted := uniqStrings([]string{"a", "a", "b", "b"}, true)
	if !slices.Equal(sorted, []string{"a", "b"}) {
		t.Fatalf("uniqStrings(sorted) = %#v", sorted)
	}
	unsorted := uniqStrings([]string{"b", "a", "b", "c", "a"}, false)
	if !slices.Equal(unsorted, []string{"b", "a", "c"}) {
		t.Fatalf("uniqStrings(unsorted) = %#v", unsorted)
	}

	limitPattern := strings.Repeat(string(Separator), 10001)
	if !exceedsPathSepLimit(limitPattern) {
		t.Fatal("exceedsPathSepLimit() = false, want true")
	}
	if exceedsPathSepLimit("alpha" + string(Separator) + "beta") {
		t.Fatal("exceedsPathSepLimit() = true for short path")
	}

	if !isSpecialDotSegment(".") || !isSpecialDotSegment("..") || isSpecialDotSegment("...") {
		t.Fatal("isSpecialDotSegment returned unexpected results")
	}
	if !hasLeadingDot(".env") || hasLeadingDot("env") {
		t.Fatal("hasLeadingDot returned unexpected results")
	}

	if runtime.GOOS == "windows" {
		if !sameVolume("C:", "c:") {
			t.Fatal("sameVolume should be case-insensitive on windows")
		}
	} else {
		if !sameVolume("", "") || sameVolume("a", "A") {
			t.Fatal("sameVolume returned unexpected result on non-windows")
		}
	}
}

func TestInternalParserAndMatchers(t *testing.T) {
	starSeq, err := parseSeq("*", config{})
	if err != nil {
		t.Fatalf("parseSeq(*) error: %v", err)
	}
	if starSeq.fast.kind != sequenceFastStar || !starSeq.matchString("alpha", config{}) {
		t.Fatalf("star sequence fast path not initialized: %#v", starSeq.fast)
	}
	if starSeq.matchesLeadingDotExplicitly(config{}) {
		t.Fatal("star sequence should not match leading dots explicitly")
	}

	fastSeq, err := parseSeq("ab*cd", config{})
	if err != nil {
		t.Fatalf("parseSeq(ab*cd) error: %v", err)
	}
	if fastSeq.fast.kind != sequenceFastLiteralStarLiteral {
		t.Fatalf("fast sequence kind = %v", fastSeq.fast.kind)
	}
	if matched, ok := fastSeq.matchFastString("abZZcd"); !ok || !matched {
		t.Fatalf("matchFastString success = (%v, %v)", matched, ok)
	}
	if matched, ok := fastSeq.matchFastString("ab"); !ok || matched {
		t.Fatalf("matchFastString short input = (%v, %v)", matched, ok)
	}

	unicodeSeq, err := parseSeq("a☺b", config{})
	if err != nil {
		t.Fatalf("parseSeq unicode error: %v", err)
	}
	if unicodeSeq.asciiOnly {
		t.Fatal("unicode sequence marked asciiOnly")
	}
	if unicodeSeq.matchesLeadingDotExplicitly(config{}) {
		t.Fatal("unicode sequence should not match leading dots explicitly")
	}
	if !unicodeSeq.matchString("a☺b", config{}) || unicodeSeq.matchString("a☹b", config{}) {
		t.Fatal("unicode sequence matching returned unexpected result")
	}

	dotSeq, err := parseSeq(".env", config{})
	if err != nil {
		t.Fatalf("parseSeq(.env) error: %v", err)
	}
	if !dotSeq.matchesLeadingDotExplicitly(config{}) {
		t.Fatal("dot-prefixed literal sequence should match leading dots explicitly")
	}

	classParser := parser{input: "[^a-c]"}
	class, err := classParser.parseClass()
	if err != nil {
		t.Fatalf("parseClass error: %v", err)
	}
	if !class.negated || !class.isASCIIOnly() {
		t.Fatalf("parseClass returned %#v", class)
	}
	if !class.matchRune('z', config{}) || class.matchRune('b', config{}) {
		t.Fatal("charClass matching returned unexpected result")
	}

	asciiCfg := config{noCase: true, asciiRanges: true}
	if !runeInRange('m', charRange{lo: 'Z', hi: 'A'}, asciiCfg) {
		t.Fatal("runeInRange should fold and normalize ASCII ranges")
	}
	if foldRune('A', asciiCfg) != 'a' || foldASCIIRune('Z', asciiCfg) != 'z' {
		t.Fatal("fold helpers returned unexpected results")
	}
	if !runesEq('A', 'a', asciiCfg) || runesEq('A', 'b', asciiCfg) {
		t.Fatal("runesEq returned unexpected result")
	}
	if !matchLiteralString("ALPHA", "alpha", asciiCfg) || matchLiteralString("ALPHA", "beta", asciiCfg) {
		t.Fatal("matchLiteralString returned unexpected result")
	}
	if !matchLiteralASCII([]rune("Go"), "gopher", 0, asciiCfg) || matchLiteralASCII([]rune("Go"), "nope", 0, asciiCfg) {
		t.Fatal("matchLiteralASCII returned unexpected result")
	}
	if !matchLiteralRunes([]rune("Go"), []rune("gopher"), 0, asciiCfg) || matchLiteralRunes([]rune("Go"), []rune("nope"), 0, asciiCfg) {
		t.Fatal("matchLiteralRunes returned unexpected result")
	}
	if !isASCIIString("alpha") || isASCIIString("☺") {
		t.Fatal("isASCIIString returned unexpected result")
	}

	if got := appendUniqInt([]int{1, 2}, 2); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("appendUniqInt existing value = %#v", got)
	}
	if got := appendUniqInt([]int{1, 2}, 3); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("appendUniqInt appended value = %#v", got)
	}

	segment, err := compileSegment("**", config{globstar: true})
	if err != nil || !segment.globstar || !segment.meta {
		t.Fatalf("compileSegment(**) = %#v, %v", segment, err)
	}
	if segment.canMatchLeadingDot() {
		t.Fatal("globstar segment should not match leading dots explicitly")
	}

	leadingDotSegment, err := compileSegment("@(.env|config)", config{extglob: true})
	if err != nil {
		t.Fatalf("compileSegment extglob error: %v", err)
	}
	if !leadingDotSegment.canMatchLeadingDot() {
		t.Fatal("extglob segment should match leading dots explicitly")
	}

	if _, err := parseSeq("@(foo|", config{extglob: true}); err != ErrBadPattern {
		t.Fatalf("parseSeq bad extglob error = %v", err)
	}
	badLiteralParser := parser{input: string([]byte{0xff})}
	if _, err := badLiteralParser.parseLiteralRune(); err != ErrBadPattern {
		t.Fatalf("parseLiteralRune invalid utf8 error = %v", err)
	}
	badClassParser := parser{input: "-"}
	if _, err := badClassParser.parseClassRune(); err != ErrBadPattern {
		t.Fatalf("parseClassRune invalid range error = %v", err)
	}

	pattern := []compiledSegment{{globstar: true, meta: true}, {literal: "file.go"}}
	if !matchPathSegments(pattern, []string{".", "dir", "file.go"}, config{globSkipDots: true, globSkipDotsSet: true}) {
		t.Fatal("matchPathSegments did not skip dot segments as expected")
	}
	pattern = []compiledSegment{{literal: "alpha"}, {literal: "beta"}, {literal: "file.go"}}
	if matched, ok := matchPathSegmentsString(pattern, filepath.Join("alpha", "beta", "file.go"), config{}); !ok || !matched {
		t.Fatalf("matchPathSegmentsString(no globstar) = (%v, %v)", matched, ok)
	}
	if matched, ok := matchPathSegmentsString(pattern, "alpha"+string(Separator)+string(Separator)+"file.go", config{}); ok || matched {
		t.Fatalf("matchPathSegmentsString(repeated separator) = (%v, %v)", matched, ok)
	}
	if matched, ok := matchPathSegmentsString([]compiledSegment{{literal: "alpha"}}, string(Separator)+"alpha", config{}); ok || matched {
		t.Fatalf("matchPathSegmentsString leading separator = (%v, %v)", matched, ok)
	}

	negate := &extGroup{op: '!', alts: []*sequence{{nodes: []node{{kind: nodeLiteral, lit: []rune("ab")}}, asciiOnly: true}}}
	if !slices.Equal(negate.match([]rune("abcd"), 0, config{}), []int{0, 1, 3, 4}) {
		t.Fatalf("negate extglob returned unexpected ends: %#v", negate.match([]rune("abcd"), 0, config{}))
	}
	if !negate.matchesAnyWhole([]rune("ab"), config{}) || negate.matchesAnyWhole([]rune("xy"), config{}) {
		t.Fatal("matchesAnyWhole returned unexpected result")
	}

	plus := &extGroup{op: '+', alts: []*sequence{{nodes: []node{{kind: nodeLiteral, lit: []rune("ab")}}, asciiOnly: true}}}
	if !slices.Equal(plus.match([]rune("abab"), 0, config{}), []int{2, 4}) {
		t.Fatalf("plus extglob returned unexpected ends: %#v", plus.match([]rune("abab"), 0, config{}))
	}
	if got := (&extGroup{op: '#'}).match([]rune("ab"), 0, config{}); got != nil {
		t.Fatalf("unknown extglob operator returned %#v", got)
	}

	if got := matchNodes([]node{{kind: nodeLiteral, lit: []rune("ab")}, {kind: nodeAny}}, 0, []rune("abc"), 0, config{}); !slices.Equal(got, []int{3}) {
		t.Fatalf("matchNodes returned %#v", got)
	}
	if got := (node{kind: nodeStar}).match([]rune("abc"), 1, config{}); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("nodeStar.match returned %#v", got)
	}
	if !(node{kind: nodeClass, class: &charClass{ranges: []charRange{{lo: '.', hi: '.'}}}}).matchesLeadingDotExplicitly(config{}) {
		t.Fatal("nodeClass should report explicit leading-dot support")
	}
	if !(node{kind: nodeExt, ext: &extGroup{alts: []*sequence{{nodes: []node{{kind: nodeLiteral, lit: []rune(".env")}}, asciiOnly: true}}}}).matchesLeadingDotExplicitly(config{}) {
		t.Fatal("nodeExt should report explicit leading-dot support")
	}
}

func TestInternalFilesystemHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	alphaDir := filepath.Join(tmpDir, "alpha")
	betaDir := filepath.Join(tmpDir, "beta")
	alphaNestedDir := filepath.Join(alphaDir, "nested")
	matchFile := filepath.Join(alphaDir, "match.txt")
	if err := os.MkdirAll(alphaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(alphaDir): %v", err)
	}
	if err := os.MkdirAll(alphaNestedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(alphaNestedDir): %v", err)
	}
	if err := os.MkdirAll(betaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(betaDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "z.txt"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile(z.txt): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile(a.txt): %v", err)
	}
	if err := os.WriteFile(matchFile, nil, 0o644); err != nil {
		t.Fatalf("WriteFile(match.txt): %v", err)
	}

	entries := readDirEntries(tmpDir, SortLexicographic)
	if len(entries) < 4 {
		t.Fatalf("readDirEntries returned %d entries, want at least 4", len(entries))
	}
	names := []string{entries[0].Name(), entries[1].Name(), entries[2].Name(), entries[3].Name()}
	if !slices.Equal(names, []string{"a.txt", "alpha", "beta", "z.txt"}) {
		t.Fatalf("sorted readDirEntries names = %#v", names)
	}
	if readDirEntries(filepath.Join(tmpDir, "missing"), SortLexicographic) != nil {
		t.Fatal("readDirEntries on missing dir should return nil")
	}

	if !isDir(alphaDir) || isDir(filepath.Join(tmpDir, "a.txt")) {
		t.Fatal("isDir returned unexpected result")
	}

	compiledName, err := compileMatchPattern(filepath.Join("alpha", "match.txt"), config{})
	if err != nil {
		t.Fatalf("compileMatchPattern error: %v", err)
	}
	if !compiledName.matchName(filepath.Join("alpha", "match.txt"), config{}) {
		t.Fatal("matchName should match the exact relative path")
	}
	if compiledName.matchName(filepath.Join("alpha", "match.txt")+string(Separator), config{}) {
		t.Fatal("matchName should reject trailing separators for file paths")
	}
	if compiledName.matchName(filepath.Join("alpha", "other.txt"), config{}) {
		t.Fatal("matchName should reject non-matching paths")
	}

	compiled, _, err := compileIgnorePatterns(config{globIgnore: []string{"*.tmp"}, sort: SortNone})
	if err != nil {
		t.Fatalf("compileIgnorePatterns error: %v", err)
	}
	if len(compiled) != 1 {
		t.Fatalf("compileIgnorePatterns returned %d patterns", len(compiled))
	}
	if _, _, err := compileIgnorePatterns(config{globIgnore: []string{"["}}); err != ErrBadPattern {
		t.Fatalf("compileIgnorePatterns bad pattern error = %v", err)
	}

	ignoreCfg := ignoreMatchConfig(config{extglob: true, globstar: true, dotglob: true, globSkipDots: true, globSkipDotsSet: true, noCase: true, nullglob: true, failglob: true, asciiRanges: true, sort: SortNone})
	if !ignoreCfg.extglob || !ignoreCfg.globstar || !ignoreCfg.dotglob || !ignoreCfg.globSkipDots || !ignoreCfg.noCase || !ignoreCfg.nullglob || !ignoreCfg.failglob || !ignoreCfg.asciiRanges || ignoreCfg.sort != SortNone {
		t.Fatalf("ignoreMatchConfig returned %#v", ignoreCfg)
	}

	yielded := make([]string, 0)
	var yieldedErr error
	emitter := globSeqEmitter{
		yield: func(path string, err error) bool {
			if err != nil {
				yieldedErr = err
				return false
			}
			yielded = append(yielded, path)
			return true
		},
		ignorePatterns: compiled,
		ignoreCfg:      config{},
		seen:           make(map[string]struct{}),
	}
	if !emitter.emit("keep.go", false, nil) {
		t.Fatal("emit keep.go returned false")
	}
	if !emitter.emit("skip.tmp", false, nil) {
		t.Fatal("emit skip.tmp returned false")
	}
	if !emitter.emit("keep.go", false, nil) {
		t.Fatal("duplicate emit returned false")
	}
	if !slices.Equal(yielded, []string{"keep.go"}) || yieldedErr != nil {
		t.Fatalf("emitter yielded (%#v, %v)", yielded, yieldedErr)
	}

	stopEmitter := globSeqEmitter{yield: func(string, error) bool { return false }}
	if stopEmitter.emit("stop.go", false, nil) {
		t.Fatal("emit should stop when yield returns false")
	}
	if stopEmitter.fail(ErrNoMatch) {
		t.Fatal("fail should return false after stop")
	}

	missEmitter := globSeqEmitter{yield: func(_ string, err error) bool {
		yieldedErr = err
		return true
	}}
	yieldedErr = nil
	missEmitter.finish(config{failglob: true})
	if yieldedErr != ErrNoMatch {
		t.Fatalf("finish failglob error = %v", yieldedErr)
	}

	filePath := filepath.Join(tmpDir, "plain.txt")
	if err := os.WriteFile(filePath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile(plain.txt): %v", err)
	}
	literalEmitter := globSeqEmitter{yield: func(path string, err error) bool {
		yielded = append(yielded[:0], path)
		yieldedErr = err
		return true
	}}
	if !literalEmitter.emitLiteral(filePath, false, nil) || len(yielded) != 1 || yielded[0] != filePath || yieldedErr != nil {
		t.Fatalf("emitLiteral(file) yielded (%#v, %v)", yielded, yieldedErr)
	}
	if !literalEmitter.emitLiteral(filePath, true, nil) {
		t.Fatal("emitLiteral(file, dirOnly) should return true")
	}
	if len(yielded) != 1 || yielded[0] != filePath {
		t.Fatalf("emitLiteral(file, dirOnly) changed yielded matches: %#v", yielded)
	}

	compiledWalk, err := compileGlobPattern(filepath.Join(tmpDir, "alpha", "*.txt"), config{})
	if err != nil {
		t.Fatalf("compileGlobPattern error: %v", err)
	}
	base, idx := compiledWalk.globWalkBase()
	walkMatches := make([]string, 0)
	walkEmitter := globSeqEmitter{yield: func(path string, err error) bool {
		if err != nil {
			t.Fatalf("walk emitter error: %v", err)
		}
		walkMatches = append(walkMatches, path)
		return true
	}}
	if !walkGlobSeq(base, resolveWalkKey(base), compiledWalk, idx, config{}, newWalkState(), &walkEmitter, nil) {
		t.Fatal("walkGlobSeq returned false for normal traversal")
	}
	if !slices.Equal(walkMatches, []string{matchFile}) {
		t.Fatalf("walkGlobSeq matches = %#v, want %#v", walkMatches, []string{matchFile})
	}

	dirOnlyPattern := &compiledPath{dirOnly: true}
	if !walkGlobSeq(filePath, resolveWalkKey(filePath), dirOnlyPattern, 0, config{}, newWalkState(), &walkEmitter, nil) {
		t.Fatal("walkGlobSeq dir-only file case should return true")
	}

	state := newWalkState()
	key, ok := state.enter(alphaDir)
	if !ok || key == "" {
		t.Fatalf("enter(alphaDir) = (%q, %v)", key, ok)
	}
	if _, ok := state.enter(alphaDir); ok {
		t.Fatal("duplicate enter(alphaDir) should fail")
	}
	state.leave(key)
	if _, ok := state.enter(alphaDir); !ok {
		t.Fatal("enter(alphaDir) should succeed after leave")
	}
	state.depth = 10000
	if _, ok := state.enter(betaDir); ok {
		t.Fatal("enter should fail after max walk depth")
	}

	if err := os.Symlink(alphaDir, filepath.Join(tmpDir, "alpha-link")); err == nil {
		link := filepath.Join(tmpDir, "alpha-link")
		entries = readDirEntries(tmpDir, SortLexicographic)
		var linkEntry os.DirEntry
		for _, entry := range entries {
			if entry.Name() == "alpha-link" {
				linkEntry = entry
				break
			}
		}
		if linkEntry == nil {
			t.Fatal("did not find symlink entry")
		}
		if !dirEntryIsDir(tmpDir, linkEntry) {
			t.Fatal("dirEntryIsDir should follow symlinked directories")
		}

		linkState := newWalkState()
		linkKey, ok := linkState.enter(link)
		if !ok {
			t.Fatal("enter(symlink) should succeed")
		}
		resolved, err := filepath.EvalSymlinks(link)
		if err == nil && linkKey != resolved {
			t.Fatalf("symlink key = %q, want %q", linkKey, resolved)
		}

		aliasDir := filepath.Join(link, "nested")
		aliasKey := resolveWalkKey(aliasDir)
		if aliasResolved, err := filepath.EvalSymlinks(aliasDir); err == nil && aliasKey != aliasResolved {
			t.Fatalf("aliased dir key = %q, want %q", aliasKey, aliasResolved)
		}

		linkState.leave(linkKey)
		realState := newWalkState()
		realKey, ok := realState.enter(alphaNestedDir)
		if !ok {
			t.Fatal("enter(real alphaNestedDir) should succeed")
		}
		if aliasEnterKey, ok := realState.enter(aliasDir); ok || aliasEnterKey != "" {
			t.Fatalf("enter(alias nested dir) = (%q, %v), want cycle rejection", aliasEnterKey, ok)
		}
		realState.leave(realKey)
	}

	segmentNoDot := compiledSegment{literal: "*", meta: true, seq: starSeqForTest(t)}
	segmentDot, err := compileSegment(".hidden", config{})
	if err != nil {
		t.Fatalf("compileSegment(.hidden) error: %v", err)
	}
	if !shouldSkipName(segmentNoDot, ".hidden", config{noCase: true}) {
		t.Fatal("shouldSkipName should skip hidden name under bash rules")
	}
	if shouldSkipName(segmentDot, ".hidden", config{noCase: true}) {
		t.Fatal("shouldSkipName should allow explicit leading-dot match")
	}
	if !shouldSkipName(segmentDot, ".", config{globSkipDots: true, globSkipDotsSet: true}) {
		t.Fatal("shouldSkipName should skip special dot segments")
	}
}

func starSeqForTest(t *testing.T) *sequence {
	t.Helper()

	seq, err := parseSeq("*", config{})
	if err != nil {
		t.Fatalf("parseSeq(*) error: %v", err)
	}

	return seq
}

func TestInternalGitIgnore(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	pkgDir := filepath.Join(srcDir, "pkg")
	generatedDir := filepath.Join(pkgDir, "generated")
	ignoredDir := filepath.Join(pkgDir, "ignored")

	for _, dir := range []string{srcDir, pkgDir, generatedDir, ignoredDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(srcDir, ".gitignore"), []byte("pkg/generated/*\n\\!literal\n"), 0o666); err != nil {
		t.Fatalf("WriteFile(src/.gitignore): %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, ".gitignore"), []byte("!generated/keep.go\nignored/\n"), 0o666); err != nil {
		t.Fatalf("WriteFile(pkg/.gitignore): %v", err)
	}

	loader := newGitIgnoreLoader(config{gitIgnore: true})
	if loader == nil {
		t.Fatal("newGitIgnoreLoader returned nil")
	}

	ctx := loader.contextForDir(pkgDir)
	if ctx == nil {
		t.Fatal("contextForDir returned nil")
	}
	if next := ctx.child(generatedDir); next == nil || next.parentContext() != ctx {
		t.Fatal("child context did not preserve parent linkage")
	}

	if !ctx.ignores(filepath.Join(pkgDir, "generated", "skip.go"), false) {
		t.Fatal("generated/skip.go should be ignored by parent .gitignore")
	}
	if ctx.ignores(filepath.Join(pkgDir, "generated", "keep.go"), false) {
		t.Fatal("generated/keep.go should be re-included by child .gitignore")
	}
	if !ctx.ignores(ignoredDir, true) {
		t.Fatal("ignored directory should be ignored")
	}

	rule, ok := parseGitIgnoreRule(`\!literal`, gitIgnoreMatchConfig(config{}))
	if !ok {
		t.Fatal("escaped leading bang should parse as a literal rule")
	}
	if !rule.matches("!literal", []string{"!literal"}, false, gitIgnoreMatchConfig(config{})) {
		t.Fatal("escaped leading bang rule did not match literal name")
	}

	commentRule, ok := parseGitIgnoreRule(`# comment`, gitIgnoreMatchConfig(config{}))
	if ok || commentRule.pattern != nil || commentRule.segment.seq != nil {
		t.Fatal("comment line should be ignored")
	}

	ancestors := gitIgnoreAncestors(filepath.Join("src", "pkg"))
	if !slices.Equal(ancestors, []string{".", "src", filepath.Join("src", "pkg")}) {
		t.Fatalf("gitIgnoreAncestors returned %#v", ancestors)
	}

	relPath, relParts, ok := gitIgnoreRelativePath(splitGitIgnorePath("."), filepath.Join("src", "pkg", "file.go"))
	if !ok || relPath != filepath.Join("src", "pkg", "file.go") || !slices.Equal(relParts, []string{"src", "pkg", "file.go"}) {
		t.Fatalf("gitIgnoreRelativePath returned (%q, %#v, %v)", relPath, relParts, ok)
	}
}
