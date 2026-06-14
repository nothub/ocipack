package ocipack

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTar builds the image and writes it as a gzip-compressed OCI image
// layout tarball (oci-archive format) to the given path.
// refName is written as org.opencontainers.image.ref.name in index.json;
// pass "" to omit the annotation.
func (img *Image) WriteTar(path, refName string) error {
	result, err := img.Build()
	if err != nil {
		return err
	}

	indexBytes, err := buildIndexJSON(result, img.platform, refName)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".ocipack-*.tar.gz")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if err := writeOCIArchive(tmp, result, indexBytes); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, path)
}

func buildIndexJSON(result *BuildResult, p Platform, refName string) ([]byte, error) {
	desc := descriptor{
		MediaType: MediaTypeImageManifest,
		Digest:    result.ManifestDigest,
		Size:      result.ManifestSize,
		Platform: &platform{
			Architecture: p.Architecture,
			OS:           p.OS,
			Variant:      p.Variant,
		},
	}
	if refName != "" {
		desc.Annotations = map[string]string{
			"org.opencontainers.image.ref.name": refName,
		}
	}

	idx := index{
		SchemaVersion: 2,
		MediaType:     MediaTypeImageIndex,
		Manifests:     []descriptor{desc},
	}

	return marshalIndex(idx)
}

func writeOCIArchive(f *os.File, result *BuildResult, indexBytes []byte) error {
	gw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	gw.ModTime = epoch

	tw := tar.NewWriter(gw)

	// Use the current user's uid/gid for outer archive entries so that
	// container tools running as non-root (e.g. skopeo) can extract the
	// archive without needing to chown to uid 0.
	uid := os.Getuid()
	gid := os.Getgid()

	type entry struct {
		name string
		data []byte
	}

	writeDir := func(name string) error {
		return tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeDir,
			Mode:     0755,
			Uid:      uid,
			Gid:      gid,
			ModTime:  epoch,
		})
	}

	if err := writeDir("blobs/"); err != nil {
		return err
	}
	if err := writeDir("blobs/sha256/"); err != nil {
		return err
	}

	entries := []entry{
		{"oci-layout", []byte(`{"imageLayoutVersion":"1.0.0"}`)},
		{"index.json", indexBytes},
	}

	configHex, err := digestHex(result.ConfigDigest)
	if err != nil {
		return fmt.Errorf("internal: %w", err)
	}
	entries = append(entries, entry{"blobs/sha256/" + configHex, result.Config})

	manifestHex, err := digestHex(result.ManifestDigest)
	if err != nil {
		return fmt.Errorf("internal: %w", err)
	}
	entries = append(entries, entry{"blobs/sha256/" + manifestHex, result.Manifest})

	if len(result.Layer) > 0 {
		layerHex, err := digestHex(result.LayerDigest)
		if err != nil {
			return fmt.Errorf("internal: %w", err)
		}
		entries = append(entries, entry{"blobs/sha256/" + layerHex, result.Layer})
	}

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     0644,
			Uid:      uid,
			Gid:      gid,
			Size:     int64(len(e.data)),
			Typeflag: tar.TypeReg,
			ModTime:  epoch,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(e.data); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}
