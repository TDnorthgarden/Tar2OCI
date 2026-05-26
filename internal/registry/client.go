package registry

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/TDnorthgarden/Tar2OCI/internal/image"
	"github.com/opencontainers/go-digest"
)

// Client interacts with a container registry
type Client struct {
	registry   string
	repository string
	tag        string
	username   string
	password   string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new registry client
func NewClient(imageRef, username, password string) (*Client, error) {
	// Parse image reference: registry/repository:tag
	registry, repository, tag := parseImageRef(imageRef)

	// Resolve credentials
	username, password = ResolveCredentials(registry, username, password)

	return &Client{
		registry:   registry,
		repository: repository,
		tag:        tag,
		username:   username,
		password:   password,
		httpClient: &http.Client{},
		baseURL:    fmt.Sprintf("https://%s/v2", registry),
	}, nil
}

// Push pushes an image to the registry
func (c *Client) Push(builder *image.Builder) error {
	// Upload layer blobs
	layers := builder.GetLayers()
	for _, layer := range layers {
		exists, err := c.blobExists(layer.Digest)
		if err != nil {
			return fmt.Errorf("E010: failed to check blob: %w", err)
		}
		if exists {
			fmt.Fprintf(os.Stderr, "[DEBUG] Layer %s already exists, skipping\n", layer.Digest)
			continue
		}

		if err := c.uploadBlob(layer); err != nil {
			return fmt.Errorf("E011: failed to upload layer: %w", err)
		}
	}

	// Upload config blob
	configJSON, err := builder.GetConfigJSON()
	if err != nil {
		return err
	}
	_, configDigest, _, err := builder.BuildConfig()
	if err != nil {
		return err
	}

	exists, err := c.blobExists(configDigest)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.uploadBlobData(configDigest, configJSON); err != nil {
			return fmt.Errorf("E011: failed to upload config: %w", err)
		}
	}

	// Upload manifest
	manifestJSON, err := builder.GetManifestJSON()
	if err != nil {
		return err
	}
	_, manifestDigest, _, err := builder.BuildManifest()
	if err != nil {
		return err
	}

	if err := c.putManifest(manifestDigest, manifestJSON); err != nil {
		return fmt.Errorf("E011: failed to upload manifest: %w", err)
	}

	return nil
}

func (c *Client) blobExists(dgst digest.Digest) (bool, error) {
	url := fmt.Sprintf("%s/%s/blobs/%s", c.baseURL, c.repository, dgst.String())
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, err
	}
	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) uploadBlob(layer *image.Layer) error {
	url := fmt.Sprintf("%s/%s/blobs/uploads/", c.baseURL, c.repository)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("no upload location header")
	}

	// Upload the blob
	return c.uploadBlobData(layer.Digest, nil)
}

func (c *Client) uploadBlobData(dgst digest.Digest, data []byte) error {
	url := fmt.Sprintf("%s/%s/blobs/uploads/?digest=%s", c.baseURL, c.repository, dgst.String())
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.addAuth(req)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) putManifest(dgst digest.Digest, data []byte) error {
	url := fmt.Sprintf("%s/%s/manifests/%s", c.baseURL, c.repository, c.tag)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.addAuth(req)
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) addAuth(req *http.Request) {
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

func parseImageRef(ref string) (registry, repository, tag string) {
	// Default tag
	tag = "latest"

	// Split tag
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) > 1 {
		tag = parts[1]
	}
	ref = parts[0]

	// Split registry and repository
	slashIdx := strings.Index(ref, "/")
	if slashIdx == -1 {
		registry = "docker.io"
		repository = "library/" + ref
	} else {
		registry = ref[:slashIdx]
		repository = ref[slashIdx+1:]
	}

	return registry, repository, tag
}
