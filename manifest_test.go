package ocipack

import (
	"bytes"
	"strings"
	"testing"
)

func TestManifestLayersEmptySlice(t *testing.T) {
	m := manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypeImageManifest,
		Config: descriptor{
			MediaType: MediaTypeImageConfig,
			Digest:    "sha256:abc",
			Size:      3,
		},
		Layers: nil,
	}
	b, err := marshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"layers":[]`) {
		t.Errorf("expected layers:[] in JSON, got: %s", b)
	}
	if strings.Contains(string(b), `"layers":null`) {
		t.Errorf("layers must not serialize as null, got: %s", b)
	}
}

func TestManifestSchemaVersion(t *testing.T) {
	m := manifest{
		SchemaVersion: 2,
		Config: descriptor{
			MediaType: MediaTypeImageConfig,
			Digest:    "sha256:abc",
			Size:      3,
		},
		Layers: []descriptor{},
	}
	b, err := marshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schemaVersion":2`) {
		t.Errorf("expected schemaVersion:2 in JSON, got: %s", b)
	}
}

func TestManifestDeterministic(t *testing.T) {
	m := manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypeImageManifest,
		Config: descriptor{
			MediaType: MediaTypeImageConfig,
			Digest:    "sha256:deadbeef",
			Size:      42,
		},
		Layers: []descriptor{},
	}
	b1, err := marshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := marshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("marshalManifest not deterministic:\n  %s\n  %s", b1, b2)
	}
}

func TestIndexManifestsEmptySlice(t *testing.T) {
	idx := index{
		SchemaVersion: 2,
		MediaType:     MediaTypeImageIndex,
		Manifests:     nil,
	}
	b, err := marshalIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"manifests":[]`) {
		t.Errorf("expected manifests:[] in JSON, got: %s", b)
	}
}

func TestIndexSchemaVersion(t *testing.T) {
	idx := index{
		SchemaVersion: 2,
		MediaType:     MediaTypeImageIndex,
		Manifests:     []descriptor{},
	}
	b, err := marshalIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schemaVersion":2`) {
		t.Errorf("expected schemaVersion:2 in JSON, got: %s", b)
	}
}

func TestDescriptorPlatformOmittedWhenNil(t *testing.T) {
	d := descriptor{
		MediaType: MediaTypeImageManifest,
		Digest:    "sha256:abc",
		Size:      3,
	}
	m := manifest{
		SchemaVersion: 2,
		Config:        d,
		Layers:        []descriptor{},
	}
	b, err := marshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"platform"`) {
		t.Errorf("nil platform should be omitted, got: %s", b)
	}
}
