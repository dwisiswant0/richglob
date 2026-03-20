package benchmarks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bmatcuk/doublestar/v4"
	"go.dw1.io/richglob"
)

type benchmarkFixture struct {
	matchPattern          string
	matchPath             string
	recursiveMatchPattern string
	recursiveMatchPath    string
	flatGlobPattern       string
	recursiveGlobPattern  string
	flatGlobMatches       int
	recursiveGlobMatches  int
}

var (
	boolSink    bool
	matchesSink []string
	errSink     error
)

func newBenchmarkFixture(b *testing.B) benchmarkFixture {
	b.Helper()

	root := b.TempDir()
	paths := []string{
		filepath.Join(root, "src", "main.go"),
		filepath.Join(root, "src", "root_test.go"),
		filepath.Join(root, "src", "tmp.go"),
		filepath.Join(root, "src", "version.go"),
		filepath.Join(root, "src", "cmd", "tool", "main.go"),
		filepath.Join(root, "src", "cmd", "tool", "readme.txt"),
		filepath.Join(root, "src", "pkg", "util.go"),
		filepath.Join(root, "src", "pkg", "util.txt"),
		filepath.Join(root, "src", "pkg", "helper", "helper.go"),
		filepath.Join(root, "src", "pkg", "helper", "helper_test.go"),
		filepath.Join(root, "src", "services", "api", "handler.go"),
		filepath.Join(root, "src", "services", "api", "main.go"),
		filepath.Join(root, "src", "services", "worker", "job.go"),
		filepath.Join(root, "src", "services", "worker", "main.go"),
	}

	for _, path := range paths {
		writeBenchmarkFile(b, path)
	}

	return benchmarkFixture{
		matchPattern:          filepath.Join("src", "pkg", "*.go"),
		matchPath:             filepath.Join("src", "pkg", "util.go"),
		recursiveMatchPattern: filepath.Join("src", "**", "main.go"),
		recursiveMatchPath:    filepath.Join("src", "cmd", "tool", "main.go"),
		flatGlobPattern:       filepath.Join(root, "src", "*.go"),
		recursiveGlobPattern:  filepath.Join(root, "src", "**", "*.go"),
		flatGlobMatches:       4,
		recursiveGlobMatches:  12,
	}
}

func writeBenchmarkFile(b *testing.B, path string) {
	b.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		b.Fatalf("creating parent directory for %q: %v", path, err)
	}

	if err := os.WriteFile(path, nil, 0o666); err != nil {
		b.Fatalf("creating file %q: %v", path, err)
	}
}

func BenchmarkRichglob(b *testing.B) {
	fixture := newBenchmarkFixture(b)

	b.Run("Match", func(b *testing.B) {
		ok, err := richglob.Match(fixture.matchPattern, fixture.matchPath)
		if err != nil {
			b.Fatalf("richglob.Match returned error during setup: %v", err)
		}
		if !ok {
			b.Fatalf("richglob.Match(%q, %q) = false during setup", fixture.matchPattern, fixture.matchPath)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			boolSink, errSink = richglob.Match(fixture.matchPattern, fixture.matchPath)
		}
	})

	b.Run("Match/recursive", func(b *testing.B) {
		ok, err := richglob.Match(fixture.recursiveMatchPattern, fixture.recursiveMatchPath, richglob.WithGlobStar())
		if err != nil {
			b.Fatalf("richglob.Match returned error during setup: %v", err)
		}
		if !ok {
			b.Fatalf("richglob.Match(%q, %q) = false during setup", fixture.recursiveMatchPattern, fixture.recursiveMatchPath)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			boolSink, errSink = richglob.Match(fixture.recursiveMatchPattern, fixture.recursiveMatchPath, richglob.WithGlobStar())
		}
	})

	b.Run("Glob", func(b *testing.B) {
		matches, err := richglob.Glob(fixture.flatGlobPattern)
		if err != nil {
			b.Fatalf("richglob.Glob returned error during setup: %v", err)
		}
		if len(matches) != fixture.flatGlobMatches {
			b.Fatalf("richglob.Glob(%q) returned %d matches during setup, want %d", fixture.flatGlobPattern, len(matches), fixture.flatGlobMatches)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			matchesSink, errSink = richglob.Glob(fixture.flatGlobPattern)
		}
	})

	b.Run("Glob/recursive", func(b *testing.B) {
		matches, err := richglob.Glob(fixture.recursiveGlobPattern, richglob.WithGlobStar())
		if err != nil {
			b.Fatalf("richglob.Glob returned error during setup: %v", err)
		}
		if len(matches) != fixture.recursiveGlobMatches {
			b.Fatalf("richglob.Glob(%q) returned %d matches during setup, want %d", fixture.recursiveGlobPattern, len(matches), fixture.recursiveGlobMatches)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			matchesSink, errSink = richglob.Glob(fixture.recursiveGlobPattern, richglob.WithGlobStar())
		}
	})
}

func BenchmarkDoublestar(b *testing.B) {
	fixture := newBenchmarkFixture(b)

	b.Run("Match", func(b *testing.B) {
		ok, err := doublestar.PathMatch(fixture.matchPattern, fixture.matchPath)
		if err != nil {
			b.Fatalf("doublestar.PathMatch returned error during setup: %v", err)
		}
		if !ok {
			b.Fatalf("doublestar.PathMatch(%q, %q) = false during setup", fixture.matchPattern, fixture.matchPath)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			boolSink, errSink = doublestar.PathMatch(fixture.matchPattern, fixture.matchPath)
		}
	})

	b.Run("Match/recursive", func(b *testing.B) {
		ok, err := doublestar.PathMatch(fixture.recursiveMatchPattern, fixture.recursiveMatchPath)
		if err != nil {
			b.Fatalf("doublestar.PathMatch returned error during setup: %v", err)
		}
		if !ok {
			b.Fatalf("doublestar.PathMatch(%q, %q) = false during setup", fixture.recursiveMatchPattern, fixture.recursiveMatchPath)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			boolSink, errSink = doublestar.PathMatch(fixture.recursiveMatchPattern, fixture.recursiveMatchPath)
		}
	})

	b.Run("Glob", func(b *testing.B) {
		matches, err := doublestar.FilepathGlob(fixture.flatGlobPattern)
		if err != nil {
			b.Fatalf("doublestar.FilepathGlob returned error during setup: %v", err)
		}
		if len(matches) != fixture.flatGlobMatches {
			b.Fatalf("doublestar.FilepathGlob(%q) returned %d matches during setup, want %d", fixture.flatGlobPattern, len(matches), fixture.flatGlobMatches)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			matchesSink, errSink = doublestar.FilepathGlob(fixture.flatGlobPattern)
		}
	})

	b.Run("Glob/recursive", func(b *testing.B) {
		matches, err := doublestar.FilepathGlob(fixture.recursiveGlobPattern)
		if err != nil {
			b.Fatalf("doublestar.FilepathGlob returned error during setup: %v", err)
		}
		if len(matches) != fixture.recursiveGlobMatches {
			b.Fatalf("doublestar.FilepathGlob(%q) returned %d matches during setup, want %d", fixture.recursiveGlobPattern, len(matches), fixture.recursiveGlobMatches)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			matchesSink, errSink = doublestar.FilepathGlob(fixture.recursiveGlobPattern)
		}
	})
}

func BenchmarkStd(b *testing.B) {
	fixture := newBenchmarkFixture(b)

	b.Run("Match", func(b *testing.B) {
		ok, err := filepath.Match(fixture.matchPattern, fixture.matchPath)
		if err != nil {
			b.Fatalf("filepath.Match returned error during setup: %v", err)
		}
		if !ok {
			b.Fatalf("filepath.Match(%q, %q) = false during setup", fixture.matchPattern, fixture.matchPath)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			boolSink, errSink = filepath.Match(fixture.matchPattern, fixture.matchPath)
		}
	})

	b.Run("Glob", func(b *testing.B) {
		matches, err := filepath.Glob(fixture.flatGlobPattern)
		if err != nil {
			b.Fatalf("filepath.Glob returned error during setup: %v", err)
		}
		if len(matches) != fixture.flatGlobMatches {
			b.Fatalf("filepath.Glob(%q) returned %d matches during setup, want %d", fixture.flatGlobPattern, len(matches), fixture.flatGlobMatches)
		}

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			matchesSink, errSink = filepath.Glob(fixture.flatGlobPattern)
		}
	})
}
