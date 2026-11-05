package supplychain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ociDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validateOCI(data []byte, runtimeDigest, exampleDigest string) error {
	if !ociDigest.MatchString(runtimeDigest) || !ociDigest.MatchString(exampleDigest) {
		return fmt.Errorf("invalid expected OCI digests")
	}
	const registry = "ghcr.io/denki77/"
	expected := map[string]string{
		"metricshell": registry + "metricshell@" + runtimeDigest,
		"artifact":    registry + "metricshell-artifact@" + runtimeDigest,
		"example":     registry + "metricshell-example@" + exampleDigest,
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, ref, ok := strings.Cut(line, "=")
		if !ok || expected[key] == "" || expected[key] != ref {
			return fmt.Errorf("unexpected, duplicate or mismatched OCI_DIGESTS record")
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		return fmt.Errorf("incomplete OCI_DIGESTS")
	}
	return nil
}

func validateBundle(filename, templates, digest string) error {
	expected := map[string][]byte{"IMAGE_DIGEST": []byte(digest + "\n")}
	for _, name := range []string{"cronjob.yaml", "deployment.yaml", "job.yaml", "podmonitor.yaml"} {
		source, err := os.ReadFile(filepath.Join(templates, name))
		if err != nil {
			return err
		}
		var rendered strings.Builder
		for _, line := range strings.SplitAfter(string(source), "\n") {
			if !strings.HasPrefix(line, "# TEMPLATE — NOT DIRECTLY DEPLOYABLE") {
				rendered.WriteString(strings.ReplaceAll(line, "sha256:__METRICSHELL_IMAGE_DIGEST__", digest))
			}
		}
		body := rendered.String()
		if strings.Contains(body, "__METRICSHELL_") || strings.Contains(body, "TEMPLATE — NOT DIRECTLY DEPLOYABLE") {
			return fmt.Errorf("unresolved template marker in %s", name)
		}
		if name != "podmonitor.yaml" && !strings.Contains(body, "image: ghcr.io/denki77/metricshell-example@"+digest+"\n") {
			return fmt.Errorf("missing pinned example image in %s", name)
		}
		expected[name] = []byte(body)
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	archive := tar.NewReader(gz)
	rootSeen := false
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		const root = "kubernetes-release"
		if (header.Name == root || header.Name == root+"/") && header.Typeflag == tar.TypeDir && header.Size == 0 && header.Linkname == "" && !rootSeen {
			rootSeen = true
			continue
		}
		name, ok := strings.CutPrefix(header.Name, root+"/")
		want, found := expected[name]
		if !ok || !found || header.Typeflag != tar.TypeReg || header.Linkname != "" || header.Size != int64(len(want)) {
			return fmt.Errorf("unexpected Kubernetes bundle entry %q", header.Name)
		}
		data, err := io.ReadAll(archive)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, want) {
			return fmt.Errorf("Kubernetes bundle entry %q differs from rendered release content", header.Name)
		}
		delete(expected, name)
	}
	if len(expected) != 0 {
		return fmt.Errorf("incomplete Kubernetes release bundle")
	}
	// tar EOF may precede the gzip trailer. Validate the trailer and reject
	// hidden content after the archive, allowing only tar record padding.
	var padding [4096]byte
	for {
		n, err := gz.Read(padding[:])
		for _, b := range padding[:n] {
			if b != 0 {
				return fmt.Errorf("unexpected trailing Kubernetes bundle data")
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
