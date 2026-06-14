package ocipack

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// FileType distinguishes file, directory, and symlink entries.
type FileType int

const (
	FileRegular FileType = iota
	FileDirectory
	FileSymlink
)

// File is a single entry to be written into the image layer.
// Path is the absolute container path (e.g. "/etc/ssl/certs/ca-certificates.crt").
// It is normalized to a relative tar path when the layer is built.
type File struct {
	Path     string
	Mode     int64
	UID      int
	GID      int
	ModTime  time.Time // zero value → Unix epoch used when writing tar
	Data     []byte
	Linkname string
	Type     FileType
}

// normalizePath converts a container path to a clean relative tar path.
// Strips a leading slash and applies path.Clean. Returns an error if the
// result is empty, ".", or contains ".." segments (possible for relative inputs).
func normalizePath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	cleaned := path.Clean(p)
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || rel == "." {
		return "", fmt.Errorf("invalid path %q: resolves to root or current directory", p)
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("invalid path %q: contains ..", p)
		}
	}
	return rel, nil
}
