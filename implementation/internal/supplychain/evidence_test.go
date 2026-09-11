package supplychain

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAndVerifyReleaseEvidence(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err != nil {
		t.Fatal(err)
	}
	assertSBOMContainsModule(t, filepath.Join(directory, "SBOM.json"), "github.com/Denki77/metricshell/implementation")
}

func TestGenerateRequiresExternalSigningKey(t *testing.T) {
	directory, inputs := preparedRelease(t)
	inputs.PrivateSigningKeyHex = ""
	if err := Generate(directory, inputs); err == nil || !strings.Contains(err.Error(), "invalid signing key") {
		t.Fatalf("Generate without signing key err = %v", err)
	}
}

func TestGenerateRejectsGovulncheckFinding(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := os.WriteFile(inputs.GovulncheckJSONPath, []byte(`{"finding":{"osv":"GO-2026-0001"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Generate(directory, inputs); err == nil || !strings.Contains(err.Error(), "policy-blocking vulnerabilities") {
		t.Fatalf("Generate with vulnerable input err = %v", err)
	}
}

func TestGenerateRejectsGovulncheckOSVRecord(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := os.WriteFile(inputs.GovulncheckJSONPath, []byte(`{"osv":{"id":"GO-2026-0002"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Generate(directory, inputs); err == nil || !strings.Contains(err.Error(), "policy-blocking vulnerabilities") {
		t.Fatalf("Generate with vulnerable OSV input err = %v", err)
	}
}

func TestVerifyRejectsTamperedBinary(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(directory, "linux_amd64", "metricshell"), "tamper")
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Verify tampered binary err = %v", err)
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS.sig"), []byte("00\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "invalid signature") {
		t.Fatalf("Verify tampered signature err = %v", err)
	}
}

func TestVerifyRejectsIncompleteSBOM(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SBOM.json"), []byte(`{"schema_version":1,"components":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "checksum mismatch: SBOM.json") {
		t.Fatalf("Verify unsigned incomplete SBOM err = %v", err)
	}
	resignManifest(t, directory, inputs)
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "SBOM component missing") {
		t.Fatalf("Verify signed incomplete SBOM err = %v", err)
	}
}

func TestVerifyRejectsTamperedEvidenceMetadata(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "VULNERABILITIES.json"), []byte(`{"schema_version":1,"scanner":"govulncheck","input_sha256":"abc","policy_blocking_findings":[],"decision":"pass"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "checksum mismatch: VULNERABILITIES.json") {
		t.Fatalf("Verify tampered evidence err = %v", err)
	}
}

func TestVerifyRejectsSBOMMissingModuleAndModuleMismatch(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	sbom := SBOM{SchemaVersion: SchemaVersion, Components: []SBOMComponent{
		{Name: "metricshell", Type: "file", Version: inputs.Version, SHA256: mustDigest(t, filepath.Join(directory, "linux_amd64", "metricshell"))},
		{Name: "metricshell", Type: "file", Version: inputs.Version, SHA256: mustDigest(t, filepath.Join(directory, "linux_arm64", "metricshell"))},
		{Name: "github.com/Denki77/metricshell/implementation", Type: "go-module", Version: inputs.Version, SHA256: mustDigest(t, filepath.Join(directory, "go.mod"))},
	}}
	if err := writeJSON(filepath.Join(directory, "SBOM.json"), sbom); err != nil {
		t.Fatal(err)
	}
	resignManifest(t, directory, inputs)
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "SBOM module missing: example.test/dependency") {
		t.Fatalf("Verify missing module err = %v", err)
	}

	sbom.Components = append(sbom.Components, SBOMComponent{Name: "example.test/dependency", Type: "go-module", Version: "v1.2.4", Sum: "h1:test"})
	if err := writeJSON(filepath.Join(directory, "SBOM.json"), sbom); err != nil {
		t.Fatal(err)
	}
	resignManifest(t, directory, inputs)
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "SBOM module mismatch: example.test/dependency") {
		t.Fatalf("Verify module mismatch err = %v", err)
	}
}

func TestVerifyRejectsWrongPublicKeyProvenanceSubjectAndVulnerabilityBlock(t *testing.T) {
	directory, inputs := preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	if err := Verify(directory, inputs.Revision, strings.Repeat("00", ed25519.PublicKeySize)); err == nil || !strings.Contains(err.Error(), "release public key mismatch") {
		t.Fatalf("Verify wrong public key err = %v", err)
	}

	if err := os.WriteFile(filepath.Join(directory, "PROVENANCE.json"), []byte(`{"schema_version":1,"version":"0.1.0","revision":"abc123","builder":"test","subjects":[{"path":"wrong","sha256":"bad"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	resignManifest(t, directory, inputs)
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "provenance subject mismatch") {
		t.Fatalf("Verify wrong provenance err = %v", err)
	}

	directory, inputs = preparedRelease(t)
	if err := Generate(directory, inputs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "VULNERABILITIES.json"), []byte(`{"schema_version":1,"scanner":"govulncheck","input_sha256":"abc","policy_blocking_findings":["GO-test"],"decision":"fail"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	resignManifest(t, directory, inputs)
	if err := Verify(directory, inputs.Revision, inputs.PublicSigningKeyHex); err == nil || !strings.Contains(err.Error(), "policy-blocking vulnerabilities") {
		t.Fatalf("Verify vulnerability block err = %v", err)
	}
}

func preparedRelease(t *testing.T) (string, Inputs) {
	t.Helper()
	directory := t.TempDir()
	for _, subject := range SortedSubjects() {
		path := filepath.Join(directory, subject)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(subject), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	checksums := ""
	for _, subject := range SortedSubjects() {
		digest, err := digestFile(filepath.Join(directory, subject))
		if err != nil {
			t.Fatal(err)
		}
		checksums += digest + "  " + subject + "\n"
	}
	goModPath := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module github.com/Denki77/metricshell/implementation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goModDigest, err := digestFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	checksums += goModDigest + "  go.mod\n"
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(checksums), 0o600); err != nil {
		t.Fatal(err)
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modulesPath := filepath.Join(directory, "modules.jsonl")
	if err := os.WriteFile(modulesPath, []byte(`{"Path":"github.com/Denki77/metricshell/implementation","Main":true}`+"\n"+`{"Path":"example.test/dependency","Version":"v1.2.3","Sum":"h1:test"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vulnerabilitiesPath := filepath.Join(directory, "govulncheck.json")
	if err := os.WriteFile(vulnerabilitiesPath, []byte(`{"config":{"scanner_name":"govulncheck"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory, Inputs{
		Version:              "0.1.0",
		Revision:             "abc123",
		PrivateSigningKeyHex: hex.EncodeToString(privateKey.Seed()),
		PublicSigningKeyHex:  hex.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
		ModulesJSONLPath:     modulesPath,
		GovulncheckJSONPath:  vulnerabilitiesPath,
	}
}

func assertSBOMContainsModule(t *testing.T, path, module string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"type": "go-module"`) || !strings.Contains(string(content), module) {
		t.Fatalf("SBOM does not include module %s: %s", module, string(content))
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func resignManifest(t *testing.T, directory string, inputs Inputs) {
	t.Helper()
	manifest, err := buildManifest(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	privateKey, _, err := signingKeys(inputs.PrivateSigningKeyHex, inputs.PublicSigningKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, manifest)
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS.sig"), []byte(hex.EncodeToString(signature)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustDigest(t *testing.T, path string) string {
	t.Helper()
	digest, err := digestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
