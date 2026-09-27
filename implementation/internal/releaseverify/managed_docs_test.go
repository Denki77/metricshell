package releaseverify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedReleaseDocumentationIsCompleteAndBilingual(t *testing.T) {
	root := "/project"
	if _, err := os.Stat(filepath.Join(root, "docs")); err != nil {
		root = filepath.Clean(filepath.Join("..", "..", ".."))
	}
	paths := []string{
		"README.md", "README_RU.md", "CHANGELOG.md", "CHANGELOG_RU.md",
		"docs/04-specification/managed-aggregation.md", "docs-ru/04-specification/managed-aggregation.md",
		"docs/04-specification/configuration.md", "docs-ru/04-specification/configuration.md",
		"docs/04-specification/self-metrics.md", "docs-ru/04-specification/self-metrics.md",
		"docs/04-specification/structured-logging.md", "docs-ru/04-specification/structured-logging.md",
	}
	content := make(map[string]string, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content[path] = string(body)
		if strings.Contains(content[path], "managed-aggregation-behavioral-model-draft") {
			t.Fatalf("obsolete draft link remains in %s", path)
		}
	}
	for _, path := range []string{"docs/04-specification/managed-aggregation.md", "docs-ru/04-specification/managed-aggregation.md"} {
		for _, required := range []string{"Accepted normative specification", "ADR-016", "ADR-020", "managed-registry", "unknown", "late"} {
			if !strings.Contains(content[path], required) {
				t.Errorf("%s misses %q", path, required)
			}
		}
	}
	managedOptions := []string{
		"managed-queue-capacity", "managed-max-frame-bytes", "managed-socket-path", "managed-socket-mode",
		"managed-max-connections", "managed-read-timeout", "managed-write-timeout", "managed-max-families",
		"managed-max-series", "managed-max-labels", "managed-max-buckets", "managed-max-batch-operations",
		"managed-max-metric-name-bytes", "managed-max-label-name-bytes", "managed-max-label-value-bytes", "managed-max-help-bytes",
	}
	for _, path := range []string{"docs/04-specification/configuration.md", "docs-ru/04-specification/configuration.md"} {
		for _, option := range managedOptions {
			if !strings.Contains(content[path], option) {
				t.Errorf("%s misses public option %s", path, option)
			}
		}
	}
	for _, path := range []string{"README.md", "README_RU.md", "CHANGELOG.md", "CHANGELOG_RU.md"} {
		if !strings.Contains(content[path], "managed-registry") {
			t.Errorf("%s misses managed release notice", path)
		}
	}
}
