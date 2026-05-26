package image

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Layer represents a processed image layer
type Layer struct {
	Digest    digest.Digest
	Size      int64
	MediaType string
	FilePath  string // temporary compressed file
}

// ProcessLayer reads a tar file, compresses it, and computes the digest
func ProcessLayer(tarPath string, compression string) (*Layer, error) {
	// Validate tar file
	if err := validateTar(tarPath); err != nil {
		return nil, fmt.Errorf("E002: invalid tar format: %s: %w", tarPath, err)
	}

	// Create temp file for compressed output
	tmpFile, err := os.CreateTemp("", "tar2oci-layer-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Open input tar
	input, err := os.Open(tarPath)
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("E001: input file not found: %s", tarPath)
	}
	defer input.Close()

	// Create hasher and multi-writer
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	// Create compressor
	var mediaType string

	switch compression {
	case "gzip":
		mediaType = v1.MediaTypeImageLayerGzip
		gzWriter := gzip.NewWriter(writer)
		if _, err := io.Copy(gzWriter, input); err != nil {
			os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("failed to compress layer: %w", err)
		}
		if err := gzWriter.Close(); err != nil {
			os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("failed to close gzip writer: %w", err)
		}

	case "zstd":
		mediaType = "application/vnd.oci.image.layer.v1.tar+zstd"
		zstdWriter, err := zstd.NewWriter(writer)
		if err != nil {
			os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("failed to create zstd writer: %w", err)
		}
		if _, err := io.Copy(zstdWriter, input); err != nil {
			os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("failed to compress layer: %w", err)
		}
		if err := zstdWriter.Close(); err != nil {
			os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("failed to close zstd writer: %w", err)
		}

	default:
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("E012: unsupported compression: %s", compression)
	}

	// Get actual file size
	info, err := tmpFile.Stat()
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to stat temp file: %w", err)
	}

	dgst := digest.NewDigestFromBytes(digest.SHA256, hasher.Sum(nil))

	return &Layer{
		Digest:    dgst,
		Size:      info.Size(),
		MediaType: mediaType,
		FilePath:  tmpFile.Name(),
	}, nil
}

// Descriptor returns an OCI descriptor for this layer
func (l *Layer) Descriptor() v1.Descriptor {
	return v1.Descriptor{
		MediaType: l.MediaType,
		Digest:    l.Digest,
		Size:      l.Size,
	}
}

// Cleanup removes the temporary file
func (l *Layer) Cleanup() {
	if l.FilePath != "" {
		os.Remove(l.FilePath)
	}
}

// validateTar checks if the file is a valid tar archive
func validateTar(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	// Try to read at least one header
	_, err = tr.Next()
	if err == io.EOF {
		// Empty tar is still valid
		return nil
	}
	return err
}
