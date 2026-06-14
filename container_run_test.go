//go:build integration

package ocipack

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContainerRun(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Fatal("podman not found in PATH")
	}

	var hello string
	switch runtime.GOARCH {
	case "amd64":
		hello = helloAMD64
	case "arm64":
		hello = helloARM64
	default:
		t.Fatalf("no test binary for arch %s", runtime.GOARCH)
	}

	out := filepath.Join(t.TempDir(), "image.tar.gz")
	tag := "localhost/ocipack-test:latest"
	img := New(Platform{OS: "linux"})
	if err := img.AddBinary(hello); err != nil {
		t.Fatal(err)
	}
	if err := img.WriteTar(out, tag); err != nil {
		t.Fatal(err)
	}

	if b, err := exec.Command("podman", "load", "-i", out).CombinedOutput(); err != nil {
		t.Fatalf("podman load: %v\n%s", err, b)
	}
	defer exec.Command("podman", "rmi", "--force", tag).Run()

	b, err := exec.Command("podman", "run", "--rm", tag).CombinedOutput()
	if err != nil {
		t.Fatalf("podman run: %v\n%s", err, b)
	}
	if !strings.Contains(string(b), "👋") {
		t.Errorf("expected 👋 in output, got: %q", string(b))
	}
}
