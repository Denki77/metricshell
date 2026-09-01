package dependencyboundary

import (
	"bufio"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
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
	checkPublicSurfaceAndCalls(t, root)
	checkResolvedDependencies(t, root)
	checkModuleLicenses(t, root)
	checkResearchUnavailable(t, root)
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

func checkPublicSurfaceAndCalls(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.TypeSpec:
				if ast.IsExported(value.Name.Name) && prohibitedSharedMemoryName(value.Name.Name) {
					t.Errorf("public type %s in %s exposes shared-memory ABI", value.Name.Name, path)
				}
			case *ast.FuncDecl:
				if ast.IsExported(value.Name.Name) && prohibitedSharedMemoryName(value.Name.Name) {
					t.Errorf("public function %s in %s exposes shared-memory API", value.Name.Name, path)
				}
			case *ast.SelectorExpr:
				if value.Sel.Name == "Mmap" || value.Sel.Name == "Munmap" || value.Sel.Name == "ShmOpen" {
					t.Errorf("production source %s calls shared-memory primitive %s", path, value.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production API: %v", err)
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

type module struct {
	Path string
	Dir  string
	Main bool
}

func checkModuleLicenses(t *testing.T, root string) {
	t.Helper()
	command := exec.Command("go", "list", "-m", "-json", "all")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("resolve module licenses: %v", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for {
		var dependency module
		if err := decoder.Decode(&dependency); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode module: %v", err)
		}
		if dependency.Main {
			continue
		}
		if prohibitedSharedMemoryName(dependency.Path) {
			t.Errorf("production module %q introduces shared-memory dependency", dependency.Path)
		}
		if dependency.Dir == "" || !hasLicense(dependency.Dir) {
			t.Errorf("production module %q has no discoverable license file", dependency.Path)
		}
	}
}

func hasLicense(directory string) bool {
	for _, pattern := range []string{"LICENSE*", "COPYING*", "NOTICE*"} {
		matches, _ := filepath.Glob(filepath.Join(directory, pattern))
		if len(matches) > 0 {
			return true
		}
	}
	return false
}

func checkResearchUnavailable(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "..", "research")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Docker production test context unexpectedly contains research tree")
	}
}

func isResearch(importPath string) bool {
	return strings.Contains(importPath, "/research/") || strings.HasSuffix(importPath, "/research")
}

func prohibitedSharedMemoryName(value string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", "/", "").Replace(value))
	return strings.Contains(normalized, "mmap") || strings.Contains(normalized, "sharedmemory") || strings.Contains(normalized, "shm")
}

func isWithin(parent, candidate string) bool {
	if candidate == "" {
		return false
	}
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
