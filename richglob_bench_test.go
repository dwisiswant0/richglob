package richglob_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "go.dw1.io/richglob"
)

type matchBenchmarkCase struct {
	name    string
	pattern string
	value   string
	opts    []Option
}

type globBenchmarkCase struct {
	name    string
	pattern string
	opts    []Option
}

func BenchmarkMatch(b *testing.B) {
	baselineCases := []matchBenchmarkCase{
		{name: "Literal", pattern: "abc", value: "abc"},
		{name: "Star", pattern: "a*b*c*d*e*/f", value: "axbxcxdxexxx/f"},
		{name: "Class", pattern: "ab[b-d]", value: "abc"},
	}
	for _, benchCase := range baselineCases {
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				bSink, errSink = Match(benchCase.pattern, benchCase.value)
			}
		})
	}

	withCases := []matchBenchmarkCase{
		{name: "NoCaseGlob", pattern: "*.GO", value: "main.go", opts: []Option{WithNoCaseGlob()}},
		{name: "GlobASCIIRanges", pattern: "[A-Z]*.go", value: "main.go", opts: []Option{WithNoCaseGlob(), WithGlobASCIIRanges()}},
		{name: "GlobStar", pattern: filepath.FromSlash("src/**/main.go"), value: filepath.FromSlash("src/cmd/tool/main.go"), opts: []Option{WithGlobStar()}},
		{name: "ExtGlob", pattern: "@(main|util).go", value: "util.go", opts: []Option{WithExtGlob()}},
		{name: "GlobSkipDots", pattern: "*", value: ".hidden.go", opts: []Option{WithGlobSkipDots(true)}},
	}

	for _, benchCase := range withCases {
		b.Run(fmt.Sprintf("With%s", benchCase.name), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				bSink, errSink = Match(benchCase.pattern, benchCase.value, benchCase.opts...)
			}
		})
	}
}

func BenchmarkGlob(b *testing.B) {
	root := benchmarkGlobTree(b)
	baselineCases := []globBenchmarkCase{
		{name: "Flat", pattern: filepath.Join(root, "src", "*.go")},
		{name: "Nested", pattern: filepath.Join(root, "src", "pkg", "*.go")},
	}
	for _, benchCase := range baselineCases {
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_, errSink = Glob(benchCase.pattern)
			}
		})
	}

	withCases := []globBenchmarkCase{
		{name: "DotGlob", pattern: filepath.Join(root, "src", "*.go"), opts: []Option{WithDotGlob()}},
		{name: "GlobSkipDots", pattern: filepath.Join(root, "src", "*"), opts: []Option{WithGlobSkipDots(true)}},
		{name: "NoCaseGlob", pattern: filepath.Join(root, "src", "pkg", "*.go"), opts: []Option{WithNoCaseGlob()}},
		{name: "GlobASCIIRanges", pattern: filepath.Join(root, "src", "pkg", "[A-Z]*.GO"), opts: []Option{WithNoCaseGlob(), WithGlobASCIIRanges()}},
		{name: "GlobStar", pattern: filepath.Join(root, "src", "**", "*.go"), opts: []Option{WithGlobStar()}},
		{name: "ExtGlob", pattern: filepath.Join(root, "src", "pkg", "@(util|main).@(go|txt)"), opts: []Option{WithExtGlob()}},
		{name: "GlobIgnore", pattern: filepath.Join(root, "src", "pkg", "*.go"), opts: []Option{WithNoCaseGlob(), WithGlobIgnore(filepath.Join(root, "src", "pkg", "util.go"))}},
		{name: "SortNone", pattern: filepath.Join(root, "src", "**", "*.go"), opts: []Option{WithGlobStar(), WithSort(SortNone)}},
		{name: "NullGlob", pattern: filepath.Join(root, "missing", "*.go"), opts: []Option{WithNullGlob()}},
		{name: "FailGlob", pattern: filepath.Join(root, "missing", "*.go"), opts: []Option{WithFailGlob()}},
	}

	for _, benchCase := range withCases {
		b.Run(fmt.Sprintf("With%s", benchCase.name), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_, errSink = Glob(benchCase.pattern, benchCase.opts...)
			}
		})
	}
}

func BenchmarkGlobSeq2(b *testing.B) {
	root := benchmarkGlobTree(b)
	benchCases := []globBenchmarkCase{
		{name: "Flat", pattern: filepath.Join(root, "src", "*.go")},
		{name: "GlobStar", pattern: filepath.Join(root, "src", "**", "*.go"), opts: []Option{WithGlobStar()}},
		{name: "GlobIgnore", pattern: filepath.Join(root, "src", "pkg", "*.go"), opts: []Option{WithNoCaseGlob(), WithGlobIgnore(filepath.Join(root, "src", "pkg", "util.go"))}},
	}

	for _, benchCase := range benchCases {
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				for _, err := range GlobSeq2(benchCase.pattern, benchCase.opts...) {
					errSink = err
				}
			}
		})
	}
}

func benchmarkGlobTree(b *testing.B) string {
	b.Helper()
	root := b.TempDir()
	paths := []string{
		filepath.Join(root, "src", "main.go"),
		filepath.Join(root, "src", "tmp.go"),
		filepath.Join(root, "src", ".hidden.go"),
		filepath.Join(root, "src", "pkg", "util.go"),
		filepath.Join(root, "src", "pkg", "UTIL.GO"),
		filepath.Join(root, "src", "pkg", "util.txt"),
		filepath.Join(root, "src", "cmd", "tool", "main.go"),
		filepath.Join(root, "src", "cmd", "tool", "readme.txt"),
	}
	for _, path := range paths {
		writeBenchmarkFile(b, path)
	}
	if runtime.GOOS != "windows" {
		_ = os.Symlink(filepath.Join(root, "src"), filepath.Join(root, "src", "pkg", "loop"))
	}
	return root
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
