package ocipack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"sort"
	"strings"
	"time"
)

var epoch = time.Unix(0, 0)

type layerResult struct {
	Compressed []byte
	Digest     string // sha256 of compressed bytes (manifest descriptor)
	DiffID     string // sha256 of uncompressed tar bytes (config diff_ids)
	Size       int64
}

// buildLayer produces a deterministic gzip-compressed OCI layer from files.
// Files are sorted by path before writing. File.Path must already be normalized
// (no leading slash). Zero ModTime is replaced with the Unix epoch.
func buildLayer(files []File) (*layerResult, error) {
	sorted := make([]File, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	var rawBuf bytes.Buffer
	tw := tar.NewWriter(&rawBuf)
	for _, f := range sorted {
		modTime := f.ModTime
		if modTime.IsZero() {
			modTime = epoch
		}
		hdr := &tar.Header{
			Uid:     f.UID,
			Gid:     f.GID,
			ModTime: modTime,
			Mode:    f.Mode,
		}
		switch f.Type {
		case FileDirectory:
			name := f.Path
			if !strings.HasSuffix(name, "/") {
				name += "/"
			}
			hdr.Name = name
			hdr.Typeflag = tar.TypeDir
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
		case FileSymlink:
			hdr.Name = f.Path
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = f.Linkname
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
		case FileRegular:
			hdr.Name = f.Path
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(f.Data))
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
			if _, err := tw.Write(f.Data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}

	rawBytes := rawBuf.Bytes()
	diffID := digestSHA256(rawBytes)

	var compBuf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&compBuf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	gw.Header.ModTime = epoch
	if _, err := gw.Write(rawBytes); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	compBytes := compBuf.Bytes()
	return &layerResult{
		Compressed: compBytes,
		Digest:     digestSHA256(compBytes),
		DiffID:     diffID,
		Size:       int64(len(compBytes)),
	}, nil
}
