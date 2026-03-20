package richglob

import "errors"

// ErrBadPattern indicates a pattern was malformed.
var ErrBadPattern = errors.New("syntax error in pattern")

// ErrNoMatch reports that pathname expansion produced no matches.
var ErrNoMatch = errors.New("no matches found")

// Option configures Match and Glob behavior.
type Option interface {
	apply(*config)
}

type option func(*config)

func (fn option) apply(cfg *config) {
	fn(cfg)
}

type builtinOption uint8

const (
	builtinOptionExtGlob builtinOption = iota
	builtinOptionGlobStar
	builtinOptionDotGlob
	builtinOptionGitIgnore
	builtinOptionNoCaseGlob
	builtinOptionNullGlob
	builtinOptionFailGlob
	builtinOptionGlobASCIIRanges
)

func (opt builtinOption) apply(cfg *config) {
	switch opt {
	case builtinOptionExtGlob:
		cfg.extglob = true
	case builtinOptionGlobStar:
		cfg.globstar = true
	case builtinOptionDotGlob:
		cfg.dotglob = true
	case builtinOptionGitIgnore:
		cfg.gitIgnore = true
	case builtinOptionNoCaseGlob:
		cfg.noCase = true
	case builtinOptionNullGlob:
		cfg.nullglob = true
	case builtinOptionFailGlob:
		cfg.failglob = true
	case builtinOptionGlobASCIIRanges:
		cfg.asciiRanges = true
	}
}

type withGlobSkipDotsOption bool

func (opt withGlobSkipDotsOption) apply(cfg *config) {
	cfg.globSkipDots = bool(opt)
	cfg.globSkipDotsSet = true
}

type withGlobIgnoreOption []string

func (opt withGlobIgnoreOption) apply(cfg *config) {
	cfg.globIgnore = append(cfg.globIgnore, opt...)
}

type withSortOption SortMode

func (opt withSortOption) apply(cfg *config) {
	cfg.sort = SortMode(opt)
}

// SortMode controls Glob result ordering.
type SortMode int

const (
	// SortLexicographic sorts Glob results in ascending path order.
	SortLexicographic SortMode = iota
	// SortNone leaves Glob results in filesystem traversal order.
	SortNone
)

type config struct {
	extglob         bool
	globstar        bool
	dotglob         bool
	gitIgnore       bool
	globSkipDots    bool
	globSkipDotsSet bool
	noCase          bool
	nullglob        bool
	failglob        bool
	asciiRanges     bool
	globIgnore      []string
	sort            SortMode
}

func defaultConfig() config {
	return config{sort: SortLexicographic}
}

func buildConfig(opts []Option) config {
	if len(opts) == 0 {
		return defaultConfig()
	}

	cfg := defaultConfig()

	for idx, opt := range opts {
		if opt == nil {
			continue
		}

		switch typed := opt.(type) {
		case builtinOption:
			typed.apply(&cfg)
		case withGlobSkipDotsOption:
			typed.apply(&cfg)
		case withGlobIgnoreOption:
			typed.apply(&cfg)
		case withSortOption:
			typed.apply(&cfg)
		default:
			return buildConfigSlow(cfg, opts[idx:])
		}
	}

	return cfg
}

func buildConfigSlow(cfg config, opts []Option) config {
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}

	return cfg
}
// WithExtGlob enables Bash extglob operators such as @(a|b) and !(tmp|cache).
func WithExtGlob() Option {
	return builtinOptionExtGlob
}

// WithGlobStar enables "**" to match zero or more directory segments.
func WithGlobStar() Option {
	return builtinOptionGlobStar
}

// WithDotGlob includes hidden path entries when Bash pathname rules are active.
func WithDotGlob() Option {
	return builtinOptionDotGlob
}

// WithGitIgnore loads and applies .gitignore rules while walking directories.
func WithGitIgnore() Option {
	return builtinOptionGitIgnore
}

// WithGlobSkipDots controls whether "." and ".." are skipped during Bash-style
// pathname expansion.
func WithGlobSkipDots(enabled bool) Option {
	return withGlobSkipDotsOption(enabled)
}

// WithNoCaseGlob enables case-insensitive matching.
func WithNoCaseGlob() Option {
	return builtinOptionNoCaseGlob
}

// WithNullGlob makes Glob return an empty, non-nil slice when nothing matches.
func WithNullGlob() Option {
	return builtinOptionNullGlob
}

// WithFailGlob makes Glob return ErrNoMatch when nothing matches.
func WithFailGlob() Option {
	return builtinOptionFailGlob
}

// WithGlobASCIIRanges makes character ranges such as [A-Z] use ASCII ordering.
func WithGlobASCIIRanges() Option {
	return builtinOptionGlobASCIIRanges
}

// WithGlobIgnore removes matches that also match any of the supplied patterns.
func WithGlobIgnore(patterns ...string) Option {
	return withGlobIgnoreOption(patterns)
}

// WithSort sets the ordering mode used by Glob.
func WithSort(mode SortMode) Option {
	return withSortOption(mode)
}

func (cfg config) usesBashPathnameRules() bool {
	return cfg.extglob || cfg.globstar || cfg.dotglob || cfg.gitIgnore || cfg.globSkipDotsSet || cfg.noCase || cfg.asciiRanges || len(cfg.globIgnore) > 0
}

func (cfg config) skipSpecialDots() bool {
	if cfg.globSkipDotsSet {
		return cfg.globSkipDots
	}

	return cfg.usesBashPathnameRules()
}

func (cfg config) includeHiddenEntries() bool {
	if !cfg.usesBashPathnameRules() {
		return true
	}

	if cfg.gitIgnore {
		return cfg.dotglob
	}

	return cfg.dotglob || len(cfg.globIgnore) > 0
}
