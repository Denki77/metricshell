package releaseverify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerfileDefinesStaticMultiArchReleaseArtifacts(t *testing.T) {
	t.Parallel()

	dockerfile := readText(t, filepath.Join("..", "..", "Dockerfile"))
	requireAll(t, dockerfile,
		"FROM source AS release",
		"CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build",
		"CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build",
		"sha256sum linux_amd64/metricshell linux_arm64/metricshell > SHA256SUMS",
		"sha256sum -c SHA256SUMS",
		"org.opencontainers.image.version",
		"org.opencontainers.image.revision",
	)
	requireNotContains(t, dockerfile, "latest")
}

func TestMakefileExportsReleaseThroughDockerOnlyTarget(t *testing.T) {
	t.Parallel()

	makefile := readText(t, filepath.Join("..", "..", "Makefile"))
	requireAll(t, makefile,
		"release:",
		"docker build --target release",
		"--output type=local,dest=\"$(CURDIR)/dist\"",
	)
}

func TestPinnedMultistageCopyExampleForbidsMutableArtifactTags(t *testing.T) {
	t.Parallel()

	readme := readText(t, filepath.Join("..", "..", "examples", "docker", "multistage-copy", "README.md"))
	requireAll(t, readme,
		"COPY --from=ghcr.io/denki77/metricshell-artifact@sha256:<immutable-digest>",
		"Mutable tags alone are not valid release evidence",
	)
}

func readText(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func requireAll(t *testing.T, content string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(content, needle) {
			t.Fatalf("expected %q in content", needle)
		}
	}
}

func requireNotContains(t *testing.T, content, needle string) {
	t.Helper()
	if strings.Contains(content, needle) {
		t.Fatalf("did not expect %q in content", needle)
	}
}
