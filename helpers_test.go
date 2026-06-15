package ocipack

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestHelpersCABundle(t *testing.T) {
	caData := []byte("--- FAKE CA BUNDLE ---")
	f, err := os.CreateTemp(t.TempDir(), "ca-*.crt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(caData); err != nil {
		t.Fatal(err)
	}
	f.Close()

	orig := caBundlePaths
	caBundlePaths = []string{f.Name()}
	t.Cleanup(func() { caBundlePaths = orig })

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddCABundle(""); err != nil {
		t.Fatal(err)
	}

	if len(img.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(img.files))
	}
	got := img.files[0]
	if got.Path != "etc/ssl/certs/ca-certificates.crt" {
		t.Errorf("path = %q, want etc/ssl/certs/ca-certificates.crt", got.Path)
	}
	if got.Mode != 0644 {
		t.Errorf("mode = %04o, want 0644", got.Mode)
	}
	if got.UID != 0 || got.GID != 0 {
		t.Errorf("uid/gid = %d/%d, want 0/0", got.UID, got.GID)
	}
	if !bytes.Equal(got.Data, caData) {
		t.Error("data mismatch: file content not preserved")
	}
}

func TestHelpersCABundleError(t *testing.T) {
	orig := caBundlePaths
	caBundlePaths = []string{"/nonexistent/ca.crt"}
	t.Cleanup(func() { caBundlePaths = orig })

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddCABundle(""); err == nil {
		t.Error("expected error when no CA bundle found on host")
	}
}

func TestHelpersCABundleSSLCertFile(t *testing.T) {
	caData := []byte("--- FAKE CA BUNDLE ---")
	f, err := os.CreateTemp(t.TempDir(), "ca-*.crt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(caData); err != nil {
		t.Fatal(err)
	}
	f.Close()

	t.Setenv("SSL_CERT_FILE", f.Name())

	orig := caBundlePaths
	caBundlePaths = []string{"/nonexistent/ca.crt"}
	t.Cleanup(func() { caBundlePaths = orig })

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddCABundle(""); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.files[0].Data, caData) {
		t.Error("data mismatch: SSL_CERT_FILE not used")
	}
}

func TestHelpersCABundleExplicitPath(t *testing.T) {
	caData := []byte("--- FAKE CA BUNDLE ---")
	f, err := os.CreateTemp(t.TempDir(), "ca-*.crt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(caData); err != nil {
		t.Fatal(err)
	}
	f.Close()

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddCABundle(f.Name()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(img.files[0].Data, caData) {
		t.Error("data mismatch: file content not preserved")
	}
}

func TestHelpersCABundleExplicitPathNotFound(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddCABundle("/nonexistent/ca.crt"); err == nil {
		t.Error("expected error for non-existent explicit path")
	}
}

func TestHelpersTZDataExplicitPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "America"), 0755); err != nil {
		t.Fatal(err)
	}
	zoneData := []byte("TZif fake data")
	if err := os.WriteFile(filepath.Join(dir, "America", "New_York"), zoneData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UTC"), []byte("UTC data"), 0644); err != nil {
		t.Fatal(err)
	}

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddTZData(dir); err != nil {
		t.Fatal(err)
	}

	byPath := map[string]File{}
	for _, f := range img.files {
		byPath[f.Path] = f
	}

	if f, ok := byPath["usr/share/zoneinfo/America"]; !ok {
		t.Error("missing usr/share/zoneinfo/America directory")
	} else if f.Type != FileDirectory || f.Mode != 0755 {
		t.Errorf("America: type=%v mode=%04o, want dir 0755", f.Type, f.Mode)
	}

	if f, ok := byPath["usr/share/zoneinfo/America/New_York"]; !ok {
		t.Error("missing usr/share/zoneinfo/America/New_York")
	} else if f.Type != FileRegular || f.Mode != 0644 || !bytes.Equal(f.Data, zoneData) {
		t.Errorf("New_York: type=%v mode=%04o data mismatch", f.Type, f.Mode)
	}

	if _, ok := byPath["usr/share/zoneinfo/UTC"]; !ok {
		t.Error("missing usr/share/zoneinfo/UTC")
	}
}

func TestHelpersTZDataSymlinksSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "UTC"), []byte("UTC data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "UTC"), filepath.Join(dir, "posix")); err != nil {
		t.Fatal(err)
	}

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddTZData(dir); err != nil {
		t.Fatal(err)
	}

	for _, f := range img.files {
		if f.Path == "usr/share/zoneinfo/posix" {
			t.Error("symlink 'posix' should not appear in img.files")
		}
	}
}

func TestHelpersTZDataAutoDetect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "UTC"), []byte("UTC data"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ZONEINFO", dir)
	orig := tzdataPaths
	tzdataPaths = []string{"/nonexistent"}
	t.Cleanup(func() { tzdataPaths = orig })

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddTZData(""); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range img.files {
		if f.Path == "usr/share/zoneinfo/UTC" {
			found = true
		}
	}
	if !found {
		t.Error("expected usr/share/zoneinfo/UTC via $ZONEINFO auto-detect")
	}
}

func TestHelpersTZDataMissingDir(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddTZData("/nonexistent/zoneinfo"); err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestHelpersTZDataNoSourceFound(t *testing.T) {
	t.Setenv("ZONEINFO", "")
	orig := tzdataPaths
	tzdataPaths = []string{"/nonexistent/zoneinfo"}
	t.Cleanup(func() { tzdataPaths = orig })

	img := New(Platform{OS: "linux", Architecture: "amd64"})
	if err := img.AddTZData(""); err == nil {
		t.Error("expected error when no zoneinfo directory found")
	}
}

func TestHelpersTmp(t *testing.T) {
	img := New(Platform{OS: "linux", Architecture: "amd64"})
	img.AddTmp()

	if len(img.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(img.files))
	}
	got := img.files[0]
	if got.Path != "tmp" {
		t.Errorf("path = %q, want tmp", got.Path)
	}
	if got.Type != FileDirectory {
		t.Errorf("type = %v, want FileDirectory", got.Type)
	}
	if got.Mode != 01777 {
		t.Errorf("mode = %04o, want 01777", got.Mode)
	}
}
