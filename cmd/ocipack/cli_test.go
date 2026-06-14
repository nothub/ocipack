package main_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const helloSrc = "package main; import \"fmt\"; func main() { fmt.Println(\"👋\") }"

var (
	helloAMD64 string
	ocipackCLI string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ocipack-cli-test-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	helloAMD64 = buildSrc(dir, "hello-amd64", "linux", "amd64", helloSrc)
	ocipackCLI = buildPkg(dir, "ocipack", runtime.GOOS, runtime.GOARCH, ".")

	os.Exit(m.Run())
}

func buildSrc(dir, name, goos, goarch, src string) string {
	srcFile := filepath.Join(dir, name+".go")
	if err := os.WriteFile(srcFile, []byte(src), 0600); err != nil {
		log.Fatal(err)
	}
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out, srcFile)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("build %s for %s/%s: %v\n%s", name, goos, goarch, err, b)
	}
	return out
}

func buildPkg(dir, name, goos, goarch, pkg string) string {
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("build %s for %s/%s: %v\n%s", pkg, goos, goarch, err, b)
	}
	return out
}

func readTarGz(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	entries := map[string][]byte{}
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		entries[hdr.Name] = data
	}
	return entries
}

func TestCLIProducesValidOCITar(t *testing.T) {
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	cmd := exec.Command(ocipackCLI, helloAMD64, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ocipack: %v\n%s", err, b)
	}

	entries := readTarGz(t, out)
	for _, name := range []string{"oci-layout", "index.json"} {
		if _, ok := entries[name]; !ok {
			t.Errorf("missing %q in CLI output", name)
		}
	}

	var idx struct {
		Manifests []struct {
			Platform struct {
				Architecture string `json:"architecture"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(entries["index.json"], &idx); err != nil {
		t.Fatalf("parse index.json: %v", err)
	}
	if len(idx.Manifests) != 1 || idx.Manifests[0].Platform.Architecture != "amd64" {
		t.Errorf("unexpected index: %s", entries["index.json"])
	}
}

func TestCLIBadInput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	cmd := exec.Command(ocipackCLI, "/nonexistent/binary", out)
	if err := cmd.Run(); err == nil {
		t.Error("expected non-zero exit for non-existent binary")
	}
}

func TestCLIMetadataFlags(t *testing.T) {
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	cmd := exec.Command(ocipackCLI,
		"-label", "app=myapp",
		"-label", "version=1.0",
		"-workdir", "/app",
		"-cmd", "serve",
		"-cmd", "--port=8080",
		"-entrypoint", "/custom",
		"-no-cacerts", "-no-tmp",
		helloAMD64, out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ocipack: %v\n%s", err, b)
	}

	var cfg struct {
		Config struct {
			WorkingDir string            `json:"WorkingDir"`
			Entrypoint []string          `json:"Entrypoint"`
			Cmd        []string          `json:"Cmd"`
			Labels     map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.Unmarshal(extractOCIConfig(t, out), &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if cfg.Config.WorkingDir != "/app" {
		t.Errorf("WorkingDir = %q, want /app", cfg.Config.WorkingDir)
	}
	if len(cfg.Config.Entrypoint) != 1 || cfg.Config.Entrypoint[0] != "/custom" {
		t.Errorf("Entrypoint = %v, want [/custom]", cfg.Config.Entrypoint)
	}
	if len(cfg.Config.Cmd) != 2 || cfg.Config.Cmd[0] != "serve" || cfg.Config.Cmd[1] != "--port=8080" {
		t.Errorf("Cmd = %v, want [serve --port=8080]", cfg.Config.Cmd)
	}
	if cfg.Config.Labels["app"] != "myapp" || cfg.Config.Labels["version"] != "1.0" {
		t.Errorf("Labels = %v, want app=myapp version=1.0", cfg.Config.Labels)
	}
}

func TestCLIAddFile(t *testing.T) {
	content := []byte("hello from extra file")
	extra := filepath.Join(t.TempDir(), "extra.txt")
	if err := os.WriteFile(extra, content, 0644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "image.tar.gz")
	cmd := exec.Command(ocipackCLI,
		"-add-file", "/extra.txt:"+extra+":0644",
		"-no-cacerts", "-no-tmp",
		helloAMD64, out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ocipack: %v\n%s", err, b)
	}

	layer := extractLayer(t, out)
	got, ok := layer["extra.txt"]
	if !ok {
		t.Fatal("extra.txt not found in layer")
	}
	if !bytes.Equal(got, content) {
		t.Errorf("extra.txt content = %q, want %q", got, content)
	}
}

func TestCLIAddLink(t *testing.T) {
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	cmd := exec.Command(ocipackCLI,
		"-add-link", "/mylink:/usr/bin/realbin",
		"-no-cacerts", "-no-tmp",
		helloAMD64, out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ocipack: %v\n%s", err, b)
	}

	entries := readTarGz(t, out)
	mDigest := strings.TrimPrefix(manifestDigest(t, entries), "sha256:")
	var m struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(entries["blobs/sha256/"+mDigest], &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	lDigest := strings.TrimPrefix(m.Layers[0].Digest, "sha256:")
	layerGz := entries["blobs/sha256/"+lDigest]

	gr, err := gzip.NewReader(bytes.NewReader(layerGz))
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "mylink" {
			if hdr.Typeflag != tar.TypeSymlink {
				t.Errorf("mylink typeflag = %d, want TypeSymlink", hdr.Typeflag)
			}
			if hdr.Linkname != "/usr/bin/realbin" {
				t.Errorf("mylink target = %q, want /usr/bin/realbin", hdr.Linkname)
			}
			return
		}
	}
	t.Fatal("mylink not found in layer")
}

// extractOCIConfig follows index → manifest → config blob and returns the raw config JSON.
func extractOCIConfig(t *testing.T, archivePath string) []byte {
	t.Helper()
	entries := readTarGz(t, archivePath)
	mDigest := strings.TrimPrefix(manifestDigest(t, entries), "sha256:")
	var m struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}
	if err := json.Unmarshal(entries["blobs/sha256/"+mDigest], &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	cDigest := strings.TrimPrefix(m.Config.Digest, "sha256:")
	return entries["blobs/sha256/"+cDigest]
}

// extractLayer follows index → manifest → layer and returns a map of tar entry
// names to their content.
func extractLayer(t *testing.T, archivePath string) map[string][]byte {
	t.Helper()
	entries := readTarGz(t, archivePath)
	mDigest := strings.TrimPrefix(manifestDigest(t, entries), "sha256:")
	var m struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(entries["blobs/sha256/"+mDigest], &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	lDigest := strings.TrimPrefix(m.Layers[0].Digest, "sha256:")
	gr, err := gzip.NewReader(bytes.NewReader(entries["blobs/sha256/"+lDigest]))
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	files := map[string][]byte{}
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(tr)
		files[hdr.Name] = data
	}
	return files
}

// manifestDigest returns the manifest digest from index.json.
func manifestDigest(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	var idx struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(entries["index.json"], &idx); err != nil {
		t.Fatalf("parse index.json: %v", err)
	}
	return idx.Manifests[0].Digest
}
