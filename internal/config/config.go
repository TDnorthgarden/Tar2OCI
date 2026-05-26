package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the Tar2OCI configuration file
type Config struct {
	Image        string            `yaml:"image"`
	Inputs       []string          `yaml:"inputs"`
	Entrypoint   []string          `yaml:"entrypoint"`
	Cmd          []string          `yaml:"cmd"`
	Env          map[string]string `yaml:"env"`
	Workdir      string            `yaml:"workdir"`
	User         string            `yaml:"user"`
	ExposedPorts []string          `yaml:"exposed_ports"`
	Labels       map[string]string `yaml:"labels"`
	StopSignal   string            `yaml:"stop_signal"`
	Compression  string            `yaml:"compression"`
	Format       string            `yaml:"format"`
	Platform     string            `yaml:"platform"`
	Registries   map[string]RegistryConfig `yaml:"registries"`
}

// RegistryConfig contains registry-specific configuration
type RegistryConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

// Load loads configuration from the specified path or default locations
func Load(path string) (*Config, error) {
	// Try specified path first
	if path != "" {
		return loadFile(path)
	}

	// Try default locations
	searchPaths := []string{
		"tar2oci.yaml",
		filepath.Join(os.Getenv("HOME"), ".tar2oci", "config.yaml"),
		"/etc/tar2oci/config.yaml",
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			return loadFile(p)
		}
	}

	// No config found, return nil
	return nil, nil
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
