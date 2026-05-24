package cryptobox

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	pt := []byte("DATABASE_URL=postgres://localhost/db\nAPI_KEY=sk-abc123\n")
	ct, err := Encrypt(key, pt)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("DATABASE_URL")) {
		t.Fatal("ciphertext contains plaintext")
	}
	got, err := Decrypt(key, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, got) {
		t.Fatalf("round-trip mismatch: %q vs %q", pt, got)
	}
}

func TestWrongKeyFails(t *testing.T) {
	k1, _ := NewKey()
	k2, _ := NewKey()
	ct, err := Encrypt(k1, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(k2, ct); err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestTamperDetection(t *testing.T) {
	key, _ := NewKey()
	ct, err := Encrypt(key, []byte("secret payload"))
	if err != nil {
		t.Fatal(err)
	}
	ct[len(ct)-1] ^= 0xff // flip a bit in the auth tag
	if _, err := Decrypt(key, ct); err == nil {
		t.Fatal("expected auth failure on tampered ciphertext")
	}
}

func TestVersionRejected(t *testing.T) {
	key, _ := NewKey()
	ct, _ := Encrypt(key, []byte("x"))
	ct[0] = 0x02
	if _, err := Decrypt(key, ct); err == nil {
		t.Fatal("expected error on unknown version")
	}
}
