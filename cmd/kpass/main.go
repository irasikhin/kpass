package main

import (
	"os"

	"github.com/irasikhin/kpass/internal/cli"
)

// version is set via -ldflags "-X main.version=vX.Y.Z" at release build time.
// When unset, cli falls back to debug.ReadBuildInfo for VCS revision.
var version = "dev"

func main() {
	cli.Version = version
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
