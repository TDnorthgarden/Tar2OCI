package image

import (
	"encoding/json"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Manifest represents an OCI image manifest
type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Config        v1.Descriptor     `json:"config"`
	Layers        []v1.Descriptor   `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// NewManifest creates a new OCI image manifest
func NewManifest(configDigest digest.Digest, configSize int64, layers []*Layer) *Manifest {
	layerDescs := make([]v1.Descriptor, len(layers))
	for i, l := range layers {
		layerDescs[i] = l.Descriptor()
	}

	return &Manifest{
		SchemaVersion: 2,
		MediaType:     v1.MediaTypeImageManifest,
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: layerDescs,
	}
}

// MarshalJSON returns the JSON representation of the manifest
func (m *Manifest) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		SchemaVersion int               `json:"schemaVersion"`
		MediaType     string            `json:"mediaType"`
		Config        v1.Descriptor     `json:"config"`
		Layers        []v1.Descriptor   `json:"layers"`
		Annotations   map[string]string `json:"annotations,omitempty"`
	}{
		SchemaVersion: m.SchemaVersion,
		MediaType:     m.MediaType,
		Config:        m.Config,
		Layers:        m.Layers,
		Annotations:   m.Annotations,
	})
}
