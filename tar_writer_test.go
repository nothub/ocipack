package ocipack

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestWriteTarCreatesFile(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "latest"); err != nil {
		t.Fatalf("WriteTar: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output file not created: %v", err)
	}
}

func TestWriteTarRequiredEntries(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "latest"); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)
	for _, name := range []string{"oci-layout", "index.json"} {
		if _, ok := entries[name]; !ok {
			t.Errorf("missing required tar entry %q", name)
		}
	}
}

func TestWriteTarOCILayout(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, ""); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)
	var layout struct {
		ImageLayoutVersion string `json:"imageLayoutVersion"`
	}
	if err := json.Unmarshal(entries["oci-layout"], &layout); err != nil {
		t.Fatalf("oci-layout parse error: %v", err)
	}
	if layout.ImageLayoutVersion != "1.0.0" {
		t.Errorf("imageLayoutVersion = %q, want 1.0.0", layout.ImageLayoutVersion)
	}
}

func TestWriteTarDigestConsistency(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "latest"); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)

	// Parse index
	var idx index
	if err := json.Unmarshal(entries["index.json"], &idx); err != nil {
		t.Fatalf("index.json parse error: %v", err)
	}
	if len(idx.Manifests) != 1 {
		t.Fatalf("expected 1 manifest in index, got %d", len(idx.Manifests))
	}
	manifestDesc := idx.Manifests[0]

	// Manifest blob: exists and digest matches
	manifestHex, _ := digestHex(manifestDesc.Digest)
	manifestBlob, ok := entries["blobs/sha256/"+manifestHex]
	if !ok {
		t.Fatalf("manifest blob blobs/sha256/%s not found in tar", manifestHex)
	}
	if digestSHA256(manifestBlob) != manifestDesc.Digest {
		t.Error("manifest blob digest mismatch")
	}

	// Parse manifest
	var m manifest
	if err := json.Unmarshal(manifestBlob, &m); err != nil {
		t.Fatalf("manifest parse error: %v", err)
	}
	if m.SchemaVersion != 2 {
		t.Errorf("manifest schemaVersion = %d, want 2", m.SchemaVersion)
	}
	if len(m.Layers) != 0 {
		t.Errorf("expected empty layers, got %v", m.Layers)
	}

	// Config blob: exists and digest matches
	configHex, _ := digestHex(m.Config.Digest)
	configBlob, ok := entries["blobs/sha256/"+configHex]
	if !ok {
		t.Fatalf("config blob blobs/sha256/%s not found in tar", configHex)
	}
	if digestSHA256(configBlob) != m.Config.Digest {
		t.Error("config blob digest mismatch")
	}

	// Parse config
	var cfg imageConfig
	if err := json.Unmarshal(configBlob, &cfg); err != nil {
		t.Fatalf("config parse error: %v", err)
	}
	if cfg.RootFS.Type != "layers" {
		t.Errorf("rootfs.type = %q, want layers", cfg.RootFS.Type)
	}
	if len(cfg.RootFS.DiffIDs) != 0 {
		t.Errorf("expected empty diff_ids, got %v", cfg.RootFS.DiffIDs)
	}
}

func TestWriteTarRefName(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "v1.2.3"); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)
	if !strings.Contains(string(entries["index.json"]), "v1.2.3") {
		t.Errorf("expected refName v1.2.3 in index.json, got: %s", entries["index.json"])
	}
}

func TestWriteTarNoRefName(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, ""); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)
	if strings.Contains(string(entries["index.json"]), "org.opencontainers.image.ref.name") {
		t.Errorf("expected no ref.name annotation for empty refName, got: %s", entries["index.json"])
	}
}

func TestWriteTarPlatformInIndex(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "arm64"})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, ""); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)
	var idx index
	if err := json.Unmarshal(entries["index.json"], &idx); err != nil {
		t.Fatal(err)
	}
	p := idx.Manifests[0].Platform
	if p == nil {
		t.Fatal("expected platform in index manifest descriptor")
	}
	if p.Architecture != "arm64" {
		t.Errorf("platform.architecture = %q, want arm64", p.Architecture)
	}
	if p.OS != "linux" {
		t.Errorf("platform.os = %q, want linux", p.OS)
	}
}

func TestWriteTarInvalidPlatform(t *testing.T) {
	img := New(Platform{OS: "linux"}) // missing Architecture
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, ""); err == nil {
		t.Error("expected error for invalid platform")
	}
}

func TestWriteTarBlobDirBeforeBlobs(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.addEntry(File{Path: "usr/local/bin/app", Type: FileRegular, Data: []byte("binary"), Mode: 0755})
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "latest"); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	var names []string
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}

	dirIdx := -1
	for i, name := range names {
		if name == "blobs/sha256/" {
			dirIdx = i
			break
		}
	}
	if dirIdx == -1 {
		t.Fatal("blobs/sha256/ directory entry not found in tar")
	}
	for i, name := range names {
		if strings.HasPrefix(name, "blobs/sha256/") && name != "blobs/sha256/" {
			if i < dirIdx {
				t.Errorf("blob entry %q (index %d) appears before blobs/sha256/ dir (index %d)", name, i, dirIdx)
			}
		}
	}
}
