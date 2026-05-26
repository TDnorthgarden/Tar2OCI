package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/TDnorthgarden/Tar2OCI/internal/config"
	"github.com/TDnorthgarden/Tar2OCI/internal/image"
	"github.com/TDnorthgarden/Tar2OCI/internal/output"
	"github.com/spf13/cobra"
)

var (
	inputs       []string
	outputPath   string
	imageName    string
	format       string
	baseImage    string
	platform     string
	entrypoint   []string
	defaultCmd   []string
	envVars      []string
	workdir      string
	user         string
	exposedPorts []string
	labels       []string
	compression  string
	stopSignal   string
	dryRun       bool
)

var buildCommand = &cobra.Command{
	Use:   "build",
	Short: "Build an OCI image from tar archives",
	Long: `Build creates an OCI-compliant container image from one or more tar archives.
Each tar archive becomes a layer in the resulting image.`,
	Example: `  # Single layer image
  tar2oci build --input app.tar --output my-app:v1.0.tar

  # Multi-layer image
  tar2oci build --input base.tar --input app.tar --output my-app:v1.0.tar

  # With OCI layout output
  tar2oci build --input app.tar --output ./my-app --format oci-layout --image my-app:v1.0`,
	RunE: runBuild,
}

func init() {
	buildCommand.Flags().StringSliceVarP(&inputs, "input", "i", nil, "Input tar file path (can be repeated)")
	buildCommand.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (tar file or directory)")
	buildCommand.Flags().StringVar(&imageName, "image", "", "Image name with tag (e.g., my-app:v1.0)")
	buildCommand.Flags().StringVarP(&format, "format", "f", "docker-tar", "Output format: docker-tar or oci-layout")
	buildCommand.Flags().StringVarP(&baseImage, "base", "b", "", "Base image path or registry reference")
	buildCommand.Flags().StringVarP(&platform, "platform", "p", "", "Target platform OS/ARCH (default: host)")
	buildCommand.Flags().StringSliceVarP(&entrypoint, "entrypoint", "e", nil, "Container entrypoint")
	buildCommand.Flags().StringSliceVarP(&defaultCmd, "cmd", "c", nil, "Container default command")
	buildCommand.Flags().StringSliceVar(&envVars, "env", nil, "Environment variables (KEY=VALUE)")
	buildCommand.Flags().StringVarP(&workdir, "workdir", "w", "", "Container working directory")
	buildCommand.Flags().StringVarP(&user, "user", "u", "", "Container user (UID:GID)")
	buildCommand.Flags().StringSliceVar(&exposedPorts, "exposed-port", nil, "Exposed ports")
	buildCommand.Flags().StringSliceVarP(&labels, "label", "l", nil, "Image labels (KEY=VALUE)")
	buildCommand.Flags().StringVar(&compression, "compression", "gzip", "Compression algorithm: gzip or zstd")
	buildCommand.Flags().StringVar(&stopSignal, "stop-signal", "SIGTERM", "Container stop signal")
	buildCommand.Flags().BoolVar(&dryRun, "dry-run", false, "Preview mode, do not write output")

	_ = buildCommand.MarkFlagRequired("input")
}

func runBuild(cmd *cobra.Command, args []string) error {
	// Load config file and merge with flags
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("E013: config file parse error: %w", err)
	}

	// Apply config defaults (CLI flags take precedence)
	applyConfig(cfg)

	// Validate inputs
	if len(inputs) == 0 {
		return fmt.Errorf("E006: missing input files")
	}

	// Resolve platform
	if platform == "" {
		platform = fmt.Sprintf("linux/%s", runtime.GOARCH)
	}
	platformParts := strings.SplitN(platform, "/", 2)
	if len(platformParts) != 2 {
		return fmt.Errorf("E007: invalid platform format: %s (expected OS/ARCH)", platform)
	}
	osName, arch := platformParts[0], platformParts[1]

	// Determine output path and image name
	if outputPath == "" {
		if imageName != "" {
			tag := imageName
			if !strings.Contains(tag, ":") {
				tag += ":latest"
			}
			safeTag := strings.ReplaceAll(tag, "/", "_")
			safeTag = strings.ReplaceAll(safeTag, ":", "_")
			if format == "oci-layout" {
				outputPath = safeTag
			} else {
				outputPath = safeTag + ".tar"
			}
		} else {
			if format == "oci-layout" {
				outputPath = "image"
			} else {
				outputPath = "image.tar"
			}
		}
	}

	if imageName == "" {
		imageName = "image:latest"
	}
	if !strings.Contains(imageName, ":") {
		imageName += ":latest"
	}

	logInfo("Building image %s", imageName)
	logInfo("Platform: %s/%s", osName, arch)
	logInfo("Layers: %d", len(inputs))
	logInfo("Compression: %s", compression)
	logInfo("Format: %s", format)

	// Parse env vars
	envMap := make(map[string]string)
	for _, e := range envVars {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Parse labels
	labelMap := make(map[string]string)
	for _, l := range labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) == 2 {
			labelMap[parts[0]] = parts[1]
		}
	}

	// Check disk space (rough estimate)
	var totalSize int64
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			return fmt.Errorf("E001: input file not found: %s", input)
		}
		totalSize += info.Size()
	}

	// Build image
	builder := image.NewBuilder(&image.BuildOptions{
		Entrypoint:   entrypoint,
		Cmd:          defaultCmd,
		Env:          envMap,
		Workdir:      workdir,
		User:         user,
		ExposedPorts: exposedPorts,
		Labels:       labelMap,
		StopSignal:   stopSignal,
		OS:           osName,
		Architecture: arch,
		Compression:  compression,
	})

	// Process layers
	for i, input := range inputs {
		logInfo("Processing layer %d/%d: %s", i+1, len(inputs), input)
		if err := builder.AddLayer(input); err != nil {
			return err
		}
	}

	// Load base image if specified
	if baseImage != "" {
		logInfo("Loading base image: %s", baseImage)
		if err := builder.SetBaseImage(baseImage); err != nil {
			return err
		}
	}

	if dryRun {
		logInfo("Dry run: would generate image at %s", outputPath)
		return nil
	}

	// Generate output
	logInfo("Writing output to %s", outputPath)
	switch format {
	case "docker-tar":
		return output.WriteDockerTar(builder, imageName, outputPath)
	case "oci-layout":
		return output.WriteOCILayout(builder, imageName, outputPath)
	default:
		return fmt.Errorf("E012: unsupported format: %s", format)
	}
}

func applyConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if len(entrypoint) == 0 && len(cfg.Entrypoint) > 0 {
		entrypoint = cfg.Entrypoint
	}
	if len(defaultCmd) == 0 && len(cfg.Cmd) > 0 {
		defaultCmd = cfg.Cmd
	}
	if len(envVars) == 0 && len(cfg.Env) > 0 {
		for k, v := range cfg.Env {
			envVars = append(envVars, k+"="+v)
		}
	}
	if workdir == "" && cfg.Workdir != "" {
		workdir = cfg.Workdir
	}
	if user == "" && cfg.User != "" {
		user = cfg.User
	}
	if len(exposedPorts) == 0 && len(cfg.ExposedPorts) > 0 {
		exposedPorts = cfg.ExposedPorts
	}
	if len(labels) == 0 && len(cfg.Labels) > 0 {
		for k, v := range cfg.Labels {
			labels = append(labels, k+"="+v)
		}
	}
	if compression == "gzip" && cfg.Compression != "" {
		compression = cfg.Compression
	}
	if format == "docker-tar" && cfg.Format != "" {
		format = cfg.Format
	}
	if platform == "" && cfg.Platform != "" {
		platform = cfg.Platform
	}
	if imageName == "" && cfg.Image != "" {
		imageName = cfg.Image
	}
}

func resolveOutputPath() string {
	if outputPath != "" {
		return outputPath
	}
	tag := imageName
	if tag == "" {
		tag = "image:latest"
	}
	if !strings.Contains(tag, ":") {
		tag += ":latest"
	}
	safeTag := strings.ReplaceAll(tag, "/", "_")
	safeTag = strings.ReplaceAll(safeTag, ":", "_")
	if format == "oci-layout" {
		return safeTag
	}
	return safeTag + ".tar"
}

func defaultOutputName() string {
	tag := imageName
	if tag == "" {
		return "image"
	}
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) > 0 {
		base := filepath.Base(parts[0])
		if base != "" {
			return base
		}
	}
	return "image"
}
