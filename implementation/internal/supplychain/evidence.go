package supplychain

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const SchemaVersion = 1

var releaseSubjects = []string{"linux_amd64/metricshell", "linux_arm64/metricshell"}
var signedEvidenceFiles = []string{"MODULES.jsonl", "GOVULNCHECK.json", "SBOM.json", "PROVENANCE.json", "VULNERABILITIES.json", "RELEASE_PUBLIC_KEY", "VERIFYING.md"}

type Inputs struct {
	Version              string
	Revision             string
	PrivateSigningKeyHex string
	PublicSigningKeyHex  string
	ModulesJSONLPath     string
	GovulncheckJSONPath  string
}

type SBOM struct {
	SchemaVersion int             `json:"schema_version"`
	Components    []SBOMComponent `json:"components"`
}

type SBOMComponent struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Sum     string `json:"sum,omitempty"`
}

type Provenance struct {
	SchemaVersion int                 `json:"schema_version"`
	Version       string              `json:"version"`
	Revision      string              `json:"revision"`
	Builder       string              `json:"builder"`
	Subjects      []ProvenanceSubject `json:"subjects"`
}

type ProvenanceSubject struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type VulnerabilityReport struct {
	SchemaVersion          int      `json:"schema_version"`
	Scanner                string   `json:"scanner"`
	InputSHA256            string   `json:"input_sha256"`
	PolicyBlockingFindings []string `json:"policy_blocking_findings"`
	Decision               string   `json:"decision"`
}

type moduleRecord struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Sum     string `json:"Sum"`
	Main    bool   `json:"Main"`
}

func Generate(directory string, inputs Inputs) error {
	privateKey, publicKey, err := signingKeys(inputs.PrivateSigningKeyHex, inputs.PublicSigningKeyHex)
	if err != nil {
		return err
	}
	checksums, err := readChecksums(directory)
	if err != nil {
		return err
	}
	for _, subject := range releaseSubjects {
		if checksums[subject] == "" {
			return fmt.Errorf("checksum subject missing: %s", subject)
		}
	}
	modulesPath := filepath.Join(directory, "MODULES.jsonl")
	if err := copyFile(modulesPath, inputs.ModulesJSONLPath); err != nil {
		return err
	}
	govulncheckPath := filepath.Join(directory, "GOVULNCHECK.json")
	if err := copyFile(govulncheckPath, inputs.GovulncheckJSONPath); err != nil {
		return err
	}
	modules, err := readModules(modulesPath)
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		return errors.New("SBOM module input is empty")
	}
	sbom := SBOM{SchemaVersion: SchemaVersion}
	for _, subject := range releaseSubjects {
		sbom.Components = append(sbom.Components, SBOMComponent{
			Name: filepath.Base(subject), Type: "file", Version: inputs.Version, SHA256: checksums[subject],
		})
	}
	for _, module := range modules {
		component := SBOMComponent{Name: module.Path, Type: "go-module", Version: module.Version, Sum: module.Sum}
		if module.Main {
			component.Version = inputs.Version
			component.SHA256 = checksums["go.mod"]
		}
		sbom.Components = append(sbom.Components, component)
	}
	if err := writeJSON(filepath.Join(directory, "SBOM.json"), sbom); err != nil {
		return err
	}
	provenance := Provenance{
		SchemaVersion: SchemaVersion, Version: inputs.Version, Revision: inputs.Revision, Builder: "implementation/Dockerfile#supply-chain",
	}
	for _, subject := range releaseSubjects {
		provenance.Subjects = append(provenance.Subjects, ProvenanceSubject{Path: subject, SHA256: checksums[subject]})
	}
	if err := writeJSON(filepath.Join(directory, "PROVENANCE.json"), provenance); err != nil {
		return err
	}
	report, err := vulnerabilityReport(govulncheckPath)
	if err != nil {
		return err
	}
	if report.Decision != "pass" {
		return errors.New("policy-blocking vulnerabilities")
	}
	if err := writeJSON(filepath.Join(directory, "VULNERABILITIES.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "RELEASE_PUBLIC_KEY"), []byte(hex.EncodeToString(publicKey)+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "VERIFYING.md"), []byte(offlineInstructions), 0o600); err != nil {
		return err
	}
	manifest, err := buildManifest(directory)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), manifest, 0o600); err != nil {
		return err
	}
	signature := ed25519.Sign(privateKey, manifest)
	return os.WriteFile(filepath.Join(directory, "SHA256SUMS.sig"), []byte(hex.EncodeToString(signature)+"\n"), 0o600)
}

func Verify(directory, expectedRevision, expectedPublicKeyHex string) error {
	checksums, err := readChecksums(directory)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		return err
	}
	signatureContent, err := os.ReadFile(filepath.Join(directory, "SHA256SUMS.sig"))
	if err != nil {
		return err
	}
	signature, err := hex.DecodeString(strings.TrimSpace(string(signatureContent)))
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	publicKey, err := decodePublicKey(expectedPublicKeyHex)
	if err != nil {
		return err
	}
	publishedPublicKey, err := os.ReadFile(filepath.Join(directory, "RELEASE_PUBLIC_KEY"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(publishedPublicKey)) != hex.EncodeToString(publicKey) {
		return errors.New("release public key mismatch")
	}
	if !ed25519.Verify(publicKey, content, signature) {
		return errors.New("invalid signature")
	}
	if err := verifyManifestFiles(directory, checksums); err != nil {
		return err
	}
	if err := verifySBOM(directory, checksums); err != nil {
		return err
	}
	if err := verifyProvenance(directory, checksums, expectedRevision); err != nil {
		return err
	}
	return verifyVulnerabilities(directory, checksums)
}

func signingKeys(privateHex, publicHex string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	privateBytes, err := hex.DecodeString(strings.TrimSpace(privateHex))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid signing key encoding: %w", err)
	}
	var privateKey ed25519.PrivateKey
	switch len(privateBytes) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(privateBytes)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(privateBytes)
	default:
		return nil, nil, errors.New("invalid signing key length")
	}
	publicKey, err := decodePublicKey(publicHex)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(privateKey.Public().(ed25519.PublicKey), publicKey) {
		return nil, nil, errors.New("signing public key does not match private key")
	}
	return privateKey, publicKey, nil
}

func decodePublicKey(publicHex string) (ed25519.PublicKey, error) {
	publicBytes, err := hex.DecodeString(strings.TrimSpace(publicHex))
	if err != nil {
		return nil, fmt.Errorf("invalid public key encoding: %w", err)
	}
	if len(publicBytes) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key length")
	}
	return ed25519.PublicKey(publicBytes), nil
}

func readChecksums(directory string) (map[string]string, error) {
	file, err := os.Open(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	checksums := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line: %q", scanner.Text())
		}
		checksums[strings.TrimPrefix(fields[1], "./")] = fields[0]
	}
	return checksums, scanner.Err()
}

func buildManifest(directory string) ([]byte, error) {
	paths := append([]string(nil), releaseSubjects...)
	paths = append(paths, "go.mod")
	paths = append(paths, signedEvidenceFiles...)
	var builder strings.Builder
	for _, path := range paths {
		digest, err := digestFile(filepath.Join(directory, path))
		if err != nil {
			return nil, err
		}
		builder.WriteString(digest)
		builder.WriteString("  ")
		builder.WriteString(path)
		builder.WriteByte('\n')
	}
	return []byte(builder.String()), nil
}

func verifyManifestFiles(directory string, checksums map[string]string) error {
	required := append([]string(nil), releaseSubjects...)
	required = append(required, "go.mod")
	required = append(required, signedEvidenceFiles...)
	for _, path := range required {
		if checksums[path] == "" {
			return fmt.Errorf("checksum subject missing: %s", path)
		}
	}
	for path, expected := range checksums {
		if path == "SHA256SUMS.sig" {
			return errors.New("signature cannot sign itself")
		}
		actual, err := digestFile(filepath.Join(directory, path))
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("checksum mismatch: %s", path)
		}
	}
	return nil
}

func copyFile(destination, source string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0o600)
}

func readModules(path string) ([]moduleRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var modules []moduleRecord
	for {
		var module moduleRecord
		if err := decoder.Decode(&module); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if module.Path == "" {
			return nil, errors.New("module path is empty")
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func vulnerabilityReport(path string) (VulnerabilityReport, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return VulnerabilityReport{}, err
	}
	inputDigest := sha256.Sum256(content)
	report := VulnerabilityReport{SchemaVersion: SchemaVersion, Scanner: "govulncheck", InputSHA256: hex.EncodeToString(inputDigest[:]), Decision: "pass"}
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	findings := map[string]struct{}{}
	for scanner.Scan() {
		var record struct {
			OSV *struct {
				ID string `json:"id"`
			} `json:"osv"`
			Finding *struct {
				OSV string `json:"osv"`
			} `json:"finding"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.OSV != nil && record.OSV.ID != "" {
			findings[record.OSV.ID] = struct{}{}
		}
		if record.Finding != nil && record.Finding.OSV != "" {
			findings[record.Finding.OSV] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return VulnerabilityReport{}, err
	}
	for finding := range findings {
		report.PolicyBlockingFindings = append(report.PolicyBlockingFindings, finding)
	}
	sort.Strings(report.PolicyBlockingFindings)
	if len(report.PolicyBlockingFindings) > 0 {
		report.Decision = "fail"
	}
	return report, nil
}

func verifySBOM(directory string, checksums map[string]string) error {
	var sbom SBOM
	if err := readJSON(filepath.Join(directory, "SBOM.json"), &sbom); err != nil {
		return err
	}
	if sbom.SchemaVersion != SchemaVersion {
		return errors.New("invalid SBOM schema")
	}
	files := map[string]SBOMComponent{}
	modules := map[string]SBOMComponent{}
	for _, component := range sbom.Components {
		if component.Type == "go-module" && component.Name == "" {
			return errors.New("SBOM module component missing name")
		}
		switch component.Type {
		case "file":
			if component.SHA256 != "" {
				files[component.SHA256] = component
			}
		case "go-module":
			modules[component.Name] = component
		}
	}
	for _, subject := range releaseSubjects {
		if files[checksums[subject]].Name == "" {
			return fmt.Errorf("SBOM component missing: %s", subject)
		}
	}
	expectedModules, err := readModules(filepath.Join(directory, "MODULES.jsonl"))
	if err != nil {
		return err
	}
	if len(expectedModules) == 0 {
		return errors.New("SBOM module input is empty")
	}
	for _, expected := range expectedModules {
		component := modules[expected.Path]
		if component.Name == "" {
			return fmt.Errorf("SBOM module missing: %s", expected.Path)
		}
		if component.Type != "go-module" {
			return fmt.Errorf("SBOM module has wrong type: %s", expected.Path)
		}
		if expected.Main {
			if component.Version == "" || component.SHA256 != checksums["go.mod"] {
				return fmt.Errorf("SBOM module mismatch: %s", expected.Path)
			}
			continue
		}
		if expected.Version == "" || expected.Sum == "" {
			return fmt.Errorf("module input incomplete: %s", expected.Path)
		}
		if component.Version != expected.Version || component.Sum != expected.Sum {
			return fmt.Errorf("SBOM module mismatch: %s", expected.Path)
		}
	}
	if len(modules) != len(expectedModules) {
		return errors.New("SBOM has unexpected module components")
	}
	return nil
}

func verifyProvenance(directory string, checksums map[string]string, expectedRevision string) error {
	var provenance Provenance
	if err := readJSON(filepath.Join(directory, "PROVENANCE.json"), &provenance); err != nil {
		return err
	}
	if provenance.SchemaVersion != SchemaVersion || provenance.Revision != expectedRevision {
		return errors.New("invalid provenance identity")
	}
	subjects := map[string]string{}
	for _, subject := range provenance.Subjects {
		subjects[subject.Path] = subject.SHA256
	}
	for _, subject := range releaseSubjects {
		if subjects[subject] != checksums[subject] {
			return fmt.Errorf("provenance subject mismatch: %s", subject)
		}
	}
	return nil
}

func verifyVulnerabilities(directory string, checksums map[string]string) error {
	var report VulnerabilityReport
	if err := readJSON(filepath.Join(directory, "VULNERABILITIES.json"), &report); err != nil {
		return err
	}
	if report.SchemaVersion != SchemaVersion || report.Scanner != "govulncheck" || report.InputSHA256 == "" || report.Decision != "pass" || len(report.PolicyBlockingFindings) != 0 {
		return errors.New("policy-blocking vulnerabilities")
	}
	if report.InputSHA256 != checksums["GOVULNCHECK.json"] {
		return errors.New("vulnerability input checksum mismatch")
	}
	return nil
}

func digestFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func writeJSON(path string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(content, '\n'), 0o600)
}

func readJSON(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(content, target)
}

func SortedSubjects() []string {
	subjects := append([]string(nil), releaseSubjects...)
	sort.Strings(subjects)
	return subjects
}

const offlineInstructions = `# Offline release verification

1. Run ` + "`sha256sum -c SHA256SUMS`" + ` from the release artifact directory.
2. Verify ` + "`SHA256SUMS.sig`" + ` against ` + "`SHA256SUMS`" + ` with the externally published MetricShell release public key.
3. Confirm ` + "`RELEASE_PUBLIC_KEY`" + ` matches the externally published public key.
4. Confirm ` + "`SHA256SUMS`" + ` includes every binary, ` + "`go.mod`" + `, raw scanner/module input, and generated evidence file.
5. Confirm every provenance subject matches the checksum file.
6. Confirm SBOM components cover every binary subject and every Go module in signed ` + "`MODULES.jsonl`" + ` with matching versions and sums.
7. Confirm ` + "`VULNERABILITIES.json`" + ` was produced from signed ` + "`GOVULNCHECK.json`" + ` input, has decision ` + "`pass`" + `, and no policy-blocking findings.
`
