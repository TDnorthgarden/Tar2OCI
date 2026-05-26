package main

import (
	"os"

	"github.com/TDnorthgarden/Tar2OCI/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
