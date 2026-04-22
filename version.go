package hal

import "fmt"

// versionInfo holds the current semantic version
var versionInfo = struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
}{
	Major: 1,
	Minor: 0,
	Patch: 0,
}

// GetVersion returns the version string (e.g., "v1.0.0" or "v1.0.0-beta")
func GetVersion() string {
	if versionInfo.PreRelease != "" {
		return fmt.Sprintf("v%d.%d.%d-%s", versionInfo.Major, versionInfo.Minor, versionInfo.Patch, versionInfo.PreRelease)
	}
	return fmt.Sprintf("v%d.%d.%d", versionInfo.Major, versionInfo.Minor, versionInfo.Patch)
}

// VersionInfo holds version metadata
type VersionInfo struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
}

// GetVersionInfo returns detailed version information
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Major:      versionInfo.Major,
		Minor:      versionInfo.Minor,
		Patch:      versionInfo.Patch,
		PreRelease: versionInfo.PreRelease,
	}
}