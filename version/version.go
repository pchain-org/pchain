package version

import (
	"fmt"
)

const (
	Major = 1  // Major version component of the current release
	Minor = 3  // Minor version component of the current release
	Patch = 4  // Patch version component of the current release
	Meta  = "" // Version metadata to append to the version string
)

var (
	// The full version string
	Version string

	// GitCommit is set with --ldflags "-X main.gitCommit=$(git rev-parse HEAD)"
	GitCommit string
)

func init() {
	Version = fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)
	if Meta != "" {
		Version += "-" + Meta
	}

	if GitCommit != "" {
		Version += "-" + GitCommit[:8]
	}
}
