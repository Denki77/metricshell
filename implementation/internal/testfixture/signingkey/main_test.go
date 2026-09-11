package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSigningKeysAreReadableByBuildKit(t *testing.T) {
	directory := t.TempDir()
	privatePath := filepath.Join(directory, "private.hex")
	publicPath := filepath.Join(directory, "public.hex")

	if err := writeSigningKeys(privatePath, publicPath); err != nil {
		t.Fatal(err)
	}
	assertMode(t, privatePath, 0o644)
	assertMode(t, publicPath, 0o644)

	seed, err := readHex(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := readHex(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(seed) != ed25519.SeedSize {
		t.Fatalf("private seed length = %d", len(seed))
	}
	if len(publicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key length = %d", len(publicKey))
	}
	if !ed25519.PrivateKey(ed25519.NewKeyFromSeed(seed)).Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(publicKey)) {
		t.Fatal("public key does not match private seed")
	}
}

func assertMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != expected {
		t.Fatalf("%s mode = %v, want %v", path, mode, expected)
	}
}

func readHex(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(string(content[:len(content)-1]))
}
