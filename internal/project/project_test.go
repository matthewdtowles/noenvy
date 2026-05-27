package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIDDeterministic(t *testing.T) {
	id1, err := ID("/tmp/projects/foo")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := ID("/tmp/projects/foo")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("expected stable ID, got %q vs %q", id1, id2)
	}
	if len(id1) != 32 {
		t.Fatalf("expected 32-char ID, got %d", len(id1))
	}
}

func TestIDDifferentForDifferentPaths(t *testing.T) {
	id1, _ := ID("/tmp/foo")
	id2, _ := ID("/tmp/bar")
	if id1 == id2 {
		t.Fatal("expected different IDs for different paths")
	}
}

func TestIDRelativePathResolved(t *testing.T) {
	cwd, _ := os.Getwd()
	rel := "."
	idRel, _ := ID(rel)
	idAbs, _ := ID(cwd)
	if idRel != idAbs {
		t.Fatalf("ID('.') = %q should match ID(cwd) = %q", idRel, idAbs)
	}
}

func TestFindRootFindsMarker(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "myproj")
	sub := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(sub)
	if err != nil {
		t.Fatal(err)
	}
	wantAbs, _ := filepath.Abs(root)
	gotAbs, _ := filepath.Abs(got)
	// Compare as resolved paths — macOS TempDir may have a /private prefix.
	gotResolved, _ := filepath.EvalSymlinks(gotAbs)
	wantResolved, _ := filepath.EvalSymlinks(wantAbs)
	if gotResolved != wantResolved {
		t.Fatalf("FindRoot returned %q, want %q", gotResolved, wantResolved)
	}
}

func TestFindRootPrefersInnermostMarker(t *testing.T) {
	tmp := t.TempDir()
	outer := filepath.Join(tmp, "monorepo")
	inner := filepath.Join(outer, "services", "api")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(inner)
	if err != nil {
		t.Fatal(err)
	}
	wantResolved, _ := filepath.EvalSymlinks(inner)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Fatalf("FindRoot returned %q (outer marker won), want %q (innermost marker)", gotResolved, wantResolved)
	}
}

func TestFindRootFallsBackToStartDir(t *testing.T) {
	tmp := t.TempDir() // has no markers anywhere in its parents inside test sandbox
	bare := filepath.Join(tmp, "no-markers", "here")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(bare)
	if err != nil {
		t.Fatal(err)
	}
	// Walking up from /tmp/.../bare may eventually find a marker outside the
	// test sandbox (e.g. on the developer's machine). We only assert that
	// when it doesn't, fall back is to startDir or an ancestor of it. A
	// relative path that starts with ".." means got is below bare or
	// unrelated to it, which would be wrong.
	bareResolved, _ := filepath.EvalSymlinks(bare)
	gotResolved, _ := filepath.EvalSymlinks(got)
	rel, err := filepath.Rel(gotResolved, bareResolved)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(rel, "..") {
		t.Fatalf("FindRoot returned unrelated path %q for start %q (rel %q)", gotResolved, bareResolved, rel)
	}
}
