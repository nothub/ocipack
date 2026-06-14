package ocipack

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	helloAMD64 string
	helloARM64 string
)

// helloSrc is the source for the hello-world test fixture binary.
const helloSrc = "package main; import \"fmt\"; func main() { fmt.Println(\"👋\") }"

// TestMain builds the test fixtures (hello cross-compiled for linux/amd64 and
// linux/arm64 from helloSrc, and the CLI for the current platform) before
// running the package test suite.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ocipack-test-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	helloAMD64 = buildSrc(dir, "hello-amd64", "linux", "amd64", helloSrc)
	helloARM64 = buildSrc(dir, "hello-arm64", "linux", "arm64", helloSrc)

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

func TestAddBinaryAMD64(t *testing.T) {
	img := New(Platform{OS: "linux"})
	if err := img.AddBinary(helloAMD64); err != nil {
		t.Fatal(err)
	}
	if img.platform.Architecture != "amd64" {
		t.Errorf("arch = %q, want amd64", img.platform.Architecture)
	}
	if len(img.entrypoint) != 1 || img.entrypoint[0] != "/hello-amd64" {
		t.Errorf("entrypoint = %v, want [/hello-amd64]", img.entrypoint)
	}
	if len(img.files) == 0 {
		t.Error("expected file to be added")
	}
}

func TestAddBinaryARM64(t *testing.T) {
	img := New(Platform{OS: "linux"})
	if err := img.AddBinary(helloARM64); err != nil {
		t.Fatal(err)
	}
	if img.platform.Architecture != "arm64" {
		t.Errorf("arch = %q, want arm64", img.platform.Architecture)
	}
	if len(img.entrypoint) != 1 || img.entrypoint[0] != "/hello-arm64" {
		t.Errorf("entrypoint = %v, want [/hello-arm64]", img.entrypoint)
	}
}

func TestAddBinaryDynamicallyLinked(t *testing.T) {
	img := New(Platform{OS: "linux"})
	err := img.AddBinary(dynELF(t))
	if !errors.Is(err, ErrDynamicallyLinked) {
		t.Fatalf("expected ErrDynamicallyLinked, got %v", err)
	}
	// Image must still be fully assembled so callers can warn and continue.
	if img.platform.Architecture != "amd64" {
		t.Errorf("arch = %q, want amd64", img.platform.Architecture)
	}
	if len(img.entrypoint) == 0 {
		t.Error("entrypoint not set")
	}
	if len(img.files) == 0 {
		t.Error("file not added")
	}
}

// dynELF writes a minimal x86-64 ELF with a PT_INTERP segment to a temp file.
// The binary does not need to be runnable; it exists only to trigger
// dynamic-linker detection.
func dynELF(t *testing.T) string {
	t.Helper()
	const (
		ehdrSize = 64
		phdrSize = 56
	)
	interp := []byte("/lib64/ld-linux-x86-64.so.2\x00")
	data := make([]byte, ehdrSize+phdrSize+len(interp))
	le := binary.LittleEndian

	copy(data[0:], []byte{0x7f, 'E', 'L', 'F'})
	data[4] = 2 // ELFCLASS64
	data[5] = 1 // ELFDATA2LSB
	data[6] = 1 // EV_CURRENT

	le.PutUint16(data[16:], 2)        // ET_EXEC
	le.PutUint16(data[18:], 62)       // EM_X86_64
	le.PutUint32(data[20:], 1)        // e_version
	le.PutUint64(data[32:], ehdrSize) // e_phoff
	le.PutUint16(data[52:], ehdrSize) // e_ehsize
	le.PutUint16(data[54:], phdrSize) // e_phentsize
	le.PutUint16(data[56:], 1)        // e_phnum

	ph := data[ehdrSize:]
	le.PutUint32(ph[0:], 3)                    // PT_INTERP
	le.PutUint32(ph[4:], 4)                    // PF_R
	le.PutUint64(ph[8:], ehdrSize+phdrSize)    // p_offset
	le.PutUint64(ph[32:], uint64(len(interp))) // p_filesz
	le.PutUint64(ph[40:], uint64(len(interp))) // p_memsz
	le.PutUint64(ph[48:], 1)                   // p_align

	copy(data[ehdrSize+phdrSize:], interp)

	f := filepath.Join(t.TempDir(), "dynbin")
	if err := os.WriteFile(f, data, 0755); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestAddBinaryNonELF(t *testing.T) {
	img := New(Platform{OS: "linux"})
	if err := img.AddBinary(writeTempContent(t, "not an ELF binary")); err == nil {
		t.Error("expected error for non-ELF input")
	}
}

func TestAddBinaryNonExistent(t *testing.T) {
	img := New(Platform{OS: "linux"})
	if err := img.AddBinary("/nonexistent/binary"); err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestOCIEndToEnd(t *testing.T) {
	img := New(Platform{OS: "linux"})
	if err := img.AddBinary(helloAMD64); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "image.tar.gz")
	if err := img.WriteTar(out, "hello:latest"); err != nil {
		t.Fatal(err)
	}

	entries := readTarGz(t, out)

	// index
	var idx struct {
		Manifests []struct {
			Digest   string `json:"digest"`
			Platform struct {
				Architecture string `json:"architecture"`
				OS           string `json:"os"`
			} `json:"platform"`
			Annotations map[string]string `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(entries["index.json"], &idx); err != nil {
		t.Fatalf("parse index.json: %v", err)
	}
	if len(idx.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(idx.Manifests))
	}
	md := idx.Manifests[0]
	if md.Platform.Architecture != "amd64" {
		t.Errorf("index platform.architecture = %q, want amd64", md.Platform.Architecture)
	}
	if md.Platform.OS != "linux" {
		t.Errorf("index platform.os = %q, want linux", md.Platform.OS)
	}
	if md.Annotations["org.opencontainers.image.ref.name"] != "hello:latest" {
		t.Errorf("ref.name = %q, want hello:latest", md.Annotations["org.opencontainers.image.ref.name"])
	}

	// manifest blob
	mhex, _ := digestHex(md.Digest)
	mBlob, ok := entries["blobs/sha256/"+mhex]
	if !ok {
		t.Fatal("manifest blob missing from archive")
	}
	if digestSHA256(mBlob) != md.Digest {
		t.Error("manifest blob digest mismatch")
	}

	// manifest → config + layer
	var m struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(mBlob, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(m.Layers))
	}

	// config blob
	chex, _ := digestHex(m.Config.Digest)
	cBlob, ok := entries["blobs/sha256/"+chex]
	if !ok {
		t.Fatal("config blob missing from archive")
	}
	if digestSHA256(cBlob) != m.Config.Digest {
		t.Error("config blob digest mismatch")
	}
	var cfg struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		Config       struct {
			Entrypoint []string `json:"Entrypoint"`
		} `json:"config"`
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}
	if err := json.Unmarshal(cBlob, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Architecture != "amd64" {
		t.Errorf("config.architecture = %q, want amd64", cfg.Architecture)
	}
	if cfg.OS != "linux" {
		t.Errorf("config.os = %q, want linux", cfg.OS)
	}
	if len(cfg.Config.Entrypoint) != 1 || cfg.Config.Entrypoint[0] != "/hello-amd64" {
		t.Errorf("config.Entrypoint = %v, want [/hello-amd64]", cfg.Config.Entrypoint)
	}

	// layer blob + diffID
	lhex, _ := digestHex(m.Layers[0].Digest)
	lBlob, ok := entries["blobs/sha256/"+lhex]
	if !ok {
		t.Fatal("layer blob missing from archive")
	}
	if digestSHA256(lBlob) != m.Layers[0].Digest {
		t.Error("layer blob digest mismatch")
	}
	if cfg.RootFS.DiffIDs[0] != digestSHA256(decompressLayer(t, lBlob)) {
		t.Error("diffID does not match sha256 of uncompressed layer")
	}
}
