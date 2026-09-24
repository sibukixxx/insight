// Package buildinfo reports which Insight Lab build is running, so every
// analysis run can record the engine that produced it.
package buildinfo

import "runtime/debug"

// Unknown is reported for any value the binary does not carry. It is never
// replaced by a guess derived from something else.
const Unknown = "UNKNOWN"

// Version is the release version injected at build time:
//
//	go build -ldflags "-X insight-lab/internal/buildinfo.Version=v0.9.0"
var Version string

// Info identifies the running engine build.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	// Dirty is "true" or "false" when the build recorded whether the git
	// working tree had uncommitted changes, otherwise Unknown.
	Dirty string `json:"dirty"`
}

// Get returns the running build's identity.
func Get() Info {
	info, ok := debug.ReadBuildInfo()
	return resolve(Version, info, ok)
}

func resolve(version string, info *debug.BuildInfo, ok bool) Info {
	out := Info{Version: version, Commit: Unknown, Dirty: Unknown}
	if out.Version == "" {
		out.Version = Unknown
	}
	if !ok || info == nil {
		return out
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if setting.Value != "" {
				out.Commit = setting.Value
			}
		case "vcs.modified":
			if setting.Value == "true" || setting.Value == "false" {
				out.Dirty = setting.Value
			}
		}
	}
	return out
}
