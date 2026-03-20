package richglob

import "go.dw1.io/richglob/internal/cache"

type compileCacheMode uint8

const (
	compileCacheMatch compileCacheMode = iota
	compileCacheGlob
)

type compileCacheKey struct {
	mode        compileCacheMode
	pattern     string
	extglob     bool
	globstar    bool
	noCase      bool
	asciiRanges bool
}

type pathPartsCacheMode uint8

const (
	pathPartsCacheMatch pathPartsCacheMode = iota
	pathPartsCacheGlob
)

type pathPartsCacheKey struct {
	mode pathPartsCacheMode
	path string
}

var globalCompiledPatternCache = cache.New[compileCacheKey, *compiledPath](256)

var globalPathPartsCache = cache.New[pathPartsCacheKey, pathParts](512)

func makeCompileCacheKey(mode compileCacheMode, pattern string, cfg config) compileCacheKey {
	return compileCacheKey{
		mode:        mode,
		pattern:     pattern,
		extglob:     cfg.extglob,
		globstar:    cfg.globstar,
		noCase:      cfg.noCase,
		asciiRanges: cfg.asciiRanges,
	}
}
