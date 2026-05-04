package erpdb

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestFernetRoundTrip(t *testing.T) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	key := base64.URLEncoding.EncodeToString(keyBytes)

	const secret = "26cc7e1701110ca"
	token, err := encryptFernet(secret, key)
	if err != nil {
		t.Fatalf("encryptFernet() error = %v", err)
	}
	got, err := decryptFernet(token, key)
	if err != nil {
		t.Fatalf("decryptFernet() error = %v", err)
	}
	if got != secret {
		t.Fatalf("expected %q, got %q", secret, got)
	}
}
