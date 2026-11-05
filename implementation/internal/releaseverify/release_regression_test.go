package releaseverify

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCurrentInstallDocumentation(t *testing.T) {
	version := strings.TrimSpace(readProjectText(t, "VERSION"))
	for _, path := range []string{"README.md", "README_RU.md", "docs/07-delivery/production-deployment.md", "docs-ru/07-delivery/production-deployment.md"} {
		content := readProjectText(t, filepath.FromSlash(path))
		requireAll(t, content, version)
		for _, m := range regexp.MustCompile(`(?:version=|version-|metricshell(?:-artifact|-example)?:)([0-9]+\.[0-9]+\.[0-9]+)`).FindAllStringSubmatch(content, -1) {
			if m[1] != version {
				t.Errorf("%s: inconsistent version %s", path, m[1])
			}
		}
		requireNotContains(t, content, "VERIFY.md")
		if strings.Contains(path, "deployment") {
			requireAll(t, content, "SHA256SUMS.sig", "RELEASE_PUBLIC_KEY", "VERIFYING.md", "TRUSTED_RELEASE_PUBLIC_KEY_HEX", "openssl pkeyutl -verify", "VERIFY_HASH")
			if !regexp.MustCompile(`(?m)^revision=[0-9a-f]{40}$`).MatchString(content) {
				t.Errorf("%s: missing full release revision", path)
			}
		}
	}
	en := readProjectText(t, "docs/07-delivery/production-deployment.md")
	ru := readProjectText(t, "docs-ru/07-delivery/production-deployment.md")
	blocks := regexp.MustCompile("(?s)```sh\\n(.*?)```")
	a, b := blocks.FindAllString(en, -1), blocks.FindAllString(ru, -1)
	if len(a) != len(b) {
		t.Fatal("installation command count differs")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("installation commands differ: block %d", i)
		}
	}
}

func TestFutureReleaseFinalizesBeforeAttestationAndUpload(t *testing.T) {
	workflow := readProjectText(t, ".github/workflows/release.yml")
	order := []string{"Render deployable Kubernetes release bundle", "Finalize and verify complete signed release manifest", "Generate GitHub artifact attestations", "Publish immutable GitHub Release"}
	previous := -1
	for _, name := range order {
		position := strings.Index(workflow, name)
		if position <= previous {
			t.Fatalf("incorrect pipeline order: %s", name)
		}
		previous = position
	}
	requireAll(t, workflow, "--target finalized-release-artifacts", "--build-context release-input=dist/supply-chain/release", "implementation/dist/supply-chain/release/*", "docker buildx build --check")
	dockerfile := readText(t, "../../examples/docker/base-image/Dockerfile")
	requireNotContains(t, dockerfile, "__METRICSHELL_IMAGE_DIGEST__")
	if !regexp.MustCompile(`(?m)^ARG METRICSHELL_IMAGE_REF=ghcr\.io/denki77/metricshell@sha256:[0-9a-f]{64}$`).MatchString(dockerfile) {
		t.Fatal("example default is not pinned to an immutable runtime digest")
	}
}

func TestAllExternalActionsUseFullCommitPins(t *testing.T) {
	root := "../../.."
	if _, err := os.Stat("/project/.github/workflows"); err == nil {
		root = "/project"
	}
	paths, err := filepath.Glob(filepath.Join(root, ".github/workflows/*"))
	if err != nil {
		t.Fatal(err)
	}
	uses := regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*["']?([^\s"'#]+)`)
	pin := regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)
	for _, path := range paths {
		if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") {
			continue
		}
		content := readText(t, path)
		for _, m := range uses.FindAllStringSubmatch(content, -1) {
			if !strings.HasPrefix(m[1], "./") && !pin.MatchString(m[1]) {
				t.Errorf("%s: unpinned action %s", path, m[1])
			}
		}
	}
}

func TestReleaseSigningUsesEphemeralMounts(t *testing.T) {
	workflow := readProjectText(t, ".github/workflows/release.yml")
	for _, option := range []string{"--secret id=release_signing_key,env=RELEASE_SIGNING_KEY", "--secret id=release_public_key,env=RELEASE_SIGNING_PUBLIC_KEY"} {
		if strings.Count(workflow, option) != 2 {
			t.Fatalf("both signing steps must use %s", option)
		}
	}
	requireNotContains(t, workflow, "release-key.hex")
	requireNotContains(t, workflow, "release-public.hex")
	requireAll(t, workflow, "--no-cache-filter supply-chain", "--no-cache-filter finalize-release")
}
