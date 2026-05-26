package registry

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DockerConfig represents the Docker config file structure
type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
}

// DockerAuth represents authentication for a registry
type DockerAuth struct {
	Auth string `json:"auth"`
}

// ResolveCredentials resolves credentials from various sources
func ResolveCredentials(registry, username, password string) (string, string) {
	// 1. Use provided credentials
	if username != "" && password != "" {
		return username, password
	}

	// 2. Check environment variables
	if username == "" {
		username = os.Getenv("TAR2OCI_USERNAME")
	}
	if password == "" {
		password = os.Getenv("TAR2OCI_PASSWORD")
	}

	// 3. Check Docker config
	if username == "" || password == "" {
		u, p := loadDockerConfig(registry)
		if username == "" {
			username = u
		}
		if password == "" {
			password = p
		}
	}

	return username, password
}

func loadDockerConfig(registry string) (string, string) {
	configPath := filepath.Join(os.Getenv("HOME"), ".docker", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", ""
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", ""
	}

	// Try exact match first
	if auth, ok := config.Auths[registry]; ok {
		return decodeAuth(auth.Auth)
	}

	// Try with https:// prefix
	if auth, ok := config.Auths["https://"+registry]; ok {
		return decodeAuth(auth.Auth)
	}

	return "", ""
}

func decodeAuth(auth string) (string, string) {
	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", ""
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}
