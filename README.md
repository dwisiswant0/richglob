# richglob

[![Go Reference](https://pkg.go.dev/badge/go.dw1.io/richglob.svg)](https://pkg.go.dev/go.dw1.io/richglob)
[![tests](https://github.com/dwisiswant0/richglob/actions/workflows/tests.yaml/badge.svg)](https://github.com/dwisiswant0/richglob/actions/workflows/tests.yaml)

Brings Bash-style globbing to Go for filesystem matching like recursive `**`, extended patterns, case-insensitive searches, ignore rules, and more. All opt-in for flexibility.

## Install

```bash
go get go.dw1.io/richglob
```

## Quick start

```go
package main

import (
	"fmt"
	"path/filepath"

	"go.dw1.io/richglob"
)

func main() {
	matched, err := richglob.Match(
		filepath.FromSlash("src/**/main.go"),
		filepath.FromSlash("src/cmd/tool/main.go"),
		richglob.WithGlobStar(),
	)
	if err != nil {
		panic(err)
	}

	println(matched)
}
```

```go
matches, err := richglob.Glob("src/**/*.go", richglob.WithGlobStar())
if err != nil {
	panic(err)
}

for _, match := range matches {
	println(match)
}
```

```go
for match, err := range richglob.GlobSeq2("src/**/*.go", richglob.WithGlobStar()) {
	if err != nil {
		panic(err)
	}

	println(match)
}
```

## Pattern syntax

By default, **richglob** supports the usual glob operators:

- `*` matches any sequence of non-separator characters
- `?` matches any single non-separator character
- `[abc]` matches a character class
- `[^abc]` matches any character outside a class
- `\` escapes metacharacters on non-Windows platforms

Patterns are path-aware. `*` and `?` do not cross path separators.

## Options

- [`WithGlobStar()`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobStar) enables `**` to match zero or more directory segments.
- [`WithExtGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithExtGlob) enables extglob operators: `?(...)`, `*(...)`, `+(...)`, `@(...)`, `!(...)`.
- [`WithNoCaseGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithNoCaseGlob) makes matching case-insensitive.
- [`WithDotGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithDotGlob) includes hidden entries when Bash pathname rules are active.
- [`WithGitIgnore()`](https://pkg.go.dev/go.dw1.io/richglob#WithGitIgnore) auto-discovers `.gitignore` files while walking and applies them as traversal-time filters.
- [`WithGlobSkipDots(true)`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobSkipDots) skips `.` and `..` during Bash-style expansion.
- [`WithGlobASCIIRanges()`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobASCIIRanges) makes ranges such as `[A-Z]` use ASCII ordering.
- [`WithGlobIgnore(patterns...)`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobIgnore) filters `Glob` results through ignore patterns.
- [`WithSort(richglob.SortNone)`](https://pkg.go.dev/go.dw1.io/richglob#WithSort) keeps filesystem traversal order.
- [`WithNullGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithNullGlob) returns an empty, non-nil slice when nothing matches.
- [`WithFailGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithFailGlob) returns [`ErrNoMatch`](https://pkg.go.dev/go.dw1.io/richglob#ErrNoMatch) when nothing matches.


> [!NOTE]
> ### Behavior notes
> - [`Match`](https://pkg.go.dev/go.dw1.io/richglob#Match) checks a single path or path segment string against a pattern.
> - [`Glob`](https://pkg.go.dev/go.dw1.io/richglob#Glob) walks the filesystem and returns matching paths.
> - [`GlobSeq2`](https://pkg.go.dev/go.dw1.io/richglob#GlobSeq2) walks the filesystem lazily and yields `(path, error)` pairs.
> - [`Glob`](https://pkg.go.dev/go.dw1.io/richglob#Glob) sorts results lexicographically by default.
> - [`GlobSeq2`](https://pkg.go.dev/go.dw1.io/richglob#GlobSeq2) yields matches in traversal order and does not globally sort before yielding.
> - If [`Glob`](https://pkg.go.dev/go.dw1.io/richglob#Glob) finds nothing, it returns `nil, nil` by default.
> - [`GlobSeq2`](https://pkg.go.dev/go.dw1.io/richglob#GlobSeq2) yields a terminal error pair for malformed patterns, invalid ignore patterns, or [`WithFailGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithFailGlob).
> - [`WithFailGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithFailGlob) takes precedence over [`WithNullGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithNullGlob).
> - Hidden files are included by default. When Bash-style pathname rules are enabled, hidden entries are excluded unless [`WithDotGlob()`](https://pkg.go.dev/go.dw1.io/richglob#WithDotGlob) is also set. [`WithGlobIgnore(...)`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobIgnore) opts into Bash-style hidden-file handling on its own, while [`WithGitIgnore()`](https://pkg.go.dev/go.dw1.io/richglob#WithGitIgnore) keeps hidden entries filtered unless `WithDotGlob()` is also enabled.
> - When [`WithGitIgnore()`](https://pkg.go.dev/go.dw1.io/richglob#WithGitIgnore) and [`WithGlobIgnore(...)`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobIgnore) are both set, `.gitignore` rules apply first during traversal and [`WithGlobIgnore(...)`](https://pkg.go.dev/go.dw1.io/richglob#WithGlobIgnore) acts as an additional result filter.

## Examples

Case-insensitive matching:

```go
ok, err := richglob.Match("*.GO", "main.go", richglob.WithNoCaseGlob())
```

Recursive search with `**`:

```go
matches, err := richglob.Glob("src/**/*.go", richglob.WithGlobStar())
```

Lazy recursive search:

```go
for match, err := range richglob.GlobSeq2("src/**/*.go", richglob.WithGlobStar()) {
	if err != nil {
		panic(err)
	}

	println(match)
}
```

Extglob alternation:

```go
ok, err := richglob.Match("@(main|util).go", "util.go", richglob.WithExtGlob())
```

Ignore generated files:

```go
matches, err := richglob.Glob(
	"src/**/*.go",
	richglob.WithGlobStar(),
	richglob.WithGlobIgnore("src/**/*_generated.go"),
)
```

Respect nested `.gitignore` files while walking:

```go
matches, err := richglob.Glob(
	"src/**/*.go",
	richglob.WithGlobStar(),
	richglob.WithGitIgnore(),
)
```

## Benchmarks

Compared against the [doublestar](https://github.com/bmatcuk/doublestar) and the std library's [filepath](https://pkg.go.dev/path/filepath).{[Match](https://pkg.go.dev/path/filepath),[Glob](https://pkg.go.dev/path/filepath#Glob)}.

<details open>
  <summary><code>benchstat</code></summary>

```
goos: linux
goarch: amd64
pkg: benchmarks
cpu: AMD EPYC 7763 64-Core Processor                
                  │  richglob   │              doublestar              │                  std                   │
                  │   sec/op    │    sec/op     vs base                │    sec/op     vs base                  │
Match-4             80.05n ± 3%   108.70n ± 0%  +35.80% (p=0.000 n=10)   106.65n ± 0%  +33.24% (p=0.000 n=10)
Match/recursive-4   129.4n ± 0%    105.7n ± 0%  -18.28% (p=0.000 n=10)
Glob-4              13.98µ ± 1%    13.74µ ± 1%   -1.70% (p=0.000 n=10)    17.55µ ± 1%  +25.54% (p=0.000 n=10)
Glob/recursive-4    90.50µ ± 1%   162.24µ ± 1%  +79.26% (p=0.000 n=10)
geomean             1.902µ         2.250µ       +18.25%                   1.368µ       +29.33%                ¹
¹ benchmark set differs from baseline; geomeans may not be comparable

                  │    richglob    │               doublestar                │                   std                    │
                  │      B/op      │     B/op       vs base                  │     B/op      vs base                    │
Match-4               0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=10) ¹     0.000 ± 0%        ~ (p=1.000 n=10) ¹
Match/recursive-4     0.000 ± 0%        0.000 ± 0%        ~ (p=1.000 n=10) ¹
Glob-4              1.495Ki ± 0%      1.872Ki ± 0%  +25.21% (p=0.000 n=10)     1.026Ki ± 0%  -31.35% (p=0.000 n=10)
Glob/recursive-4    8.115Ki ± 0%     11.178Ki ± 0%  +37.74% (p=0.000 n=10)
geomean                          ²                  +14.60%                ²                 -17.15%                ³ ²
¹ all samples are equal
² summaries must be >0 to compute geomean
³ benchmark set differs from baseline; geomeans may not be comparable

                  │   richglob   │              doublestar              │                  std                   │
                  │  allocs/op   │ allocs/op   vs base                  │ allocs/op   vs base                    │
Match-4             0.000 ± 0%     0.000 ± 0%        ~ (p=1.000 n=10) ¹   0.000 ± 0%        ~ (p=1.000 n=10) ¹
Match/recursive-4   0.000 ± 0%     0.000 ± 0%        ~ (p=1.000 n=10) ¹
Glob-4              31.00 ± 0%     53.00 ± 0%  +70.97% (p=0.000 n=10)     24.00 ± 0%  -22.58% (p=0.000 n=10)
Glob/recursive-4    152.0 ± 0%     273.0 ± 0%  +79.61% (p=0.000 n=10)
geomean                        ²               +32.38%                ²               -12.01%                ³ ²
¹ all samples are equal
² summaries must be >0 to compute geomean
³ benchmark set differs from baseline; geomeans may not be comparable
```
</details>

Highlights:

- **richglob** outperforms doublestar by 35-36% in `Match` ops and is **25% faster** than the standard library's `Glob`.
- For recursive globbing (`**`), **richglob** is **79% faster** than doublestar.
- **richglob** uses fewer memory allocs (31 vs 53 for doublestar) and less memory in `Glob` ops.
- Overall geomean perf shows **richglob** is **18% faster** than doublestar and **29% faster** than std.

Run benchmarks yourself:

```bash
make -C benchmarks/
```

## License

**richglob** is released with ♡ by [**@dwisiswant0**](https://github.com/dwisiswant0) under the Apache 2.0 license. See [LICENSE](/LICENSE).