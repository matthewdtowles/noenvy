package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCentralizedPathShape(t *testing.T) {
	got, err := CentralizedPath("abc123")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, UserDirName, ProjectsSubdir, "abc123")
	if got != want {
		t.Fatalf("CentralizedPath = %q, want %q", got, want)
	}
}

func TestInProjectPath(t *testing.T) {
	got := InProjectPath("/tmp/myproj")
	want := "/tmp/myproj/.noenvy"
	if got != want {
		t.Fatalf("InProjectPath = %q, want %q", got, want)
	}
}

func TestLocatePrefersInProject(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(InProjectPath(root), []byte("blob"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Note: we don't create a centralized file here. If both existed, in-project
	// should still win, but constructing a centralized fixture would require
	// writing to ~/.noenvy on the real machine — out of scope for a unit test.
	got, err := Locate(root, "doesntmatter")
	if err != nil {
		t.Fatal(err)
	}
	if got != InProjectPath(root) {
		t.Fatalf("Locate returned %q, want in-project path %q", got, InProjectPath(root))
	}
}

func TestLocateNotFound(t *testing.T) {
	root := t.TempDir()
	// Use a project ID that vanishingly likely doesn't exist on disk.
	_, err := Locate(root, "0000000000000000000000nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWriteCreatesParentDirs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "deeply", "nested", "file")
	if err := Write(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("contents = %q, want hello", got)
	}
}

func TestWriteTightensExistingPermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "file")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 0600", perm)
	}
}

func TestWriteReplacesSymlinkInsteadOfFollowing(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "elsewhere")
	if err := os.WriteFile(target, []byte("decoy"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, "file")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("path is still a symlink after Write")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "decoy" {
		t.Fatalf("symlink target was overwritten with %q", got)
	}
}

func TestParseEnvRoundTrip(t *testing.T) {
	plain := []byte("FOO=bar\nBAZ=qux quux\n")
	m, err := ParseEnv(plain)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" || m["BAZ"] != "qux quux" {
		t.Fatalf("parsed wrong: %#v", m)
	}
}
