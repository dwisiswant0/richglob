package richglob_test

import (
	"fmt"
	"os"
	"path/filepath"

	"go.dw1.io/richglob"
)

func ExampleMatch() {
	matched, err := richglob.Match(
		filepath.FromSlash("src/**/main.go"),
		filepath.FromSlash("src/cmd/tool/main.go"),
		richglob.WithGlobStar(),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(matched)
	// Output:
	// true
}

func ExampleGlob() {
	tmpDir, err := os.MkdirTemp("", "richglob-example-")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Println(err)
		}
	}()

	oldWD, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			fmt.Println(err)
		}
	}()

	for _, path := range []string{
		filepath.Join(tmpDir, "src", "main.go"),
		filepath.Join(tmpDir, "src", "pkg", "util.go"),
		filepath.Join(tmpDir, "src", "pkg", "util.txt"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Println(err)
			return
		}
		if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
			fmt.Println(err)
			return
		}
	}

	if err := os.Chdir(tmpDir); err != nil {
		fmt.Println(err)
		return
	}

	matches, err := richglob.Glob(filepath.Join("src", "**", "*.go"), richglob.WithGlobStar())
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, match := range matches {
		fmt.Println(filepath.ToSlash(match))
	}

	// Output:
	// src/main.go
	// src/pkg/util.go
}

func ExampleGlobSeq2() {
	tmpDir, err := os.MkdirTemp("", "richglob-example-")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Println(err)
		}
	}()

	oldWD, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			fmt.Println(err)
		}
	}()

	for _, path := range []string{
		filepath.Join(tmpDir, "src", "main.go"),
		filepath.Join(tmpDir, "src", "pkg", "util.go"),
		filepath.Join(tmpDir, "src", "pkg", "util.txt"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Println(err)
			return
		}
		if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
			fmt.Println(err)
			return
		}
	}

	if err := os.Chdir(tmpDir); err != nil {
		fmt.Println(err)
		return
	}

	for match, err := range richglob.GlobSeq2(filepath.Join("src", "**", "*.go"), richglob.WithGlobStar()) {
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println(filepath.ToSlash(match))
	}

	// Output:
	// src/main.go
	// src/pkg/util.go
}
