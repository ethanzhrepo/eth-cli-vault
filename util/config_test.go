package util

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	// On macOS, the default storage provider should be "keychain"
	if runtime.GOOS == "darwin" {
		if config.StorageProvider != "keychain" {
			t.Errorf("Expected default storage provider to be 'keychain' on macOS, got '%s'", config.StorageProvider)
		}
	} else {
		// On other platforms, it should be "local"
		if config.StorageProvider != "local" {
			t.Errorf("Expected default storage provider to be 'local' on non-macOS platforms, got '%s'", config.StorageProvider)
		}
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary config directory for testing
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get user home directory: %v", err)
	}

	tempConfigDir := filepath.Join(homeDir, ".eth-cli-wallet-test")

	// Ensure temp directory exists and is empty
	os.RemoveAll(tempConfigDir)
	if err := os.MkdirAll(tempConfigDir, 0700); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Set the custom config directory for testing
	SetConfigDir(tempConfigDir)

	// Clean up after test
	defer func() {
		ResetConfigDir()
		os.RemoveAll(tempConfigDir)
	}()

	// Create a test config
	testConfig := Config{
		StorageProvider: "test-provider",
		StoragePath:     "/test/path",
	}

	// Save the config
	err = SaveConfig(testConfig)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load the config
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches the saved config
	if loadedConfig.StorageProvider != testConfig.StorageProvider {
		t.Errorf("Loaded StorageProvider does not match: expected '%s', got '%s'",
			testConfig.StorageProvider, loadedConfig.StorageProvider)
	}

	if loadedConfig.StoragePath != testConfig.StoragePath {
		t.Errorf("Loaded StoragePath does not match: expected '%s', got '%s'",
			testConfig.StoragePath, loadedConfig.StoragePath)
	}
}
