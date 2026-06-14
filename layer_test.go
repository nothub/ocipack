package ocipack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"sort"
	"testing"
)

func decompressLayer(t *testing.T, data []byte) []byte {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	out, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLayerDeterminism(t *testing.T) {
	files := []File{
		{Path: "usr/local/bin/app", Type: FileRegular, Data: []byte("hello"), Mode: 0755},
		{Path: "etc/ssl", Type: FileDirectory, Mode: 0755},
	}
	r1, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r1.Compressed, r2.Compressed) {
		t.Error("buildLayer output is not deterministic")
	}
}

func TestLayerDiffIDIsUncompressedHash(t *testing.T) {
	files := []File{
		{Path: "usr/local/bin/app", Type: FileRegular, Data: []byte("hello"), Mode: 0755},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	raw := decompressLayer(t, r.Compressed)
	if digestSHA256(raw) != r.DiffID {
		t.Errorf("DiffID = %q, want sha256(uncompressed) = %q", r.DiffID, digestSHA256(raw))
	}
}

func TestLayerDigestIsCompressedHash(t *testing.T) {
	files := []File{
		{Path: "usr/local/bin/app", Type: FileRegular, Data: []byte("hello"), Mode: 0755},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	if digestSHA256(r.Compressed) != r.Digest {
		t.Error("Digest != sha256(compressed)")
	}
}

func TestLayerDiffIDDistinctFromDigest(t *testing.T) {
	files := []File{
		{Path: "usr/local/bin/app", Type: FileRegular, Data: []byte("hello"), Mode: 0755},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	if r.DiffID == r.Digest {
		t.Error("DiffID and Digest must differ")
	}
}

func TestLayerSortedByPath(t *testing.T) {
	files := []File{
		{Path: "z/last", Type: FileRegular, Data: []byte("z"), Mode: 0644},
		{Path: "a/first", Type: FileRegular, Data: []byte("a"), Mode: 0644},
		{Path: "m/mid", Type: FileRegular, Data: []byte("m"), Mode: 0644},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	raw := decompressLayer(t, r.Compressed)
	tr := tar.NewReader(bytes.NewReader(raw))
	var names []string
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
	if !sort.StringsAreSorted(names) {
		t.Errorf("tar entries not in lexicographic order: %v", names)
	}
}

func TestLayerDirectoryEntry(t *testing.T) {
	files := []File{
		{Path: "etc/ssl", Type: FileDirectory, Mode: 0755},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	raw := decompressLayer(t, r.Compressed)
	tr := tar.NewReader(bytes.NewReader(raw))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Typeflag != tar.TypeDir {
		t.Errorf("Typeflag = %d, want TypeDir", hdr.Typeflag)
	}
	if hdr.Size != 0 {
		t.Errorf("Size = %d, want 0", hdr.Size)
	}
}

func TestLayerSymlinkEntry(t *testing.T) {
	files := []File{
		{Path: "etc/mtab", Type: FileSymlink, Linkname: "/proc/mounts"},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	raw := decompressLayer(t, r.Compressed)
	tr := tar.NewReader(bytes.NewReader(raw))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Typeflag != tar.TypeSymlink {
		t.Errorf("Typeflag = %d, want TypeSymlink", hdr.Typeflag)
	}
	if hdr.Linkname != "/proc/mounts" {
		t.Errorf("Linkname = %q, want /proc/mounts", hdr.Linkname)
	}
}

func TestLayerEmptyInput(t *testing.T) {
	r, err := buildLayer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Compressed == nil {
		t.Fatal("expected non-nil compressed output for empty layer")
	}
	raw := decompressLayer(t, r.Compressed)
	tr := tar.NewReader(bytes.NewReader(raw))
	_, err = tr.Next()
	if err != io.EOF {
		t.Errorf("expected empty tar (EOF immediately), got err=%v", err)
	}
}

func TestLayerTarHeaderDefaults(t *testing.T) {
	files := []File{
		{Path: "usr/bin/app", Type: FileRegular, Data: []byte("data"), Mode: 0755},
	}
	r, err := buildLayer(files)
	if err != nil {
		t.Fatal(err)
	}
	raw := decompressLayer(t, r.Compressed)
	tr := tar.NewReader(bytes.NewReader(raw))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Uid != 0 {
		t.Errorf("Uid = %d, want 0", hdr.Uid)
	}
	if hdr.Gid != 0 {
		t.Errorf("Gid = %d, want 0", hdr.Gid)
	}
	if hdr.Uname != "" {
		t.Errorf("Uname = %q, want empty", hdr.Uname)
	}
	if hdr.Gname != "" {
		t.Errorf("Gname = %q, want empty", hdr.Gname)
	}
	if hdr.ModTime.Unix() != 0 {
		t.Errorf("ModTime.Unix() = %d, want 0 (epoch)", hdr.ModTime.Unix())
	}
}
