package main

import (
	"os"

	"github.com/irasikhin/kpass/internal/cli"
)

// version is set via -ldflags "-X main.version=vX.Y.Z" at release build time.
// When unset, cli falls back to debug.ReadBuildInfo for VCS revision.
var version = "dev"

// exit is a seam so tests can drive main() in-process without terminating the
// test binary.
var exit = os.Exit

func main() {
	cli.Version = version
	exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
