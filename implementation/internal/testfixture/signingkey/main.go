package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: signingkey PRIVATE_KEY_FILE PUBLIC_KEY_FILE")
		os.Exit(2)
	}
	if err := writeSigningKeys(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeSigningKeys(privatePath, publicPath string) error {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.WriteFile(privatePath, []byte(hex.EncodeToString(privateKey.Seed())+"\n"), 0o644); err != nil {
		return err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	if err := os.WriteFile(publicPath, []byte(hex.EncodeToString(publicKey)+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}
