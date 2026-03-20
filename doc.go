// Package richglob brings Bash-style globbing to your filesystem matching like
// globstar, extglob, case-insensitive matching, ignore patterns, and
// .gitignore-aware traversal.
//
//   - [Match] reports whether a single name matches a pattern.
//   - [Glob] walks the filesystem and returns matching paths.
//   - [GlobSeq2] walks the filesystem lazily and yields matching paths together
//     with a terminal error when one occurs.
//
// By default, the package follows its own matching rules rather than Bash's
// pathname expansion rules. Bash-specific behaviors are enabled explicitly
// with options such as [WithGlobStar], [WithExtGlob], [WithNoCaseGlob],
// [WithDotGlob], and [WithGitIgnore].
package richglob
