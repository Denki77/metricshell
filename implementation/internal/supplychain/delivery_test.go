package supplychain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testRuntimeDigest = "sha256:" + strings.Repeat("a", 64)
var testExampleDigest = "sha256:" + strings.Repeat("b", 64)

const testTemplates = "../../examples/kubernetes/production"

func testOCI() string {
	return "metricshell=ghcr.io/denki77/metricshell@" + testRuntimeDigest +
		"\nartifact=ghcr.io/denki77/metricshell-artifact@" + testRuntimeDigest +
		"\nexample=ghcr.io/denki77/metricshell-example@" + testExampleDigest + "\n"
}

func writeDeliveryFixture(t *testing.T, dir, version string) {
	t.Helper()
	for name, data := range map[string][]byte{
		"METADATA":    []byte("version=" + version + "\n"),
		"OCI_DIGESTS": []byte(testOCI()),
		"metricshell-kubernetes-v" + version + ".tar.gz": bundleFixture(t, nil),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func bundleFixture(t *testing.T, mutate func(*tar.Header, string) string) []byte {
	t.Helper()
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"IMAGE_DIGEST", "job.yaml", "cronjob.yaml", "deployment.yaml", "podmonitor.yaml"} {
		content := testExampleDigest + "\n"
		if name != "IMAGE_DIGEST" {
			data, err := os.ReadFile(filepath.Join(testTemplates, name))
			if err != nil {
				t.Fatal(err)
			}
			content = strings.ReplaceAll(string(data), "sha256:__METRICSHELL_IMAGE_DIGEST__", testExampleDigest)
			if strings.HasPrefix(content, "# TEMPLATE") {
				_, content, _ = strings.Cut(content, "\n")
			}
		}
		header := &tar.Header{Name: "kubernetes-release/" + name, Mode: 0600, Typeflag: tar.TypeReg}
		if mutate != nil {
			content = mutate(header, content)
		}
		if header.Typeflag == tar.TypeReg {
			header.Size = int64(len(content))
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestOCIRejectsWrongRecords(t *testing.T) {
	valid := testOCI()
	if err := validateOCI([]byte(valid), testRuntimeDigest, testExampleDigest); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"extra":            valid + "extra=x\n",
		"duplicate":        valid + strings.Split(valid, "\n")[0] + "\n",
		"missing":          strings.SplitN(valid, "\n", 2)[1],
		"wrong registry":   strings.Replace(valid, "denki77", "other", 1),
		"wrong image":      strings.Replace(valid, "metricshell-artifact@", "metricshell@", 1),
		"malformed":        strings.Replace(valid, testRuntimeDigest, "sha256:xyz", 1),
		"mismatch":         strings.Replace(valid, testRuntimeDigest, testExampleDigest, 1),
		"example mismatch": strings.Replace(valid, testExampleDigest, testRuntimeDigest, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateOCI([]byte(data), testRuntimeDigest, testExampleDigest); err == nil {
				t.Fatal("accepted invalid OCI records")
			}
		})
	}
}

func TestBundleRejectsUnsafeAndInconsistentContent(t *testing.T) {
	for name, mutate := range map[string]func(*tar.Header, string) string{
		"traversal": func(h *tar.Header, s string) string { h.Name = "kubernetes-release/../outside"; return s },
		"symlink":   func(h *tar.Header, s string) string { h.Typeflag = tar.TypeSymlink; h.Linkname = "outside"; return s },
		"duplicate": func(h *tar.Header, s string) string {
			h.Name = "kubernetes-release/IMAGE_DIGEST"
			return testExampleDigest + "\n"
		},
		"placeholder": func(h *tar.Header, s string) string {
			if strings.HasSuffix(h.Name, ".yaml") {
				return strings.ReplaceAll(s, testExampleDigest, "sha256:__METRICSHELL_IMAGE_DIGEST__")
			}
			return s
		},
		"image mismatch": func(h *tar.Header, s string) string {
			if strings.HasSuffix(h.Name, ".yaml") {
				return strings.ReplaceAll(s, testExampleDigest, testRuntimeDigest)
			}
			return s
		},
		"marker mismatch": func(h *tar.Header, s string) string {
			if strings.HasSuffix(h.Name, "IMAGE_DIGEST") {
				return testRuntimeDigest + "\n"
			}
			return s
		},
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "bundle.tar.gz")
			if err := os.WriteFile(file, bundleFixture(t, mutate), 0600); err != nil {
				t.Fatal(err)
			}
			if err := validateBundle(file, testTemplates, testExampleDigest); err == nil {
				t.Fatal("accepted invalid bundle")
			}
		})
	}
	t.Run("truncated gzip", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "bundle.tar.gz")
		data := bundleFixture(t, nil)
		if err := os.WriteFile(file, data[:len(data)-4], 0600); err != nil {
			t.Fatal(err)
		}
		if err := validateBundle(file, testTemplates, testExampleDigest); err == nil {
			t.Fatal("accepted truncated bundle")
		}
	})
}

func TestRejectsStagedSigningKey(t *testing.T) {
	dir, inputs := preparedRelease(t)
	if err := Generate(dir, inputs); err != nil {
		t.Fatal(err)
	}
	writeDeliveryFixture(t, dir, inputs.Version)
	if err := os.WriteFile(filepath.Join(dir, "private.hex"), []byte(inputs.PrivateSigningKeyHex), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Finalize(dir, inputs.Version, inputs.Revision, inputs.PrivateSigningKeyHex, inputs.PublicSigningKeyHex, testRuntimeDigest, testExampleDigest, testTemplates); err == nil {
		t.Fatal("accepted staged signing material")
	}
}
