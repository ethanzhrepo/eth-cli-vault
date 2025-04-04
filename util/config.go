package util

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds the application configuration
type Config struct {
	StorageProvider string `json:"storageProvider"`
	StoragePath     string `json:"storagePath,omitempty"`
}

// Variable to hold the config directory for testing purposes
var customConfigDir string

// GetDefaultConfig returns the default configuration based on the current system
func GetDefaultConfig() Config {
	config := Config{
		StorageProvider: "local",
	}

	// Use keychain by default on macOS
	if runtime.GOOS == "darwin" {
		config.StorageProvider = "keychain"
	}

	return config
}

// LoadConfig loads the configuration from the config file
func LoadConfig() (Config, error) {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, ConfigFile)

	// Use default config if file doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return GetDefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("error reading config file: %v", err)
	}

	// Parse config
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("error parsing config file: %v", err)
	}

	return config, nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig(config Config) error {
	configDir := getConfigDir()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("error creating config directory: %v", err)
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing config: %v", err)
	}

	// Write to file
	configPath := filepath.Join(configDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	return nil
}

// getConfigDir returns the path to the configuration directory
func getConfigDir() string {
	// If custom config dir is set (for testing), use it
	if customConfigDir != "" {
		return customConfigDir
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fall back to current directory if home directory cannot be determined
		return ConfigDir
	}
	return filepath.Join(homeDir, ConfigDir)
}

// SetConfigDir sets a custom config directory (for testing)
func SetConfigDir(dir string) {
	customConfigDir = dir
}

// ResetConfigDir resets to using the default config directory
func ResetConfigDir() {
	customConfigDir = ""
}

// InitializeConfig creates the default configuration if it doesn't exist
func InitializeConfig() error {
	configDir := getConfigDir()
	configPath := filepath.Join(configDir, ConfigFile)

	// Skip if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	// Create default config
	config := GetDefaultConfig()
	return SaveConfig(config)
}
