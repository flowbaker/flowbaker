package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	// Version is the semantic version (set by ldflags during build)
	Version = "dev"
	
	// GitCommit is the git commit hash (set by ldflags during build)
	GitCommit = ""
	
	// BuildDate is the build date (set by ldflags during build)
	BuildDate = ""
	
	// BuildUser is the user who built the binary (set by ldflags during build)
	BuildUser = ""
)

// Info represents version and build information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	BuildUser string `json:"build_user,omitempty"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Get returns the version information
func Get() Info {
	return Info{
		Version:   getVersion(),
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		BuildUser: BuildUser,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// GetVersion returns the version string
func GetVersion() string {
	return getVersion()
}

// getVersion returns the version, falling back to build info if ldflags version is not set
func getVersion() string {
	if Version != "" && Version != "dev" {
		return Version
	}
	
	// Fallback to build info
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "(devel)" && info.Main.Version != "" {
			return info.Main.Version
		}
	}
	
	return "dev"
}

// GetShortVersion returns a short version string
func GetShortVersion() string {
	version := getVersion()
	if GitCommit != "" && len(GitCommit) >= 7 {
		return fmt.Sprintf("%s-%s", version, GitCommit[:7])
	}
	return version
}