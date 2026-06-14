package ocipack

import (
	"bytes"
	"os"
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
