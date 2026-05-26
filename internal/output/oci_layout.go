package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TDnorthgarden/Tar2OCI/internal/image"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// WriteOCILayout writes the image as an OCI layout directory
func WriteOCILayout(builder *image.Builder, imageName string, outputPath string) error {
	// Create directory structure
	blobsDir := filepath.Join(outputPath, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0755); err != nil {
		return fmt.Errorf("E003: permission denied: %s", blobsDir)
	}

	// Write oci-layout file
	ociLayout := map[string]string{
		"imageLayoutVersion": "1.0.0",
	}
	ociLayoutJSON, err := json.MarshalIndent(ociLayout, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "oci-layout"), ociLayoutJSON, 0644); err != nil {
		return err
	}

	// Write config blob
	configJSON, err := builder.GetConfigJSON()
	if err != nil {
		return err
	}
	_, configDigest, _, err := builder.BuildConfig()
	if err != nil {
		return err
	}
	configHash := strings.TrimPrefix(configDigest.String(), "sha256:")
	if err := os.WriteFile(filepath.Join(blobsDir, configHash), configJSON, 0644); err != nil {
		return err
	}

	// Write layer blobs
	layers := builder.GetLayers()
	for i, layer := range layers {
		layerHash := strings.TrimPrefix(layer.Digest.String(), "sha256:")
		layerPath := filepath.Join(blobsDir, layerHash)

		// Copy layer file
		if err := copyFile(layer.FilePath, layerPath); err != nil {
			return fmt.Errorf("failed to write layer blob: %w", err)
		}
		_ = i
	}

	// Write manifest blob
	manifestJSON, err := builder.GetManifestJSON()
	if err != nil {
		return err
	}
	_, manifestDigest, _, err := builder.BuildManifest()
	if err != nil {
		return err
	}
	manifestHash := strings.TrimPrefix(manifestDigest.String(), "sha256:")
	if err := os.WriteFile(filepath.Join(blobsDir, manifestHash), manifestJSON, 0644); err != nil {
		return err
	}

	// Write index.json
	index := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     v1.MediaTypeImageIndex,
		"manifests": []map[string]interface{}{
			{
				"mediaType": v1.MediaTypeImageManifest,
				"digest":    manifestDigest.String(),
				"size":      int64(len(manifestJSON)),
				"annotations": map[string]string{
					"io.containerd.image.name": imageName,
				},
			},
		},
	}
	indexJSON, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputPath, "index.json"), indexJSON, 0644); err != nil {
		return err
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
