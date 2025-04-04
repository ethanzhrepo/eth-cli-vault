package util

import (
	"fmt"
	"runtime"
)

// SystemInfo contains information about the current system
type SystemInfo struct {
	OS           string
	Architecture string
	IsMacOS      bool
	IsLinux      bool
	IsWindows    bool
}

// GetSystemInfo returns information about the current system
func GetSystemInfo() SystemInfo {
	return SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		IsMacOS:      runtime.GOOS == "darwin",
		IsLinux:      runtime.GOOS == "linux",
		IsWindows:    runtime.GOOS == "windows",
	}
}

// PrintSystemInfo prints information about the current system
func PrintSystemInfo() {
	info := GetSystemInfo()
	fmt.Printf("Operating System: %s\n", info.OS)
	fmt.Printf("Architecture: %s\n", info.Architecture)

	if info.IsMacOS {
		fmt.Println("Using Apple Keychain for secure storage")
	}
}
