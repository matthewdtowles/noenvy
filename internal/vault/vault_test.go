package vault

import (
	"errors"
	"sort"
	"testing"
)

func newVault(initial map[string]string) *Vault {
	if initial == nil {
		initial = map[string]string{}
	}
	cp := map[string]string{}
	for k, v := range initial {
		cp[k] = v
	}
	return &Vault{vars: cp}
}

func TestKeysSorted(t *testing.T) {
	v := newVault(map[string]string{"c": "3", "a": "1", "b": "2"})
	got := v.Keys()
	want := []string{"a", "b", "c"}
	if !equalSlices(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
}

func TestGetSetDelete(t *testing.T) {
	v := newVault(nil)
	if _, ok := v.Get("FOO"); ok {
		t.Fatal("Get on empty vault returned ok=true")
	}
	v.Set("FOO", "bar")
	if val, ok := v.Get("FOO"); !ok || val != "bar" {
		t.Fatalf("after Set, Get returned (%q, %v)", val, ok)
	}
	if !v.Has("FOO") {
		t.Fatal("Has returned false for present key")
	}
	if v.Count() != 1 {
		t.Fatalf("Count = %d, want 1", v.Count())
	}
	if !v.Delete("FOO") {
		t.Fatal("Delete returned false for present key")
	}
	if v.Delete("FOO") {
		t.Fatal("Delete returned true on second call")
	}
}

func TestMergeError(t *testing.T) {
	v := newVault(map[string]string{"A": "1", "B": "2"})
	incoming := map[string]string{"A": "new", "C": "3"}
	_, err := v.Merge(incoming, MergeError)
	if err == nil {
		t.Fatal("expected ErrMergeConflict")
	}
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("expected ErrMergeConflict, got %v", err)
	}
	// Original vault must be unchanged on error.
	if got, _ := v.Get("A"); got != "1" {
		t.Fatalf("vault mutated on MergeError, A=%q", got)
	}
	if v.Has("C") {
		t.Fatal("vault gained C on MergeError")
	}
}

func TestMergeOverwrite(t *testing.T) {
	v := newVault(map[string]string{"A": "1", "B": "2"})
	res, err := v.Merge(map[string]string{"A": "new", "C": "3"}, MergeOverwrite)
	if err != nil {
		t.Fatal(err)
	}
	if !equalSlices(res.Replaced, []string{"A"}) {
		t.Fatalf("Replaced = %v, want [A]", res.Replaced)
	}
	if !equalSlices(res.Added, []string{"C"}) {
		t.Fatalf("Added = %v, want [C]", res.Added)
	}
	if val, _ := v.Get("A"); val != "new" {
		t.Fatalf("A = %q, want new", val)
	}
	if val, _ := v.Get("C"); val != "3" {
		t.Fatalf("C = %q, want 3", val)
	}
}

func TestMergeSkipExisting(t *testing.T) {
	v := newVault(map[string]string{"A": "1", "B": "2"})
	res, err := v.Merge(map[string]string{"A": "new", "C": "3"}, MergeSkipExisting)
	if err != nil {
		t.Fatal(err)
	}
	if !equalSlices(res.Skipped, []string{"A"}) {
		t.Fatalf("Skipped = %v, want [A]", res.Skipped)
	}
	if !equalSlices(res.Added, []string{"C"}) {
		t.Fatalf("Added = %v, want [C]", res.Added)
	}
	if val, _ := v.Get("A"); val != "1" {
		t.Fatalf("A = %q, want 1 (skipped)", val)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a2 := append([]string{}, a...)
	b2 := append([]string{}, b...)
	sort.Strings(a2)
	sort.Strings(b2)
	for i := range a2 {
		if a2[i] != b2[i] {
			return false
		}
	}
	return true
}
