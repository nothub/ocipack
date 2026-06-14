package ocipack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTempContent(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ocipack-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestNew(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if img == nil {
		t.Fatal("New returned nil")
	}
}

func TestBuildEmptyImage(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	result, err := img.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if digestSHA256(result.Config) != result.ConfigDigest {
		t.Error("ConfigDigest does not match sha256 of Config bytes")
	}
	if digestSHA256(result.Manifest) != result.ManifestDigest {
		t.Error("ManifestDigest does not match sha256 of Manifest bytes")
	}
	if result.Layer != nil {
		t.Error("expected nil Layer for zero-layer image")
	}
	if result.LayerDigest != "" {
		t.Error("expected empty LayerDigest for zero-layer image")
	}
	if len(result.DiffIDs) != 0 {
		t.Errorf("expected empty DiffIDs, got %v", result.DiffIDs)
	}
	if !strings.Contains(string(result.Config), `"diff_ids":[]`) {
		t.Errorf("config missing diff_ids:[], got: %s", result.Config)
	}
	if !strings.Contains(string(result.Manifest), `"layers":[]`) {
		t.Errorf("manifest missing layers:[], got: %s", result.Manifest)
	}
}

func TestBuildValidationOSRequired(t *testing.T) {
	img := New(Platform{Architecture: "amd64"})
	_, err := img.Build()
	if err == nil {
		t.Error("expected error when OS is empty")
	}
}

func TestBuildValidationArchRequired(t *testing.T) {
	img := New(Platform{OS: "linux"})
	_, err := img.Build()
	if err == nil {
		t.Error("expected error when Architecture is empty")
	}
}

func TestBuildPlatformInConfig(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "arm64"})
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Config), `"arm64"`) {
		t.Errorf("expected arm64 in config, got: %s", result.Config)
	}
	if !strings.Contains(string(result.Config), `"linux"`) {
		t.Errorf("expected linux in config, got: %s", result.Config)
	}
}

func TestImageWithFiles(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddFile("/usr/local/bin/app", writeTempContent(t, "binary content"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if result.Layer == nil {
		t.Error("expected non-nil Layer")
	}
	if result.LayerDigest == "" {
		t.Error("expected non-empty LayerDigest")
	}
	if len(result.DiffIDs) == 0 {
		t.Error("expected non-empty DiffIDs")
	}
}

func TestImageWithDir(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddDir("/tmp", 01777); err != nil {
		t.Fatal(err)
	}
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if result.Layer == nil {
		t.Error("expected non-nil Layer for image with directory")
	}
}

func TestImageWithSymlink(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddSymlink("/etc/mtab", "/proc/mounts"); err != nil {
		t.Fatal(err)
	}
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if result.Layer == nil {
		t.Error("expected non-nil Layer for image with symlink")
	}
}

func TestDigestDistinction(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddFile("/usr/local/bin/app", writeTempContent(t, "data"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	raw := decompressLayer(t, result.Layer)
	if result.DiffIDs[0] != digestSHA256(raw) {
		t.Errorf("DiffIDs[0] = %q, want sha256(uncompressed layer)", result.DiffIDs[0])
	}
	if result.LayerDigest != digestSHA256(result.Layer) {
		t.Error("LayerDigest != sha256(compressed layer)")
	}
	if result.DiffIDs[0] == result.LayerDigest {
		t.Error("DiffID and LayerDigest must differ")
	}
	if !strings.Contains(string(result.Config), result.DiffIDs[0]) {
		t.Errorf("config missing DiffID: %s", result.Config)
	}
	if !strings.Contains(string(result.Manifest), result.LayerDigest) {
		t.Errorf("manifest missing LayerDigest: %s", result.Manifest)
	}
}

func TestForceEmptyLayer(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.ForceEmptyLayer()
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if result.Layer == nil {
		t.Error("ForceEmptyLayer: expected non-nil Layer")
	}
	if len(result.DiffIDs) == 0 {
		t.Error("ForceEmptyLayer: expected non-empty DiffIDs")
	}
}

func TestWriteTarIncludesLayerBlob(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddFile("/usr/local/bin/app", writeTempContent(t, "binary"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	layerHex, _ := digestHex(result.LayerDigest)

	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "latest"); err != nil {
		t.Fatal(err)
	}
	entries := readTarGz(t, out)
	if _, ok := entries["blobs/sha256/"+layerHex]; !ok {
		t.Errorf("layer blob blobs/sha256/%s not found in output tar", layerHex)
	}
}

func TestSetCreated(t *testing.T) {
	ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.SetCreated(ts)
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Config), "2024-01-15T12:00:00Z") {
		t.Errorf("expected created timestamp in config, got: %s", result.Config)
	}
}

func TestDefaultUser(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Config), `"User":"65534"`) {
		t.Errorf("expected default User 65534 in config, got: %s", result.Config)
	}
}

func TestSetUser(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.SetUser("0")
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Config), `"User":"0"`) {
		t.Errorf("expected User 0 in config, got: %s", result.Config)
	}
}

func TestSetWorkdir(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.SetWorkdir("/app")
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Config), `"WorkingDir":"/app"`) {
		t.Errorf("expected WorkingDir /app in config, got: %s", result.Config)
	}
}

func TestWorkdirOmittedWhenEmpty(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result.Config), "WorkingDir") {
		t.Errorf("expected no WorkingDir in config when not set, got: %s", result.Config)
	}
}

func TestBuilderSetters(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.AddEnv("PATH", "/usr/local/bin")
	img.SetEntrypoint("/app/server")
	img.SetCmd("--port=8080")
	img.SetLabel("version", "1.0")

	result, err := img.Build()
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(result.Config)
	if !strings.Contains(cfg, "PATH=/usr/local/bin") {
		t.Errorf("expected env in config, got: %s", cfg)
	}
	if !strings.Contains(cfg, "/app/server") {
		t.Errorf("expected entrypoint in config, got: %s", cfg)
	}
	if !strings.Contains(cfg, "--port=8080") {
		t.Errorf("expected cmd in config, got: %s", cfg)
	}
	if !strings.Contains(cfg, `"version":"1.0"`) {
		t.Errorf("expected label in config, got: %s", cfg)
	}
}
