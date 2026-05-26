package image

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/opencontainers/go-digest"
)

// BuildOptions contains all the options for building an image
type BuildOptions struct {
	Entrypoint   []string
	Cmd          []string
	Env          map[string]string
	Workdir      string
	User         string
	ExposedPorts []string
	Labels       map[string]string
	StopSignal   string
	OS           string
	Architecture string
	Compression  string
}

// Builder builds OCI images
type Builder struct {
	opts   *BuildOptions
	layers []*Layer
}

// NewBuilder creates a new image builder
func NewBuilder(opts *BuildOptions) *Builder {
	return &Builder{
		opts:   opts,
		layers: make([]*Layer, 0),
	}
}

// AddLayer processes a tar file and adds it as a layer
func (b *Builder) AddLayer(tarPath string) error {
	layer, err := ProcessLayer(tarPath, b.opts.Compression)
	if err != nil {
		return err
	}
	b.layers = append(b.layers, layer)
	return nil
}

// SetBaseImage loads a base image and prepends its layers
func (b *Builder) SetBaseImage(basePath string) error {
	baseLayers, err := LoadBaseImage(basePath)
	if err != nil {
		return err
	}
	// Prepend base layers
	b.layers = append(baseLayers, b.layers...)
	return nil
}

// GetLayers returns all layers
func (b *Builder) GetLayers() []*Layer {
	return b.layers
}

// BuildConfig generates the OCI image configuration
func (b *Builder) BuildConfig() (*ImageConfig, digest.Digest, int64, error) {
	// Get layer diff IDs (uncompressed digests)
	diffIDs := make([]string, len(b.layers))
	for i, l := range b.layers {
		diffIDs[i] = l.Digest.String()
	}

	config := NewImageConfig(b.opts, diffIDs)

	// Marshal config to JSON
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Compute config digest
	hasher := sha256.New()
	hasher.Write(configJSON)
	configDigest := digest.NewDigestFromBytes(digest.SHA256, hasher.Sum(nil))

	return config, configDigest, int64(len(configJSON)), nil
}

// BuildManifest generates the OCI image manifest
func (b *Builder) BuildManifest() (*Manifest, digest.Digest, int64, error) {
	_, configDigest, configSize, err := b.BuildConfig()
	if err != nil {
		return nil, "", 0, err
	}

	manifest := NewManifest(configDigest, configSize, b.layers)

	// Marshal manifest to JSON
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Compute manifest digest
	hasher := sha256.New()
	hasher.Write(manifestJSON)
	manifestDigest := digest.NewDigestFromBytes(digest.SHA256, hasher.Sum(nil))

	return manifest, manifestDigest, int64(len(manifestJSON)), nil
}

// GetConfigJSON returns the config as JSON bytes
func (b *Builder) GetConfigJSON() ([]byte, error) {
	config, _, _, err := b.BuildConfig()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(config, "", "  ")
}

// GetManifestJSON returns the manifest as JSON bytes
func (b *Builder) GetManifestJSON() ([]byte, error) {
	manifest, _, _, err := b.BuildManifest()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(manifest, "", "  ")
}

// Cleanup removes all temporary layer files
func (b *Builder) Cleanup() {
	for _, l := range b.layers {
		l.Cleanup()
	}
}

// CopyLayer copies a layer file to a writer
func (b *Builder) CopyLayer(layerIndex int, w io.Writer) error {
	if layerIndex < 0 || layerIndex >= len(b.layers) {
		return fmt.Errorf("layer index out of range: %d", layerIndex)
	}

	f, err := os.Open(b.layers[layerIndex].FilePath)
	if err != nil {
		return fmt.Errorf("failed to open layer file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}
