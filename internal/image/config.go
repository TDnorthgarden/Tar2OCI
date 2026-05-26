package image

import (
	"encoding/json"
	"time"
)

// BuildConfig contains the OCI image configuration
type BuildConfig struct {
	Entrypoint   []string          `json:"Entrypoint,omitempty"`
	Cmd          []string          `json:"Cmd,omitempty"`
	Env          []string          `json:"Env,omitempty"`
	WorkingDir   string            `json:"WorkingDir,omitempty"`
	User         string            `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Labels       map[string]string `json:"Labels,omitempty"`
	StopSignal   string            `json:"StopSignal,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	ArgsEscaped  bool              `json:"ArgsEscaped,omitempty"`
}

// ImageConfig represents the full OCI image configuration
type ImageConfig struct {
	Architecture string      `json:"architecture"`
	OS           string      `json:"os"`
	Config       BuildConfig `json:"config"`
	RootFS       RootFS      `json:"rootfs"`
	History      []History   `json:"history,omitempty"`
	Created      *time.Time  `json:"created,omitempty"`
	Author       string      `json:"author,omitempty"`
}

// RootFS describes the root filesystem
type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

// History describes a layer history entry
type History struct {
	Created    *time.Time `json:"created,omitempty"`
	CreatedBy  string     `json:"created_by,omitempty"`
	Author     string     `json:"author,omitempty"`
	Comment    string     `json:"comment,omitempty"`
	EmptyLayer bool       `json:"empty_layer,omitempty"`
}

// NewImageConfig creates a new OCI image configuration
func NewImageConfig(opts *BuildOptions, layerDigests []string) *ImageConfig {
	env := make([]string, 0, len(opts.Env))
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}

	ports := make(map[string]struct{})
	for _, p := range opts.ExposedPorts {
		ports[p] = struct{}{}
	}

	now := time.Now().UTC()

	diffIDs := make([]string, len(layerDigests))
	copy(diffIDs, layerDigests)

	return &ImageConfig{
		Architecture: opts.Architecture,
		OS:           opts.OS,
		Config: BuildConfig{
			Entrypoint:   opts.Entrypoint,
			Cmd:          opts.Cmd,
			Env:          env,
			WorkingDir:   opts.Workdir,
			User:         opts.User,
			ExposedPorts: ports,
			Labels:       opts.Labels,
			StopSignal:   opts.StopSignal,
		},
		RootFS: RootFS{
			Type:    "layers",
			DiffIDs: diffIDs,
		},
		Created: &now,
	}
}

// MarshalJSON returns the JSON representation of the config
func (c *ImageConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Architecture string      `json:"architecture"`
		OS           string      `json:"os"`
		Config       BuildConfig `json:"config"`
		RootFS       RootFS      `json:"rootfs"`
		History      []History   `json:"history,omitempty"`
		Created      *time.Time  `json:"created,omitempty"`
		Author       string      `json:"author,omitempty"`
	}{
		Architecture: c.Architecture,
		OS:           c.OS,
		Config:       c.Config,
		RootFS:       c.RootFS,
		History:      c.History,
		Created:      c.Created,
		Author:       c.Author,
	})
}
