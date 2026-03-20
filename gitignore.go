package richglob

import (
	"os"
	"path/filepath"
	"strings"
)

const gitIgnoreFileName = ".gitignore"

type gitIgnoreRule struct {
	negate       bool
	dirOnly      bool
	basenameOnly bool
	pattern      *compiledPath
	segment      compiledSegment
}

type gitIgnoreDirRules struct {
	dir   string
	parts pathParts
	rules []gitIgnoreRule
}

type gitIgnoreLoader struct {
	matchCfg config
	cache    map[string]*gitIgnoreDirRules
}

type gitIgnoreContext struct {
	parent *gitIgnoreContext
	loader *gitIgnoreLoader
	rules  *gitIgnoreDirRules
}

func newGitIgnoreLoader(cfg config) *gitIgnoreLoader {
	if !cfg.gitIgnore {
		return nil
	}

	return &gitIgnoreLoader{
		matchCfg: gitIgnoreMatchConfig(cfg),
		cache:    make(map[string]*gitIgnoreDirRules),
	}
}

func gitIgnoreMatchConfig(cfg config) config {
	return config{
		extglob:     cfg.extglob,
		globstar:    true,
		dotglob:     true,
		gitIgnore:   true,
		noCase:      cfg.noCase,
		asciiRanges: cfg.asciiRanges,
	}
}

func (loader *gitIgnoreLoader) contextForDir(dir string) *gitIgnoreContext {
	if loader == nil {
		return nil
	}

	var ctx *gitIgnoreContext

	ancestors := gitIgnoreAncestors(dir)
	for _, ancestor := range ancestors {
		ctx = &gitIgnoreContext{
			parent: ctx,
			loader: loader,
			rules:  loader.load(ancestor),
		}
	}

	return ctx
}

func (ctx *gitIgnoreContext) child(dir string) *gitIgnoreContext {
	if ctx == nil || ctx.loader == nil {
		return nil
	}

	return &gitIgnoreContext{
		parent: ctx,
		loader: ctx.loader,
		rules:  ctx.loader.load(dir),
	}
}

func (ctx *gitIgnoreContext) parentContext() *gitIgnoreContext {
	if ctx == nil {
		return nil
	}

	return ctx.parent
}

func (ctx *gitIgnoreContext) ignores(path string, isDir bool) bool {
	if ctx == nil {
		return false
	}

	ignored := false
	if ctx.parent != nil {
		ignored = ctx.parent.ignores(path, isDir)
	}

	if ctx.rules == nil {
		return ignored
	}

	relPath, relParts, ok := gitIgnoreRelativePath(ctx.rules.parts, path)
	if !ok {
		return ignored
	}

	for _, rule := range ctx.rules.rules {
		if rule.matches(relPath, relParts, isDir, ctx.loader.matchCfg) {
			ignored = !rule.negate
		}
	}

	return ignored
}

func (loader *gitIgnoreLoader) load(dir string) *gitIgnoreDirRules {
	if rules, ok := loader.cache[dir]; ok {
		return rules
	}

	rules := &gitIgnoreDirRules{dir: dir, parts: splitGitIgnorePath(dir)}
	path := joinGlobPath(dir, gitIgnoreFileName)

	data, err := os.ReadFile(path)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			rule, ok := parseGitIgnoreRule(strings.TrimSuffix(line, "\r"), loader.matchCfg)
			if !ok {
				continue
			}

			rules.rules = append(rules.rules, rule)
		}
	}

	loader.cache[dir] = rules

	return rules
}

func parseGitIgnoreRule(line string, cfg config) (gitIgnoreRule, bool) {
	if line == "" {
		return gitIgnoreRule{}, false
	}

	negate := false
	if strings.HasPrefix(line, `\#`) || strings.HasPrefix(line, `\!`) {
		line = line[1:]
	} else {
		if line[0] == '#' {
			return gitIgnoreRule{}, false
		}

		if line[0] == '!' {
			negate = true
			line = line[1:]
		}
	}

	line = strings.ReplaceAll(line, "/", string(Separator))

	dirOnly := strings.HasSuffix(line, string(Separator))
	if dirOnly {
		line = strings.TrimSuffix(line, string(Separator))
	}

	if line == "" {
		return gitIgnoreRule{}, false
	}

	rooted := strings.HasPrefix(line, string(Separator))
	if rooted {
		line = strings.TrimPrefix(line, string(Separator))
	}

	if line == "" {
		return gitIgnoreRule{}, false
	}

	rule := gitIgnoreRule{negate: negate, dirOnly: dirOnly, basenameOnly: !rooted && !strings.ContainsRune(line, Separator)}
	if rule.basenameOnly {
		segment, err := compileSegment(line, cfg)
		if err != nil {
			return gitIgnoreRule{}, false
		}

		rule.segment = segment

		return rule, true
	}

	pattern, err := compileMatchPattern(line, cfg)
	if err != nil {
		return gitIgnoreRule{}, false
	}

	rule.pattern = pattern

	return rule, true
}

func (rule gitIgnoreRule) matches(relPath string, relParts []string, isDir bool, cfg config) bool {
	if rule.dirOnly && !isDir {
		return false
	}

	if rule.basenameOnly {
		for _, part := range relParts {
			if matchGitIgnoreSegment(rule.segment, part, cfg) {
				return true
			}
		}

		return false
	}

	if rule.pattern == nil {
		return false
	}

	return rule.pattern.matchName(relPath, cfg)
}

func matchGitIgnoreSegment(segment compiledSegment, name string, cfg config) bool {
	if segment.globstar {
		return true
	}

	if segment.meta {
		return segment.seq.matchString(name, cfg)
	}

	return matchLiteralString(segment.literal, name, cfg)
}

func gitIgnoreAncestors(dir string) []string {
	parts := splitGitIgnorePath(dir)
	if dir == "." || (!parts.absolute && parts.volume == "" && len(parts.segments) == 0) {
		return []string{"."}
	}

	current := "."
	ancestors := make([]string, 0, len(parts.segments)+1)

	if parts.absolute {
		current = parts.volume + string(Separator)
		ancestors = append(ancestors, current)
	} else if parts.volume != "" {
		current = parts.volume
		ancestors = append(ancestors, current)
	} else {
		ancestors = append(ancestors, current)
	}

	for _, segment := range parts.segments {
		current = joinGlobPath(current, segment)
		ancestors = append(ancestors, current)
	}

	return ancestors
}

func gitIgnoreRelativePath(base pathParts, target string) (string, []string, bool) {
	targetParts := splitGitIgnorePath(target)
	if !sameVolume(base.volume, targetParts.volume) || base.absolute != targetParts.absolute {
		return "", nil, false
	}

	if len(targetParts.segments) < len(base.segments) {
		return "", nil, false
	}

	for idx, segment := range base.segments {
		if targetParts.segments[idx] != segment {
			return "", nil, false
		}
	}

	relParts := targetParts.segments[len(base.segments):]
	if len(relParts) == 0 {
		return "", nil, true
	}

	return strings.Join(relParts, string(Separator)), relParts, true
}

func splitGitIgnorePath(path string) pathParts {
	parts := splitMatchPath(path)
	if parts.absolute || parts.volume != "" {
		return parts
	}

	filtered := make([]string, 0, len(parts.segments))
	for _, segment := range parts.segments {
		if segment == "" || segment == "." {
			continue
		}

		filtered = append(filtered, segment)
	}

	parts.segments = filtered

	return parts
}

func gitIgnoreParentDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "" {
		return "."
	}

	return dir
}
