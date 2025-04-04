package util

import (
	"runtime"
)

// GetRecommendedStorageProvider returns the recommended storage provider based on the current system
func GetRecommendedStorageProvider() string {
	if runtime.GOOS == "darwin" {
		return "keychain"
	}
	return "local"
}

// IsKeychainAvailable checks if Apple Keychain storage is available on the current system
func IsKeychainAvailable() bool {
	return runtime.GOOS == "darwin"
}
