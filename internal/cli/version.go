package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is overwritten by cmd/kpass/main.go from the binary's ldflags-injected
// version string. Defaults to "dev" for source builds; falls back to the VCS
// revision from debug.ReadBuildInfo when no explicit version was injected.
var Version = "dev"

// versionString returns the resolved version: explicit ldflags value when
// available, otherwise "dev (<short-sha>[+dirty])" from BuildInfo, otherwise
// just "dev".
func versionString() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	if v != "dev" {
		return v
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return v
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = "+dirty"
			}
		}
	}
	if rev == "" {
		return v
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return fmt.Sprintf("dev (%s%s)", rev, modified)
}

// versionLine is what kong.VersionFlag prints. Includes the GOOS/GOARCH so
// users reporting bugs always include the platform.
func versionLine() string {
	return fmt.Sprintf("kpass %s %s/%s", versionString(), runtime.GOOS, runtime.GOARCH)
}
