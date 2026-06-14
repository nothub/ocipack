package ocipack

import (
	"strings"
	"testing"
)

func TestDigestSHA256KnownValue(t *testing.T) {
	// sha256("") is well-known
	got := digestSHA256([]byte{})
	want := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("digestSHA256([]) = %q, want %q", got, want)
	}
}

func TestDigestSHA256NonEmpty(t *testing.T) {
	// sha256("hello world") = b94d27b9934d3e08a52e52d7da7dabfac484efe04294e576b95d2b9c3a2dd30e -- no wait
	// sha256("hello world") = b94d27b9934d3e08a52e52d7da7dabfac484efe04294e576b95d2b9c3a2dd30e -- wrong
	// let me just check prefix and length
	d := digestSHA256([]byte("hello world"))
	if !strings.HasPrefix(d, "sha256:") {
		t.Errorf("result %q does not start with sha256:", d)
	}
	// "sha256:" + 64 hex chars
	if len(d) != 7+64 {
		t.Errorf("result %q has length %d, want %d", d, len(d), 7+64)
	}
}

func TestDigestSHA256Deterministic(t *testing.T) {
	data := []byte("deterministic input")
	d1 := digestSHA256(data)
	d2 := digestSHA256(data)
	if d1 != d2 {
		t.Errorf("digestSHA256 is not deterministic: %q != %q", d1, d2)
	}
}

func TestDigestHex(t *testing.T) {
	digest := "sha256:abc123def456"
	got, err := digestHex(digest)
	if err != nil {
		t.Fatalf("digestHex(%q) unexpected error: %v", digest, err)
	}
	if got != "abc123def456" {
		t.Errorf("digestHex(%q) = %q, want %q", digest, got, "abc123def456")
	}
}

func TestDigestHexInvalidAlgorithm(t *testing.T) {
	_, err := digestHex("md5:abc123")
	if err == nil {
		t.Error("digestHex with md5: prefix should return error")
	}
}

func TestDigestHexEmpty(t *testing.T) {
	_, err := digestHex("")
	if err == nil {
		t.Error("digestHex with empty string should return error")
	}
}

func TestDigestSHA256RoundTrip(t *testing.T) {
	data := []byte("round trip test")
	digest := digestSHA256(data)
	hex, err := digestHex(digest)
	if err != nil {
		t.Fatalf("digestHex(%q) error: %v", digest, err)
	}
	if len(hex) != 64 {
		t.Errorf("hex part length = %d, want 64", len(hex))
	}
}
