package ocipack

import (
	"encoding/json"
	"time"
)

type imageConfig struct {
	Created      *time.Time    `json:"created,omitempty"`
	Architecture string        `json:"architecture"`
	OS           string        `json:"os"`
	Variant      string        `json:"variant,omitempty"`
	Config       runtimeConfig `json:"config"`
	RootFS       rootFS        `json:"rootfs"`
}

// runtimeConfig uses capitalized JSON keys to match the Docker-inherited OCI image config convention.
type runtimeConfig struct {
	User       string            `json:"User,omitempty"`
	WorkingDir string            `json:"WorkingDir,omitempty"`
	Env        []string          `json:"Env,omitempty"`
	Entrypoint []string          `json:"Entrypoint,omitempty"`
	Cmd        []string          `json:"Cmd,omitempty"`
	Labels     map[string]string `json:"Labels,omitempty"`
}

type rootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

func marshalConfig(cfg imageConfig) ([]byte, error) {
	if cfg.RootFS.DiffIDs == nil {
		cfg.RootFS.DiffIDs = []string{}
	}
	return json.Marshal(cfg)
}
