package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/TDnorthgarden/Tar2OCI/internal/config"
	"github.com/TDnorthgarden/Tar2OCI/internal/image"
	"github.com/TDnorthgarden/Tar2OCI/internal/registry"
	"github.com/spf13/cobra"
)

var (
	pushInputs      []string
	pushImage       string
	pushUsername    string
	pushPassword    string
	pushPasswordStdin bool
	pushEntrypoint  []string
	pushDefaultCmd  []string
	pushEnvVars     []string
	pushWorkdir     string
	pushUser        string
	pushPorts       []string
	pushLabels      []string
	pushCompression string
	pushPlatform    string
	pushBaseImage   string
)

var pushCommand = &cobra.Command{
	Use:   "push",
	Short: "Build and push an OCI image to a registry",
	Long: `Push builds an OCI image from tar archives and pushes it directly
to a remote container registry.`,
	Example: `  # Push to registry with basic auth
  tar2oci push --input app.tar --image registry.example.com/team/my-app:v1.0 \
    --username admin --password secret

  # Push with password from stdin
  echo "secret" | tar2oci push --input app.tar --image registry.example.com/team/my-app:v1.0 \
    --username admin --password-stdin`,
	RunE: runPush,
}

func init() {
	pushCommand.Flags().StringSliceVarP(&pushInputs, "input", "i", nil, "Input tar file path (can be repeated)")
	pushCommand.Flags().StringVar(&pushImage, "image", "", "Remote image reference (registry/repo:tag)")
	pushCommand.Flags().StringVar(&pushUsername, "username", "", "Registry username")
	pushCommand.Flags().StringVar(&pushPassword, "password", "", "Registry password")
	pushCommand.Flags().BoolVar(&pushPasswordStdin, "password-stdin", false, "Read password from stdin")
	pushCommand.Flags().StringSliceVarP(&pushEntrypoint, "entrypoint", "e", nil, "Container entrypoint")
	pushCommand.Flags().StringSliceVarP(&pushDefaultCmd, "cmd", "c", nil, "Container default command")
	pushCommand.Flags().StringSliceVar(&pushEnvVars, "env", nil, "Environment variables (KEY=VALUE)")
	pushCommand.Flags().StringVarP(&pushWorkdir, "workdir", "w", "", "Container working directory")
	pushCommand.Flags().StringVarP(&pushUser, "user", "u", "", "Container user (UID:GID)")
	pushCommand.Flags().StringSliceVar(&pushPorts, "exposed-port", nil, "Exposed ports")
	pushCommand.Flags().StringSliceVarP(&pushLabels, "label", "l", nil, "Image labels (KEY=VALUE)")
	pushCommand.Flags().StringVar(&pushCompression, "compression", "gzip", "Compression: gzip or zstd")
	pushCommand.Flags().StringVarP(&pushPlatform, "platform", "p", "", "Target platform OS/ARCH")
	pushCommand.Flags().StringVarP(&pushBaseImage, "base", "b", "", "Base image path or registry reference")

	_ = pushCommand.MarkFlagRequired("input")
	_ = pushCommand.MarkFlagRequired("image")
}

func runPush(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("E013: config file parse error: %w", err)
	}
	applyPushConfig(cfg)

	// Validate
	if len(pushInputs) == 0 {
		return fmt.Errorf("E006: missing input files")
	}
	if pushImage == "" {
		return fmt.Errorf("E005: missing image repository")
	}

	// Handle password-stdin
	if pushPasswordStdin {
		passwordBytes, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("failed to read password from stdin: %w", err)
		}
		pushPassword = strings.TrimSpace(string(passwordBytes))
	}

	// Resolve credentials
	username, password := resolveCredentials()

	// Parse platform
	if pushPlatform == "" {
		pushPlatform = "linux/amd64"
	}
	platformParts := strings.SplitN(pushPlatform, "/", 2)
	if len(platformParts) != 2 {
		return fmt.Errorf("E007: invalid platform format: %s", pushPlatform)
	}

	// Parse env and labels
	envMap := parseKeyValuePairs(pushEnvVars)
	labelMap := parseKeyValuePairs(pushLabels)

	logInfo("Building image %s", pushImage)
	logInfo("Layers: %d", len(pushInputs))

	// Build image
	builder := image.NewBuilder(&image.BuildOptions{
		Entrypoint:   pushEntrypoint,
		Cmd:          pushDefaultCmd,
		Env:          envMap,
		Workdir:      pushWorkdir,
		User:         pushUser,
		ExposedPorts: pushPorts,
		Labels:       labelMap,
		StopSignal:   "SIGTERM",
		OS:           platformParts[0],
		Architecture: platformParts[1],
		Compression:  pushCompression,
	})

	// Process layers
	for i, input := range pushInputs {
		logInfo("Processing layer %d/%d: %s", i+1, len(pushInputs), input)
		if err := builder.AddLayer(input); err != nil {
			return err
		}
	}

	// Load base image
	if pushBaseImage != "" {
		logInfo("Loading base image: %s", pushBaseImage)
		if err := builder.SetBaseImage(pushBaseImage); err != nil {
			return err
		}
	}

	// Push to registry
	logInfo("Pushing to %s", pushImage)
	client, err := registry.NewClient(pushImage, username, password)
	if err != nil {
		return fmt.Errorf("E010: failed to create registry client: %w", err)
	}

	if err := client.Push(builder); err != nil {
		return fmt.Errorf("E011: registry push failed: %w", err)
	}

	logInfo("Successfully pushed %s", pushImage)
	return nil
}

func applyPushConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if len(pushInputs) == 0 && len(cfg.Inputs) > 0 {
		pushInputs = cfg.Inputs
	}
	if pushImage == "" && cfg.Image != "" {
		pushImage = cfg.Image
	}
	if len(pushEntrypoint) == 0 && len(cfg.Entrypoint) > 0 {
		pushEntrypoint = cfg.Entrypoint
	}
	if len(pushDefaultCmd) == 0 && len(cfg.Cmd) > 0 {
		pushDefaultCmd = cfg.Cmd
	}
	if len(pushEnvVars) == 0 && len(cfg.Env) > 0 {
		for k, v := range cfg.Env {
			pushEnvVars = append(pushEnvVars, k+"="+v)
		}
	}
	if pushWorkdir == "" && cfg.Workdir != "" {
		pushWorkdir = cfg.Workdir
	}
	if pushUser == "" && cfg.User != "" {
		pushUser = cfg.User
	}
	if pushPlatform == "" && cfg.Platform != "" {
		pushPlatform = cfg.Platform
	}
	if pushCompression == "gzip" && cfg.Compression != "" {
		pushCompression = cfg.Compression
	}
}

func resolveCredentials() (string, string) {
	// Use registry package to resolve credentials
	// Extract registry from image reference
	registryName := ""
	parts := strings.SplitN(pushImage, "/", 2)
	if len(parts) > 1 {
		registryName = parts[0]
	}
	return registry.ResolveCredentials(registryName, pushUsername, pushPassword)
}

func parseKeyValuePairs(pairs []string) map[string]string {
	result := make(map[string]string)
	for _, p := range pairs {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
