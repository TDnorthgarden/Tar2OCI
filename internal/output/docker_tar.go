package output

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TDnorthgarden/Tar2OCI/internal/image"
)

// DockerManifestEntry represents a Docker tar manifest entry
type DockerManifestEntry struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// WriteDockerTar writes the image as a Docker-loadable tar file
func WriteDockerTar(builder *image.Builder, imageName string, outputPath string) error {
	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("E003: permission denied: %s", dir)
		}
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("E003: permission denied: %s", outputPath)
	}
	defer outFile.Close()

	tw := tar.NewWriter(outFile)
	defer tw.Close()

	// Get config
	configJSON, err := builder.GetConfigJSON()
	if err != nil {
		return err
	}
	_, configDigest, _, err := builder.BuildConfig()
	if err != nil {
		return err
	}

	// Compute config filename (without "sha256:" prefix)
	configName := strings.TrimPrefix(configDigest.String(), "sha256:") + ".json"

	// Write config file
	if err := writeTarEntry(tw, configName, configJSON); err != nil {
		return err
	}

	// Write layers
	layers := builder.GetLayers()
	layerNames := make([]string, len(layers))
	for i, layer := range layers {
		layerName := strings.TrimPrefix(layer.Digest.String(), "sha256:") + ".tar.gz"
		layerNames[i] = layerName

		// Create a pipe to stream layer data
		pipeR, pipeW := io.Pipe()
		go func() {
			defer pipeW.Close()
			if err := builder.CopyLayer(i, pipeW); err != nil {
				pipeW.CloseWithError(err)
			}
		}()

		// Write layer to tar
		header := &tar.Header{
			Name: layerName,
			Mode: 0644,
			Size: layer.Size,
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := io.Copy(tw, pipeR); err != nil {
			return err
		}
	}

	// Create manifest
	manifest := []DockerManifestEntry{
		{
			Config:   configName,
			RepoTags: []string{imageName},
			Layers:   layerNames,
		},
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeTarEntry(tw, "manifest.json", manifestJSON); err != nil {
		return err
	}

	// Write repositories file
	parts := strings.SplitN(imageName, ":", 2)
	repoName := parts[0]
	tag := "latest"
	if len(parts) > 1 {
		tag = parts[1]
	}
	repos := map[string]map[string]string{
		repoName: {
			tag: layerNames[len(layerNames)-1], // last layer digest
		},
	}
	reposJSON, err := json.Marshal(repos)
	if err != nil {
		return err
	}
	if err := writeTarEntry(tw, "repositories", reposJSON); err != nil {
		return err
	}

	return nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
