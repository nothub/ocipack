package ocipack

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigDiffIDsEmptySlice(t *testing.T) {
	cfg := imageConfig{
		Architecture: "amd64",
		OS:           "linux",
		RootFS:       rootFS{Type: "layers", DiffIDs: nil},
	}
	b, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"diff_ids":[]`) {
		t.Errorf("expected diff_ids:[] in JSON, got: %s", b)
	}
	if strings.Contains(string(b), `"diff_ids":null`) {
		t.Errorf("diff_ids must not serialize as null, got: %s", b)
	}
}

func TestConfigRootFSType(t *testing.T) {
	cfg := imageConfig{
		Architecture: "amd64",
		OS:           "linux",
		RootFS:       rootFS{Type: "layers"},
	}
	b, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	var rfs map[string]json.RawMessage
	if err := json.Unmarshal(out["rootfs"], &rfs); err != nil {
		t.Fatal(err)
	}
	var typ string
	if err := json.Unmarshal(rfs["type"], &typ); err != nil {
		t.Fatal(err)
	}
	if typ != "layers" {
		t.Errorf("rootfs.type = %q, want %q", typ, "layers")
	}
}

func TestConfigRuntimeConfigCapitalizedKeys(t *testing.T) {
	cfg := imageConfig{
		Architecture: "amd64",
		OS:           "linux",
		Config: runtimeConfig{
			Env:        []string{"FOO=bar"},
			Entrypoint: []string{"/bin/sh"},
			Cmd:        []string{"-c", "echo hello"},
			Labels:     map[string]string{"version": "1"},
		},
		RootFS: rootFS{Type: "layers"},
	}
	b, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"Env"`, `"Entrypoint"`, `"Cmd"`, `"Labels"`} {
		if !strings.Contains(s, key) {
			t.Errorf("expected capitalized key %s in JSON, got: %s", key, s)
		}
	}
	for _, key := range []string{`"env"`, `"entrypoint"`, `"cmd"`, `"labels"`} {
		if strings.Contains(s, key) {
			t.Errorf("unexpected lowercase key %s in JSON, got: %s", key, s)
		}
	}
}

func TestConfigDeterministic(t *testing.T) {
	cfg := imageConfig{
		Architecture: "amd64",
		OS:           "linux",
		Config: runtimeConfig{
			Env:    []string{"A=1", "B=2"},
			Labels: map[string]string{"k": "v"},
		},
		RootFS: rootFS{Type: "layers"},
	}
	b1, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("marshalConfig not deterministic:\n  %s\n  %s", b1, b2)
	}
}

func TestConfigOmitsCreatedWhenZero(t *testing.T) {
	cfg := imageConfig{
		Architecture: "amd64",
		OS:           "linux",
		RootFS:       rootFS{Type: "layers"},
	}
	b, err := marshalConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"created"`) {
		t.Errorf("zero Created should be omitted, got: %s", b)
	}
}
