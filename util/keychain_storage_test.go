package util

import (
	"runtime"
	"testing"
)

func TestKeychainStorage(t *testing.T) {
	// Skip test if not running on macOS
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping test on non-macOS platform")
	}

	// Create a keychain storage instance
	storage := &KeychainStorage{}

	// Test data
	testData := []byte("test wallet data")
	testFilePath := "test-wallet.json"

	// Test Put
	result, err := storage.Put(testData, testFilePath)
	if err != nil {
		t.Fatalf("Failed to store data in keychain: %v", err)
	}
	t.Logf("Put result: %s", result)

	// Test Get
	retrievedData, err := storage.Get(testFilePath)
	if err != nil {
		t.Fatalf("Failed to retrieve data from keychain: %v", err)
	}

	// Verify retrieved data matches the original
	if string(retrievedData) != string(testData) {
		t.Errorf("Retrieved data does not match original: got %s, want %s",
			string(retrievedData), string(testData))
	}

	// Test List
	wallets, err := storage.List("")
	if err != nil {
		t.Fatalf("Failed to list wallets in keychain: %v", err)
	}

	// Check if our test wallet is in the list
	found := false
	for _, wallet := range wallets {
		if wallet == "test-wallet" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Test wallet not found in list: %v", wallets)
	}
}
