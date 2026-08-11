package dependencyboundary

import (
	"bufio"
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestProductionDependencyBoundary(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	checkSourceImports(t, root)
	checkResolvedDependencies(t, root)
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func checkSourceImports(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if isResearch(value) {
				t.Errorf("production source %s imports research package %q", path, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan source imports: %v", err)
	}
}

func checkResolvedDependencies(t *testing.T, root string) {
	t.Helper()
	command := exec.Command("go", "list", "-deps", "-test", "-f", `{{if .Module}}{{.ImportPath}}{{"\t"}}{{.Dir}}{{"\t"}}{{.Module.Dir}}{{end}}`, "./...")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("resolve dependencies: %v", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 {
			continue
		}
		importPath, packageDir, moduleDir := fields[0], fields[1], fields[2]
		if isResearch(importPath) || isWithin(filepath.Join(root, "..", "research"), packageDir) || isWithin(filepath.Join(root, "..", "research"), moduleDir) {
			t.Errorf("resolved production dependency %q points under research", importPath)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan resolved dependencies: %v", err)
	}
}

func isResearch(importPath string) bool {
	return strings.Contains(importPath, "/research/") || strings.HasSuffix(importPath, "/research")
}

func isWithin(parent, candidate string) bool {
	if candidate == "" {
		return false
	}
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
