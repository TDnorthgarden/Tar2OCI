package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	quiet   bool
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "tar2oci",
	Short: "Convert tar archives to OCI container images",
	Long: `Tar2OCI is a lightweight CLI tool that converts tar archives
into OCI-compliant container images without requiring Docker daemon.

It can generate Docker-loadable tar files or OCI layout directories,
and push images directly to remote registries.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress all output except errors")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "C", "", "Config file path (default: ./tar2oci.yaml)")

	rootCmd.AddCommand(buildCommand)
	rootCmd.AddCommand(pushCommand)
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}

func isVerbose() bool {
	return verbose && !quiet
}

func logInfo(format string, args ...interface{}) {
	if !quiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func logDebug(format string, args ...interface{}) {
	if isVerbose() {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
