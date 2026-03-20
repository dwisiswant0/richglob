package richglob

import (
	"io/fs"
	"iter"
	"os"
	"runtime"
	"slices"
	"strings"
)

// Glob returns the filesystem paths that match pattern.
//
// When nothing matches, Glob returns nil, nil by default. Use [WithNullGlob] to
// return an empty slice instead, or [WithFailGlob] to return [ErrNoMatch].
func Glob(pattern string, opts ...Option) ([]string, error) {
	cfg := buildConfig(opts)
	matches := make([]string, 0)

	for match, err := range GlobSeq2(pattern, opts...) {
		if err != nil {
			return nil, err
		}

		matches = append(matches, match)
	}

	if cfg.sort == SortLexicographic {
		slices.Sort(matches)
	}

	if len(matches) > 1 {
		compiled, err := compileGlobPattern(pattern, cfg)
		if err != nil {
			return nil, err
		}

		if compiled.globstars > 1 {
			matches = uniqStrings(matches, cfg.sort == SortLexicographic)
		}
	}

	return finalizeMatches(matches, cfg)
}

// GlobSeq2 walks the filesystem lazily and yields matching paths.
//
// The iterator yields `(path, nil)` pairs for matches. If pattern compilation,
// ignore-pattern compilation, or failglob processing encounters an error, the
// iterator yields a final `("", err)` pair and then stops. Matches are yielded
// in traversal order; unlike [Glob], GlobSeq2 does not collect and globally
// sort results before emission.
func GlobSeq2(pattern string, opts ...Option) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		if exceedsPathSepLimit(pattern) {
			yield("", ErrBadPattern)

			return
		}

		cfg := buildConfig(opts)

		compiled, err := compileGlobPattern(pattern, cfg)
		if err != nil {
			yield("", err)

			return
		}

		ignorePatterns, ignoreCfg, err := compileIgnorePatterns(cfg)
		if err != nil {
			yield("", err)

			return
		}

		gitIgnoreLoader := newGitIgnoreLoader(cfg)

		emitter := globSeqEmitter{
			yield:          yield,
			ignorePatterns: ignorePatterns,
			ignoreCfg:      ignoreCfg,
		}

		if compiled.globstars > 1 {
			emitter.seen = make(map[string]struct{})
		}

		if !compiled.hasMeta {
			var gitIgnoreCtx *gitIgnoreContext
			if gitIgnoreLoader != nil {
				gitIgnoreCtx = gitIgnoreLoader.contextForDir(gitIgnoreParentDir(compiled.literalPath()))
			}

			emitter.emitLiteral(compiled.literalPath(), compiled.dirOnly, gitIgnoreCtx)
			emitter.finish(cfg)

			return
		}

		base, idx := compiled.globWalkBase()
		if idx > 0 && !isDir(base) {
			emitter.finish(cfg)

			return
		}

		var state *walkState
		baseKey := ""

		if compiled.globstars > 0 {
			state = newWalkState()
			baseKey = resolveWalkKey(base)
		}

		var gitIgnoreCtx *gitIgnoreContext
		if gitIgnoreLoader != nil {
			gitIgnoreCtx = gitIgnoreLoader.contextForDir(base)
		}

		walkGlobSeq(base, baseKey, compiled, idx, cfg, state, &emitter, gitIgnoreCtx)
		emitter.finish(cfg)
	}
}

type globSeqEmitter struct {
	yield          func(string, error) bool
	ignorePatterns []*compiledPath
	ignoreCfg      config
	seen           map[string]struct{}
	matched        bool
	stopped        bool
}

func (emitter *globSeqEmitter) emit(path string, isDir bool, gitIgnoreCtx *gitIgnoreContext) bool {
	if emitter.stopped {
		return false
	}

	if gitIgnoreCtx != nil && gitIgnoreCtx.ignores(path, isDir) {
		return true
	}

	if emitter.seen != nil {
		if _, ok := emitter.seen[path]; ok {
			return true
		}

		emitter.seen[path] = struct{}{}
	}

	for _, pattern := range emitter.ignorePatterns {
		if pattern.matchName(path, emitter.ignoreCfg) {
			return true
		}
	}

	emitter.matched = true
	if !emitter.yield(path, nil) {
		emitter.stopped = true

		return false
	}

	return true
}

func (emitter *globSeqEmitter) emitLiteral(path string, dirOnly bool, gitIgnoreCtx *gitIgnoreContext) bool {
	if _, err := os.Lstat(path); err != nil {
		return true
	}

	isDirPath := false

	if dirOnly {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return true
		}

		isDirPath = true
	} else {
		isDirPath = isDir(path)
	}

	return emitter.emit(path, isDirPath, gitIgnoreCtx)
}

func (emitter *globSeqEmitter) fail(err error) bool {
	if emitter.stopped {
		return false
	}

	emitter.stopped = true

	return emitter.yield("", err)
}

func (emitter *globSeqEmitter) finish(cfg config) {
	if emitter.stopped {
		return
	}

	if !emitter.matched && cfg.failglob {
		emitter.fail(ErrNoMatch)
	}
}

func walkGlobSeq(base, baseKey string, pattern *compiledPath, idx int, cfg config, state *walkState, emitter *globSeqEmitter, gitIgnoreCtx *gitIgnoreContext) bool {
	if idx == len(pattern.segments) {
		entryGitIgnoreCtx := gitIgnoreCtx.parentContext()
		isDirPath := false

		if pattern.dirOnly {
			info, err := os.Stat(base)
			if err != nil || !info.IsDir() {
				return true
			}

			isDirPath = true
		} else {
			isDirPath = isDir(base)
		}

		return emitter.emit(base, isDirPath, entryGitIgnoreCtx)
	}

	segment := pattern.segments[idx]
	if segment.globstar {
		nextIdx := idx + 1
		if nextIdx < len(pattern.segments) {
			nextSegment := pattern.segments[nextIdx]
			if nextSegment.meta && !nextSegment.globstar {
				return walkGlobStarMetaSeq(base, baseKey, pattern, idx, cfg, state, emitter, gitIgnoreCtx)
			}
		}

		if !walkGlobSeq(base, baseKey, pattern, idx+1, cfg, state, emitter, gitIgnoreCtx) {
			return false
		}

		enteredKey, ok := state.enterKey(baseKey)
		if !ok {
			return true
		}
		defer state.leave(enteredKey)

		for _, entry := range readDirEntries(base, cfg.sort) {
			name := entry.Name()
			if shouldSkipName(segment, name, cfg) {
				continue
			}

			child := joinGlobPath(base, name)

			childIsDir, _ := dirEntryInfo(base, entry)
			if gitIgnoreCtx != nil && childIsDir && gitIgnoreCtx.ignores(child, true) {
				continue
			}

			if !childIsDir {
				continue
			}

			nextGitIgnoreCtx := gitIgnoreCtx
			if nextGitIgnoreCtx != nil {
				nextGitIgnoreCtx = nextGitIgnoreCtx.child(child)
			}

			childKey := walkChildKey(baseKey, child, name, entry)

			if !walkGlobSeq(child, childKey, pattern, idx, cfg, state, emitter, nextGitIgnoreCtx) {
				return false
			}
		}

		return true
	}

	if !segment.meta {
		child := joinGlobPath(base, segment.literal)
		if idx == len(pattern.segments)-1 {
			if _, err := os.Lstat(child); err == nil {
				return emitter.emitLiteral(child, pattern.dirOnly, gitIgnoreCtx)
			}

			return true
		}

		if isDir(child) {
			if gitIgnoreCtx != nil && gitIgnoreCtx.ignores(child, true) {
				return true
			}

			nextGitIgnoreCtx := gitIgnoreCtx
			if nextGitIgnoreCtx != nil {
				nextGitIgnoreCtx = nextGitIgnoreCtx.child(child)
			}

			childKey := ""
			if state != nil {
				childKey = walkLiteralChildKey(baseKey, child, segment.literal)
			}

			return walkGlobSeq(child, childKey, pattern, idx+1, cfg, state, emitter, nextGitIgnoreCtx)
		}

		return true
	}

	for _, entry := range readDirEntries(base, cfg.sort) {
		name := entry.Name()
		if shouldSkipName(segment, name, cfg) {
			continue
		}

		if !segment.seq.matchString(name, cfg) {
			continue
		}

		child := joinGlobPath(base, name)
		childIsDir, _ := dirEntryInfo(base, entry)

		if idx == len(pattern.segments)-1 {
			if !pattern.dirOnly || childIsDir {
				if !emitter.emit(child, childIsDir, gitIgnoreCtx) {
					return false
				}
			}

			continue
		}

		if childIsDir {
			if gitIgnoreCtx != nil && gitIgnoreCtx.ignores(child, true) {
				continue
			}

			nextGitIgnoreCtx := gitIgnoreCtx
			if nextGitIgnoreCtx != nil {
				nextGitIgnoreCtx = nextGitIgnoreCtx.child(child)
			}

			childKey := ""
			if state != nil {
				childKey = walkChildKey(baseKey, child, name, entry)
			}

			if !walkGlobSeq(child, childKey, pattern, idx+1, cfg, state, emitter, nextGitIgnoreCtx) {
				return false
			}
		}
	}

	return true
}

func walkGlobStarMetaSeq(base, baseKey string, pattern *compiledPath, idx int, cfg config, state *walkState, emitter *globSeqEmitter, gitIgnoreCtx *gitIgnoreContext) bool {
	entries := readDirEntries(base, cfg.sort)
	if entries == nil {
		return true
	}

	nextIdx := idx + 1
	nextSegment := pattern.segments[nextIdx]
	lastIdx := len(pattern.segments) - 1
	globstarSegment := pattern.segments[idx]

	type pendingDir struct {
		path string
		key  string
	}

	dirs := make([]pendingDir, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()
		child := joinGlobPath(base, name)
		childIsDir, _ := dirEntryInfo(base, entry)
		ignoredDir := gitIgnoreCtx != nil && childIsDir && gitIgnoreCtx.ignores(child, true)

		var childKey string
		if childIsDir && state != nil {
			childKey = walkChildKey(baseKey, child, name, entry)
		}

		if childIsDir && !ignoredDir && !shouldSkipName(globstarSegment, name, cfg) {
			dirs = append(dirs, pendingDir{path: child, key: childKey})
		}

		if shouldSkipName(nextSegment, name, cfg) {
			continue
		}

		if !nextSegment.seq.matchString(name, cfg) {
			continue
		}

		if nextIdx == lastIdx {
			if !pattern.dirOnly || childIsDir {
				if !emitter.emit(child, childIsDir, gitIgnoreCtx) {
					return false
				}
			}

			continue
		}

		if !childIsDir {
			continue
		}

		if ignoredDir {
			continue
		}

		nextGitIgnoreCtx := gitIgnoreCtx
		if nextGitIgnoreCtx != nil {
			nextGitIgnoreCtx = nextGitIgnoreCtx.child(child)
		}

		if !walkGlobSeq(child, childKey, pattern, nextIdx+1, cfg, state, emitter, nextGitIgnoreCtx) {
			return false
		}
	}

	enteredKey, ok := state.enterKey(baseKey)
	if !ok {
		return true
	}
	defer state.leave(enteredKey)

	for _, dir := range dirs {
		nextGitIgnoreCtx := gitIgnoreCtx
		if nextGitIgnoreCtx != nil {
			nextGitIgnoreCtx = nextGitIgnoreCtx.child(dir.path)
		}

		if !walkGlobSeq(dir.path, dir.key, pattern, idx, cfg, state, emitter, nextGitIgnoreCtx) {
			return false
		}
	}

	return true
}

func finalizeMatches(matches []string, cfg config) ([]string, error) {
	if len(matches) != 0 {
		return matches, nil
	}

	if cfg.failglob {
		return nil, ErrNoMatch
	}

	if cfg.nullglob {
		return []string{}, nil
	}

	return nil, nil
}

func compileIgnorePatterns(cfg config) ([]*compiledPath, config, error) {
	if len(cfg.globIgnore) == 0 {
		return nil, config{}, nil
	}

	ignoreCfg := ignoreMatchConfig(cfg)
	ignorePatterns := make([]*compiledPath, 0, len(cfg.globIgnore))

	for _, pattern := range cfg.globIgnore {
		compiled, err := compileMatchPattern(pattern, ignoreCfg)
		if err != nil {
			return nil, config{}, err
		}

		ignorePatterns = append(ignorePatterns, compiled)
	}

	return ignorePatterns, ignoreCfg, nil
}

func ignoreMatchConfig(cfg config) config {
	return config{
		extglob:         cfg.extglob,
		globstar:        cfg.globstar,
		dotglob:         cfg.dotglob,
		globSkipDots:    cfg.globSkipDots,
		globSkipDotsSet: cfg.globSkipDotsSet,
		noCase:          cfg.noCase,
		nullglob:        cfg.nullglob,
		failglob:        cfg.failglob,
		asciiRanges:     cfg.asciiRanges,
		sort:            cfg.sort,
	}
}

func readDirEntries(dir string, sortMode SortMode) []os.DirEntry {
	if sortMode == SortLexicographic {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}

		return entries
	}

	fh, err := os.Open(dir)
	if err != nil {
		return nil
	}

	entries, err := fh.ReadDir(-1)
	if err != nil {
		if closeErr := fh.Close(); closeErr != nil {
			return nil
		}
		return nil
	}

	if err := fh.Close(); err != nil {
		return nil
	}

	return entries
}

func dirEntryInfo(base string, entry os.DirEntry) (bool, fs.FileInfo) {
	if entry.IsDir() {
		return true, nil
	}

	if entry.Type()&fs.ModeSymlink == 0 {
		return false, nil
	}

	info, err := os.Stat(joinGlobPath(base, entry.Name()))
	if err != nil {
		return false, nil
	}

	return info.IsDir(), info
}

func dirEntryIsDir(base string, entry os.DirEntry) bool {
	isDir, _ := dirEntryInfo(base, entry)

	return isDir
}

func (compiled *compiledPath) globWalkBase() (string, int) {
	if compiled.walkBaseValue == "" {
		base := compilePathBase(compiled.volume, compiled.absolute)
		for idx := 0; idx < compiled.prefixLen; idx++ {
			base = joinGlobPath(base, compiled.segments[idx].literal)
		}

		return base, compiled.prefixLen
	}

	return compiled.walkBaseValue, compiled.prefixLen
}

func hasMetaSegments(segments []compiledSegment) bool {
	for _, segment := range segments {
		if segment.meta || segment.globstar {
			return true
		}
	}

	return false
}

func walkChildKey(parentKey, childPath, childName string, entry os.DirEntry) string {
	if entry.Type()&fs.ModeSymlink != 0 {
		return resolveWalkKey(childPath)
	}

	return joinGlobPath(parentKey, childName)
}

func walkLiteralChildKey(parentKey, childPath, childName string) string {
	info, err := os.Lstat(childPath)
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return resolveWalkKey(childPath)
	}

	return joinGlobPath(parentKey, childName)
}

func joinGlobPath(base, elem string) string {
	switch {
	case base == "", base == ".":
		return elem
	case base == string(Separator), strings.HasSuffix(base, string(Separator)):
		return base + elem
	case runtime.GOOS == "windows" && len(base) == 2 && base[1] == ':':
		return base + elem
	default:
		return base + string(Separator) + elem
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func shouldSkipName(segment compiledSegment, name string, cfg config) bool {
	if isSpecialDotSegment(name) && cfg.skipSpecialDots() {
		return true
	}

	if hasLeadingDot(name) && !isSpecialDotSegment(name) && cfg.usesBashPathnameRules() && !cfg.includeHiddenEntries() && !segment.canMatchLeadingDot() {
		return true
	}

	return false
}

func uniqStrings(values []string, sorted bool) []string {
	if len(values) < 2 {
		return values
	}

	if sorted {
		out := 1
		for _, value := range values[1:] {
			if value == values[out-1] {
				continue
			}

			values[out] = value
			out++
		}

		return values[:out]
	}

	seen := make(map[string]struct{}, len(values))
	result := values[:0]

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func exceedsPathSepLimit(pattern string) bool {
	const pathSeparatorsLimit = 10000
	count := 0

	for i := 0; i < len(pattern); i++ {
		if os.IsPathSeparator(pattern[i]) {
			count++
			if count > pathSeparatorsLimit {
				return true
			}
		}
	}

	return false
}
