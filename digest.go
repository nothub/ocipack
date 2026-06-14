package ocipack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// digestSHA256 returns the OCI digest string for b: "sha256:<hex>".
func digestSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// digestHex strips the "sha256:" prefix from a digest string and returns the hex part.
func digestHex(digest string) (string, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return "", fmt.Errorf("unsupported digest algorithm in %q", digest)
	}
	return strings.TrimPrefix(digest, prefix), nil
}
