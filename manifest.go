package ocipack

import "encoding/json"

type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    *platform         `json:"platform,omitempty"`
}

type platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType,omitempty"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}

type index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType,omitempty"`
	Manifests     []descriptor `json:"manifests"`
}

func marshalManifest(m manifest) ([]byte, error) {
	if m.Layers == nil {
		m.Layers = []descriptor{}
	}
	return json.Marshal(m)
}

func marshalIndex(idx index) ([]byte, error) {
	if idx.Manifests == nil {
		idx.Manifests = []descriptor{}
	}
	return json.Marshal(idx)
}
