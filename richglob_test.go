package richglob_test

import (
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	. "go.dw1.io/richglob"
)

type MatchTest struct {
	pattern, s string
	match      bool
	err        error
}

var matchTests = []MatchTest{
	{"abc", "abc", true, nil},
	{"*", "abc", true, nil},
	{"*c", "abc", true, nil},
	{"a*", "a", true, nil},
	{"a*", "abc", true, nil},
	{"a*", "ab/c", false, nil},
	{"a*/b", "abc/b", true, nil},
	{"a*/b", "a/c/b", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/xxx/f", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/fff", false, nil},
	{"a*b?c*x", "abxbbxdbxebxczzx", true, nil},
	{"a*b?c*x", "abxbbxdbxebxczzy", false, nil},
	{"ab[c]", "abc", true, nil},
	{"ab[b-d]", "abc", true, nil},
	{"ab[e-g]", "abc", false, nil},
	{"ab[^c]", "abc", false, nil},
	{"ab[^b-d]", "abc", false, nil},
	{"ab[^e-g]", "abc", true, nil},
	{"a\\*b", "a*b", true, nil},
	{"a\\*b", "ab", false, nil},
	{"a?b", "a☺b", true, nil},
	{"a[^a]b", "a☺b", true, nil},
	{"a???b", "a☺b", false, nil},
	{"a[^a][^a][^a]b", "a☺b", false, nil},
	{"[a-ζ]*", "α", true, nil},
	{"*[a-ζ]", "A", false, nil},
	{"a?b", "a/b", false, nil},
	{"a*b", "a/b", false, nil},
	{"[\\]a]", "]", true, nil},
	{"[\\-]", "-", true, nil},
	{"[x\\-]", "x", true, nil},
	{"[x\\-]", "-", true, nil},
	{"[x\\-]", "z", false, nil},
	{"[\\-x]", "x", true, nil},
	{"[\\-x]", "-", true, nil},
	{"[\\-x]", "a", false, nil},
	{"[]a]", "]", false, ErrBadPattern},
	{"[-]", "-", false, ErrBadPattern},
	{"[x-]", "x", false, ErrBadPattern},
	{"[x-]", "-", false, ErrBadPattern},
	{"[x-]", "z", false, ErrBadPattern},
	{"[-x]", "x", false, ErrBadPattern},
	{"[-x]", "-", false, ErrBadPattern},
	{"[-x]", "a", false, ErrBadPattern},
	{"\\", "a", false, ErrBadPattern},
	{"[a-b-c]", "a", false, ErrBadPattern},
	{"[", "a", false, ErrBadPattern},
	{"[^", "a", false, ErrBadPattern},
	{"[^bc", "a", false, ErrBadPattern},
	{"a[", "a", false, ErrBadPattern},
	{"a[", "ab", false, ErrBadPattern},
	{"a[", "x", false, ErrBadPattern},
	{"a/b[", "x", false, ErrBadPattern},
	{"*x", "xxx", true, nil},
}

func errp(e error) string {
	if e == nil {
		return "<nil>"
	}
	return e.Error()
}

func mustHaveSymlink(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	link := filepath.Join(tmpDir, "link")
	if err := os.WriteFile(target, nil, 0666); err != nil {
		t.Fatalf("creating symlink target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("skipping symlink test: %v", err)
	}
}

func tempDirCanonical(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(tmpDir); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(tmpDir)
}

func TestMatch(t *testing.T) {
	for _, tt := range matchTests {
		pattern := tt.pattern
		s := tt.s
		if runtime.GOOS == "windows" {
			if strings.Contains(pattern, "\\") {
				// no escape allowed on windows.
				continue
			}
			pattern = filepath.Clean(pattern)
			s = filepath.Clean(s)
		}
		ok, err := Match(pattern, s)
		if ok != tt.match || err != tt.err {
			t.Errorf("Match(%#q, %#q) = %v, %q want %v, %q", pattern, s, ok, errp(err), tt.match, errp(tt.err))
		}
	}
}

var (
	bSink   bool
	errSink error
)

func writeTestFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		t.Fatalf("creating parent directory for %q: %v", path, err)
	}
	if err := os.WriteFile(path, nil, 0666); err != nil {
		t.Fatalf("creating file %q: %v", path, err)
	}
}

func collectGlobSeq2(seq iter.Seq2[string, error]) ([]string, error) {
	var matches []string

	for match, err := range seq {
		if err != nil {
			return matches, err
		}

		matches = append(matches, match)
	}

	return matches, nil
}

func TestGlob(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	writeTestFile(t, filepath.Join(workDir, "match.go"))
	writeTestFile(t, filepath.Join(workDir, ".hidden.go"))
	writeTestFile(t, filepath.Join(tmpDir, "filepath", "match.go"))
	t.Chdir(workDir)

	tests := []struct {
		pattern string
		want    []string
	}{
		{"match.go", []string{"match.go"}},
		{"mat?h.go", []string{"match.go"}},
		{"*", []string{".hidden.go", "match.go"}},
		{filepath.FromSlash("../filepath/match.go"), []string{filepath.FromSlash("../filepath/match.go")}},
	}

	for _, tt := range tests {
		matches, err := Glob(tt.pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", tt.pattern, err)
			continue
		}
		if !slices.Equal(matches, tt.want) {
			t.Errorf("Glob(%#q) = %#v want %#v", tt.pattern, matches, tt.want)
		}
	}

	for _, pattern := range []string{"no_match", filepath.FromSlash("../filepath/no_match")} {
		matches, err := Glob(pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if len(matches) != 0 {
			t.Errorf("Glob(%#q) = %#v want []", pattern, matches)
		}
	}
}

func TestCVE202230632(t *testing.T) {
	// Prior to CVE-2022-30632, this would cause a stack exhaustion given a
	// large number of separators (more than 4,000,000). There is now a limit
	// of 10,000.
	_, err := Glob("/*" + strings.Repeat("/", 10001))
	if err != ErrBadPattern {
		t.Fatalf("Glob returned err=%v, want ErrBadPattern", err)
	}
}

func TestGlobError(t *testing.T) {
	bad := []string{`[]`, `nonexist/[]`}
	for _, pattern := range bad {
		if _, err := Glob(pattern); err != ErrBadPattern {
			t.Errorf("Glob(%#q) returned err=%v, want ErrBadPattern", pattern, err)
		}
	}
}

func TestMatchOptions(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		opts    []Option
		want    bool
	}{
		{name: "nocase", pattern: "*.GO", value: "main.go", opts: []Option{WithNoCaseGlob()}, want: true},
		{name: "globstar", pattern: filepath.FromSlash("src/**/main.go"), value: filepath.FromSlash("src/cmd/tool/main.go"), opts: []Option{WithGlobStar()}, want: true},
		{name: "extglob alt", pattern: "@(main|util).go", value: "util.go", opts: []Option{WithExtGlob()}, want: true},
		{name: "extglob negate", pattern: "!(tmp|cache)", value: "config", opts: []Option{WithExtGlob()}, want: true},
		{name: "skip dots wildcard", pattern: "*", value: ".", opts: []Option{WithGlobSkipDots(true)}, want: false},
		{name: "skip dots literal", pattern: ".", value: ".", opts: []Option{WithGlobSkipDots(true)}, want: true},
		{name: "bash rules hide dotfile", pattern: "*.go", value: ".hidden.go", opts: []Option{WithNoCaseGlob()}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Match(tt.pattern, tt.value, tt.opts...)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Match(%q, %q) = %v want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestGlobOptions(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "src", "main.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "util.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "UTIL.GO"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "util.txt"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "tmp.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", ".hidden.go"))

	t.Run("nullglob", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "missing", "*.go"), WithNullGlob())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}
		if matches == nil || len(matches) != 0 {
			t.Fatalf("Glob returned %#v, want empty non-nil slice", matches)
		}
	})

	t.Run("failglob wins", func(t *testing.T) {
		_, err := Glob(filepath.Join(tmpDir, "missing", "*.go"), WithNullGlob(), WithFailGlob())
		if err != ErrNoMatch {
			t.Fatalf("Glob returned error %v, want ErrNoMatch", err)
		}
	})

	t.Run("globstar", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "**", "*.go"), WithGlobStar())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}
		want := []string{
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "pkg", "util.go"),
			filepath.Join(tmpDir, "src", "tmp.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})

	t.Run("dotglob restores hidden", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "*.go"), WithNoCaseGlob(), WithDotGlob())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}
		want := []string{
			filepath.Join(tmpDir, "src", ".hidden.go"),
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "tmp.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})

	t.Run("globignore implies dot inclusion", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "*.go"), WithNoCaseGlob(), WithGlobIgnore(filepath.Join(tmpDir, "src", "nomatch")))
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}
		want := []string{
			filepath.Join(tmpDir, "src", ".hidden.go"),
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "tmp.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})

	t.Run("nocase and ignore", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "pkg", "*.go"), WithNoCaseGlob(), WithGlobIgnore(filepath.Join(tmpDir, "src", "pkg", "util.go")))
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}
		if matches != nil {
			t.Fatalf("Glob returned %#v, want nil after nocase ignore removes both matches", matches)
		}
	})

	t.Run("extglob", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "pkg", "@(util|main).@(go|txt)"), WithExtGlob())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}
		want := []string{
			filepath.Join(tmpDir, "src", "pkg", "util.go"),
			filepath.Join(tmpDir, "src", "pkg", "util.txt"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})
}

func TestWithGitIgnore(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "src", "main.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", ".hidden.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "util.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "generated", "keep.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "generated", "skip.go"))

	if err := os.WriteFile(filepath.Join(tmpDir, "src", ".gitignore"), []byte("pkg/generated/*\n"), 0o666); err != nil {
		t.Fatalf("WriteFile(src/.gitignore): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "pkg", ".gitignore"), []byte("!generated/keep.go\n"), 0o666); err != nil {
		t.Fatalf("WriteFile(src/pkg/.gitignore): %v", err)
	}

	t.Run("nested discovery and negation", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "**", "*.go"), WithGlobStar(), WithGitIgnore())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}

		want := []string{
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "pkg", "generated", "keep.go"),
			filepath.Join(tmpDir, "src", "pkg", "util.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})

	t.Run("combines with globignore", func(t *testing.T) {
		matches, err := Glob(
			filepath.Join(tmpDir, "src", "**", "*.go"),
			WithGlobStar(),
			WithGitIgnore(),
			WithGlobIgnore(
				filepath.Join(tmpDir, "src", "pkg", "util.go"),
				filepath.Join(tmpDir, "src", "**", ".gitignore"),
			),
		)
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}

		want := []string{
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "pkg", "generated", "keep.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})

	t.Run("gitignore activates hidden filtering", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "*.go"), WithGitIgnore())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}

		want := []string{filepath.Join(tmpDir, "src", "main.go")}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})

	t.Run("dotglob restores hidden filtering", func(t *testing.T) {
		matches, err := Glob(filepath.Join(tmpDir, "src", "*.go"), WithGitIgnore(), WithDotGlob())
		if err != nil {
			t.Fatalf("Glob returned error: %v", err)
		}

		want := []string{
			filepath.Join(tmpDir, "src", ".hidden.go"),
			filepath.Join(tmpDir, "src", "main.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("Glob returned %#v, want %#v", matches, want)
		}
	})
}

func TestGlobSeq2(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "src", "main.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "util.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "UTIL.GO"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "pkg", "util.txt"))
	writeTestFile(t, filepath.Join(tmpDir, "src", "tmp.go"))
	writeTestFile(t, filepath.Join(tmpDir, "src", ".hidden.go"))

	t.Run("basic", func(t *testing.T) {
		matches, err := collectGlobSeq2(GlobSeq2(filepath.Join(tmpDir, "src", "*.go")))
		if err != nil {
			t.Fatalf("GlobSeq2 returned error: %v", err)
		}

		want := []string{
			filepath.Join(tmpDir, "src", ".hidden.go"),
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "tmp.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("GlobSeq2 returned %#v, want %#v", matches, want)
		}
	})

	t.Run("globstar", func(t *testing.T) {
		matches, err := collectGlobSeq2(GlobSeq2(filepath.Join(tmpDir, "src", "**", "*.go"), WithGlobStar()))
		if err != nil {
			t.Fatalf("GlobSeq2 returned error: %v", err)
		}

		want := []string{
			filepath.Join(tmpDir, "src", "main.go"),
			filepath.Join(tmpDir, "src", "tmp.go"),
			filepath.Join(tmpDir, "src", "pkg", "util.go"),
		}
		if !slices.Equal(matches, want) {
			t.Fatalf("GlobSeq2 returned %#v, want %#v", matches, want)
		}
	})

	t.Run("ignore filters lazily", func(t *testing.T) {
		matches, err := collectGlobSeq2(GlobSeq2(
			filepath.Join(tmpDir, "src", "pkg", "*.go"),
			WithNoCaseGlob(),
			WithGlobIgnore(filepath.Join(tmpDir, "src", "pkg", "util.go")),
		))
		if err != nil {
			t.Fatalf("GlobSeq2 returned error: %v", err)
		}
		if matches != nil {
			t.Fatalf("GlobSeq2 returned %#v, want nil after ignore removes every match", matches)
		}
	})

	t.Run("bad pattern", func(t *testing.T) {
		matches, err := collectGlobSeq2(GlobSeq2("[]"))
		if err != ErrBadPattern {
			t.Fatalf("GlobSeq2 returned error %v, want ErrBadPattern", err)
		}
		if len(matches) != 0 {
			t.Fatalf("GlobSeq2 returned %#v, want no matches on bad pattern", matches)
		}
	})

	t.Run("failglob", func(t *testing.T) {
		matches, err := collectGlobSeq2(GlobSeq2(filepath.Join(tmpDir, "missing", "*.go"), WithFailGlob()))
		if err != ErrNoMatch {
			t.Fatalf("GlobSeq2 returned error %v, want ErrNoMatch", err)
		}
		if len(matches) != 0 {
			t.Fatalf("GlobSeq2 returned %#v, want no matches for failglob miss", matches)
		}
	})

	t.Run("break early", func(t *testing.T) {
		seen := 0
		for match, err := range GlobSeq2(filepath.Join(tmpDir, "src", "*.go")) {
			if err != nil {
				t.Fatalf("GlobSeq2 returned error: %v", err)
			}
			if match == "" {
				t.Fatal("GlobSeq2 yielded an empty match")
			}
			seen++
			break
		}

		if seen != 1 {
			t.Fatalf("GlobSeq2 early break saw %d matches, want 1", seen)
		}
	})
}

func TestExtglobErrors(t *testing.T) {
	for _, pattern := range []string{"@(foo|", "*(bar", "!(a|b"} {
		if _, err := Match(pattern, "foo", WithExtGlob()); err != ErrBadPattern {
			t.Fatalf("Match(%q) returned %v, want ErrBadPattern", pattern, err)
		}
	}
}

func TestExtglobOperators(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{name: "optional empty", pattern: "?(ab)c", value: "c", want: true},
		{name: "optional one", pattern: "?(ab)c", value: "abc", want: true},
		{name: "optional too many", pattern: "?(ab)c", value: "ababc", want: false},
		{name: "star empty", pattern: "*(ab)c", value: "c", want: true},
		{name: "star one", pattern: "*(ab)c", value: "abc", want: true},
		{name: "star many", pattern: "*(ab)c", value: "ababc", want: true},
		{name: "plus empty", pattern: "+(ab)c", value: "c", want: false},
		{name: "plus one", pattern: "+(ab)c", value: "abc", want: true},
		{name: "plus many", pattern: "+(ab)c", value: "abababc", want: true},
		{name: "negate excluded", pattern: "!(ab)c", value: "abc", want: false},
		{name: "negate allowed", pattern: "!(ab)c", value: "xc", want: true},
		{name: "alternation one", pattern: "@(ab|xy)c", value: "xyc", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Match(tt.pattern, tt.value, WithExtGlob())
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Match(%q, %q) = %v want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestGlobUNC(t *testing.T) {
	// Just make sure this runs without crashing for now.
	// See issue 15879.
	_, _ = Glob(`\\?\C:\*`)
}

var globSymlinkTests = []struct {
	path, dest string
	brokenLink bool
}{
	{"test1", "link1", false},
	{"test2", "link2", true},
}

func TestGlobSymlink(t *testing.T) {
	mustHaveSymlink(t)

	tmpDir := t.TempDir()
	for _, tt := range globSymlinkTests {
		path := filepath.Join(tmpDir, tt.path)
		dest := filepath.Join(tmpDir, tt.dest)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		err = os.Symlink(path, dest)
		if err != nil {
			t.Fatal(err)
		}
		if tt.brokenLink {
			// Break the symlink.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		matches, err := Glob(dest)
		if err != nil {
			t.Errorf("GlobSymlink error for %q: %s", dest, err)
		}
		if !slices.Contains(matches, dest) {
			t.Errorf("Glob(%#q) = %#v want %v", dest, matches, dest)
		}
	}
}

func TestGlobStarSymlinkCycle(t *testing.T) {
	mustHaveSymlink(t)
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "root")
	writeTestFile(t, filepath.Join(root, "dir", "file.go"))
	if err := os.Symlink(root, filepath.Join(root, "dir", "loop")); err != nil {
		t.Skipf("skipping symlink cycle test: %v", err)
	}
	matches, err := Glob(filepath.Join(root, "**", "*.go"), WithGlobStar())
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	want := []string{filepath.Join(root, "dir", "file.go")}
	if !slices.Equal(matches, want) {
		t.Fatalf("Glob returned %#v, want %#v", matches, want)
	}
}

func TestGlobStarFollowsSymlinkedDirectory(t *testing.T) {
	mustHaveSymlink(t)
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "root")
	realDir := filepath.Join(root, "real")
	writeTestFile(t, filepath.Join(realDir, "nested", "file.go"))
	if err := os.Symlink(realDir, filepath.Join(root, "link")); err != nil {
		t.Skipf("skipping symlink traversal test: %v", err)
	}
	matches, err := Glob(filepath.Join(root, "**", "*.go"), WithGlobStar())
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	want := []string{
		filepath.Join(root, "link", "nested", "file.go"),
		filepath.Join(root, "real", "nested", "file.go"),
	}
	if !slices.Equal(matches, want) {
		t.Fatalf("Glob returned %#v, want %#v", matches, want)
	}
}

type globTest struct {
	pattern string
	matches []string
}

func (test *globTest) buildWant(root string) []string {
	want := make([]string, 0)
	for _, m := range test.matches {
		want = append(want, root+filepath.FromSlash(m))
	}
	slices.Sort(want)
	return want
}

func (test *globTest) globAbs(root, rootPattern string) error {
	p := filepath.FromSlash(rootPattern + `\` + test.pattern)
	have, err := Glob(p)
	if err != nil {
		return err
	}
	slices.Sort(have)
	want := test.buildWant(root + `\`)
	if slices.Equal(want, have) {
		return nil
	}
	return fmt.Errorf("Glob(%q) returns %q, but %q expected", p, have, want)
}

func (test *globTest) globRel(root string) error {
	p := root + filepath.FromSlash(test.pattern)
	have, err := Glob(p)
	if err != nil {
		return err
	}
	slices.Sort(have)
	want := test.buildWant(root)
	if slices.Equal(want, have) {
		return nil
	}
	// try also matching version without root prefix
	wantWithNoRoot := test.buildWant("")
	if slices.Equal(wantWithNoRoot, have) {
		return nil
	}
	return fmt.Errorf("Glob(%q) returns %q, but %q expected", p, have, want)
}

func TestWindowsGlob(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("skipping windows specific test")
	}

	tmpDir := tempDirCanonical(t)
	if len(tmpDir) < 3 {
		t.Fatalf("tmpDir path %q is too short", tmpDir)
	}
	if tmpDir[1] != ':' {
		t.Fatalf("tmpDir path %q must have drive letter in it", tmpDir)
	}

	dirs := []string{
		"a",
		"b",
		"dir/d/bin",
	}
	files := []string{
		"dir/d/bin/git.exe",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0777)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		err := os.WriteFile(filepath.Join(tmpDir, file), nil, 0666)
		if err != nil {
			t.Fatal(err)
		}
	}

	tests := []globTest{
		{"a", []string{"a"}},
		{"b", []string{"b"}},
		{"c", []string{}},
		{"*", []string{"a", "b", "dir"}},
		{"d*", []string{"dir"}},
		{"*i*", []string{"dir"}},
		{"*r", []string{"dir"}},
		{"?ir", []string{"dir"}},
		{"?r", []string{}},
		{"d*/*/bin/git.exe", []string{"dir/d/bin/git.exe"}},
	}

	// test absolute paths
	for _, test := range tests {
		var p string
		if err := test.globAbs(tmpDir, tmpDir); err != nil {
			t.Error(err)
		}
		// test C:\*Documents and Settings\...
		p = tmpDir
		p = strings.Replace(p, `:\`, `:\*`, 1)
		if err := test.globAbs(tmpDir, p); err != nil {
			t.Error(err)
		}
		// test C:\Documents and Settings*\...
		p = tmpDir
		p = strings.Replace(p, `:\`, `:`, 1)
		p = strings.Replace(p, `\`, `*\`, 1)
		p = strings.Replace(p, `:`, `:\`, 1)
		if err := test.globAbs(tmpDir, p); err != nil {
			t.Error(err)
		}
	}

	// test relative paths
	t.Chdir(tmpDir)
	for _, test := range tests {
		err := test.globRel("")
		if err != nil {
			t.Error(err)
		}
		err = test.globRel(`.\`)
		if err != nil {
			t.Error(err)
		}
		err = test.globRel(tmpDir[:2]) // C:
		if err != nil {
			t.Error(err)
		}
	}
}

func TestNonWindowsGlobEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("skipping non-windows specific test")
	}
	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "match.go"))
	t.Chdir(tmpDir)
	pattern := `\match.go`
	want := []string{"match.go"}
	matches, err := Glob(pattern)
	if err != nil {
		t.Fatalf("Glob error for %q: %s", pattern, err)
	}
	if !slices.Equal(matches, want) {
		t.Fatalf("Glob(%#q) = %v want %v", pattern, matches, want)
	}
}
