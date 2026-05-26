package image

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/go-digest"
)

// LoadBaseImage loads a base image from a path (Docker tar or OCI layout)
func LoadBaseImage(basePath string) ([]*Layer, error) {
	// Check if path exists
	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("E008: base image not found: %s", basePath)
	}

	if info.IsDir() {
		// Try OCI layout
		return loadOCILayout(basePath)
	}

	// Try Docker tar
	return loadDockerTar(basePath)
}

func loadDockerTar(tarPath string) ([]*Layer, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var manifestData []byte
	layerFiles := make(map[string]string)

	// Extract files to temp directory
	tmpDir, err := os.MkdirTemp("", "tar2oci-base-*")
	if err != nil {
		return nil, err
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Read manifest
		if header.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			manifestData = data
			continue
		}

		// Extract layer files
		if strings.HasSuffix(header.Name, ".tar.gz") || strings.HasSuffix(header.Name, "/layer.tar") {
			layerPath := filepath.Join(tmpDir, header.Name)
			if err := os.MkdirAll(filepath.Dir(layerPath), 0755); err != nil {
				return nil, err
			}
			out, err := os.Create(layerPath)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return nil, err
			}
			out.Close()
			layerFiles[header.Name] = layerPath
		}
	}

	// Parse manifest to get layer order
	if manifestData == nil {
		return nil, fmt.Errorf("E002: invalid Docker tar: missing manifest.json")
	}

	var manifest []struct {
		Layers []string `json:"Layers"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, err
	}

	if len(manifest) == 0 {
		return nil, fmt.Errorf("E002: invalid Docker tar: empty manifest")
	}

	// Create layers from extracted files
	var layers []*Layer
	for _, layerName := range manifest[0].Layers {
		layerPath, ok := layerFiles[layerName]
		if !ok {
			return nil, fmt.Errorf("E002: invalid Docker tar: missing layer %s", layerName)
		}

		// Compute digest
		layerFile, err := os.Open(layerPath)
		if err != nil {
			return nil, err
		}
		h := digest.SHA256.Digester().Hash()
		if _, err := io.Copy(h, layerFile); err != nil {
			layerFile.Close()
			return nil, err
		}
		layerFile.Close()

		info, err := os.Stat(layerPath)
		if err != nil {
			return nil, err
		}

		layers = append(layers, &Layer{
			Digest:    digest.NewDigestFromBytes(digest.SHA256, h.Sum(nil)),
			Size:      info.Size(),
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			FilePath:  layerPath,
		})
	}

	return layers, nil
}

func loadOCILayout(layoutPath string) ([]*Layer, error) {
	// Read index.json
	indexPath := filepath.Join(layoutPath, "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("E002: invalid OCI layout: missing index.json")
	}

	var index struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(indexData, &index); err != nil {
		return nil, err
	}

	if len(index.Manifests) == 0 {
		return nil, fmt.Errorf("E002: invalid OCI layout: no manifests")
	}

	// Read manifest
	manifestDigest := index.Manifests[0].Digest
	manifestPath := filepath.Join(layoutPath, "blobs", "sha256", strings.TrimPrefix(manifestDigest, "sha256:"))
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("E002: invalid OCI layout: missing manifest")
	}

	var manifest struct {
		Layers []struct {
			Digest    string `json:"digest"`
			MediaType string `json:"mediaType"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, err
	}

	// Load layers
	var layers []*Layer
	for _, layerDesc := range manifest.Layers {
		layerHash := strings.TrimPrefix(layerDesc.Digest, "sha256:")
		layerPath := filepath.Join(layoutPath, "blobs", "sha256", layerHash)

		info, err := os.Stat(layerPath)
		if err != nil {
			return nil, fmt.Errorf("E002: invalid OCI layout: missing layer %s", layerHash)
		}

		dgst, err := digest.Parse(layerDesc.Digest)
		if err != nil {
			return nil, err
		}

		layers = append(layers, &Layer{
			Digest:    dgst,
			Size:      info.Size(),
			MediaType: layerDesc.MediaType,
			FilePath:  layerPath,
		})
	}

	return layers, nil
}
