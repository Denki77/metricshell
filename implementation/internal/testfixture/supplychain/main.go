package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Denki77/metricshell/implementation/internal/supplychain"
)

func main() {
	if len(os.Args) != 8 {
		fmt.Fprintln(os.Stderr, "usage: supplychain RELEASE_DIR VERSION REVISION PRIVATE_KEY_FILE PUBLIC_KEY_FILE MODULES_JSONL GOVULNCHECK_JSON")
		os.Exit(2)
	}
	privateKey, err := readSecret(os.Args[4])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	publicKey, err := readSecret(os.Args[5])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	inputs := supplychain.Inputs{
		Version: os.Args[2], Revision: os.Args[3], PrivateSigningKeyHex: privateKey, PublicSigningKeyHex: publicKey,
		ModulesJSONLPath: os.Args[6], GovulncheckJSONPath: os.Args[7],
	}
	if err := supplychain.Generate(os.Args[1], inputs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := supplychain.Verify(os.Args[1], os.Args[3], publicKey); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readSecret(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}
