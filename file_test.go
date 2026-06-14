package ocipack

import (
	"testing"
)

func TestNormalizePathValid(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"/etc/ssl/certs/ca-certificates.crt", "etc/ssl/certs/ca-certificates.crt"},
		{"/usr/local/lib/data", "usr/local/lib/data"},
		{"/tmp", "tmp"},
		{"/foo/bar", "foo/bar"},
		{"//foo//bar", "foo/bar"},      // double slashes collapsed
		{"/foo/./bar", "foo/bar"},      // dot segment removed
		{"/etc/../etc/ssl", "etc/ssl"}, // dot-dot resolved for absolute paths
	}
	for _, c := range cases {
		got, err := normalizePath(c.input)
		if err != nil {
			t.Errorf("normalizePath(%q) unexpected error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizePath(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestNormalizePathInvalid(t *testing.T) {
	cases := []string{
		"",          // empty
		".",         // current dir
		"/",         // root resolves to empty
		"../../etc", // relative upward traversal
		"../foo",    // relative upward
		"foo/../..", // resolves to ".." after clean
	}
	for _, c := range cases {
		_, err := normalizePath(c)
		if err == nil {
			t.Errorf("normalizePath(%q) expected error, got nil", c)
		}
	}
}
