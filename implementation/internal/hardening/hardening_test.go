package hardening

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeImageIsNonRootByDefault(t *testing.T) {
	t.Parallel()

	dockerfile := readText(t, filepath.Join("..", "..", "Dockerfile"))
	requireAll(t, dockerfile,
		"FROM scratch AS runtime",
		"USER 65532:65532",
		"ENTRYPOINT [\"/metricshell\"]",
	)
}

func TestHardenedComposeKeepsOnlyRuntimeDirectoryWritable(t *testing.T) {
	t.Parallel()

	compose := readText(t, filepath.Join("..", "..", "examples", "docker", "hardened-runtime", "compose.yaml"))
	requireAll(t, compose,
		"user: \"65532:65532\"",
		"read_only: true",
		"cap_drop:",
		"- ALL",
		"no-new-privileges:true",
		"pids_limit: 64",
		"mem_limit: 64m",
		"soft: 64",
		"hard: 64",
		"127.0.0.1:19100:9090",
		"/run/metricshell:mode=0700,uid=65532,gid=65532",
		"METRICSHELL_UNIX_SOCKET_PATH: /run/metricshell/ingest.sock",
	)
	requireNotContains(t, compose, "9091:")
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
