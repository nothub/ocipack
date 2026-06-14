package ocipack

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReproducibility(t *testing.T) {
	t.Run("zero-layer", func(t *testing.T) {
		build := func() *BuildResult {
			t.Helper()
			img := New(Platform{OS: "linux", Architecture: "amd64"})
			img.AddEnv("PATH", "/usr/local/bin:/usr/bin")
			img.SetEntrypoint("/app/server")
			img.SetCmd("--port=8080")
			img.SetLabel("org.example.version", "1.0")
			result, err := img.Build()
			if err != nil {
				t.Fatal(err)
			}
			return result
		}

		r1, r2 := build(), build()

		if !bytes.Equal(r1.Config, r2.Config) {
			t.Error("config bytes differ between builds")
		}
		if !bytes.Equal(r1.Manifest, r2.Manifest) {
			t.Error("manifest bytes differ between builds")
		}
		if r1.ConfigDigest != r2.ConfigDigest {
			t.Error("config digest differs between builds")
		}
		if r1.ManifestDigest != r2.ManifestDigest {
			t.Error("manifest digest differs between builds")
		}
		if digestSHA256(r1.Config) != r1.ConfigDigest {
			t.Error("config digest does not match sha256(configBytes)")
		}
		if digestSHA256(r1.Manifest) != r1.ManifestDigest {
			t.Error("manifest digest does not match sha256(manifestBytes)")
		}
	})

	t.Run("single-layer", func(t *testing.T) {
		build := func() *BuildResult {
			t.Helper()
			img := New(Platform{OS: "linux", Architecture: "amd64"})
			img.AddEnv("PATH", "/usr/local/bin:/usr/bin")
			img.SetEntrypoint("/app/server")
			if err := img.AddFile("/usr/local/bin/app", writeTempContent(t, "binary content"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := img.AddDir("/tmp", 01777); err != nil {
				t.Fatal(err)
			}
			if err := img.AddSymlink("/etc/mtab", "/proc/mounts"); err != nil {
				t.Fatal(err)
			}
			result, err := img.Build()
			if err != nil {
				t.Fatal(err)
			}
			return result
		}

		r1, r2 := build(), build()

		if !bytes.Equal(r1.Config, r2.Config) {
			t.Error("config bytes differ between builds")
		}
		if !bytes.Equal(r1.Manifest, r2.Manifest) {
			t.Error("manifest bytes differ between builds")
		}
		if !bytes.Equal(r1.Layer, r2.Layer) {
			t.Error("layer bytes differ between builds")
		}
		if r1.ConfigDigest != r2.ConfigDigest {
			t.Error("config digest differs between builds")
		}
		if r1.ManifestDigest != r2.ManifestDigest {
			t.Error("manifest digest differs between builds")
		}
		if r1.LayerDigest != r2.LayerDigest {
			t.Error("layer digest differs between builds")
		}
		if r1.DiffIDs[0] != r2.DiffIDs[0] {
			t.Error("diffID differs between builds")
		}
		if digestSHA256(r1.Config) != r1.ConfigDigest {
			t.Error("config digest does not match sha256(configBytes)")
		}
		if digestSHA256(r1.Manifest) != r1.ManifestDigest {
			t.Error("manifest digest does not match sha256(manifestBytes)")
		}
		if digestSHA256(r1.Layer) != r1.LayerDigest {
			t.Error("layer digest does not match sha256(layer)")
		}
	})

	t.Run("write-tar", func(t *testing.T) {
		buildTar := func(dir string) []byte {
			t.Helper()
			img := New(Platform{OS: "linux", Architecture: "amd64"})
			if err := img.AddFile("/usr/local/bin/app", writeTempContent(t, "binary"), 0755); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, "image.tar.gz")
			if err := img.WriteTar(out, "latest"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}

		a := buildTar(t.TempDir())
		b := buildTar(t.TempDir())

		if !bytes.Equal(a, b) {
			t.Errorf("WriteTar output differs: len(a)=%d len(b)=%d", len(a), len(b))
		}
	})
}
