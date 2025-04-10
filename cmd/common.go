package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip39"
)

// WalletFile 钱包文件结构
type WalletFile struct {
	Version           int                    `json:"version"`
	EncryptedMnemonic util.EncryptedMnemonic `json:"encrypted_mnemonic"`
	HDPath            string                 `json:"hd_path"`
	DerivationPath    string                 `json:"derivation_path"`
	TestNet           bool                   `json:"testnet"`
}

// initTxConfig initializes the configuration for transaction commands
func initTxConfig() (string, error) {
	// Initialize config
	initConfig()

	// Get RPC URL from config
	rpcURL := viper.GetString("rpc")
	if rpcURL == "" {
		return "", fmt.Errorf("RPC URL not configured. Please run 'eth-cli config set rpc YOUR_RPC_URL'")
	}

	return rpcURL, nil
}

// getAddressFromMnemonic derives Ethereum address from mnemonic and passphrase
func getAddressFromMnemonic(mnemonic, passphrase string, derivationPath string) (string, []byte, error) {
	// Generate seed from mnemonic
	seed := bip39.NewSeed(mnemonic, passphrase)

	// Use proper HD wallet derivation
	wallet, err := hdwallet.NewFromSeed(seed)
	if err != nil {
		return "", nil, fmt.Errorf("error creating HD wallet: %v", err)
	}

	// Use provided derivation path or default if empty
	path := hdwallet.DefaultBaseDerivationPath
	if derivationPath != "" {
		var parsedPath accounts.DerivationPath
		parsedPath, err = hdwallet.ParseDerivationPath(derivationPath)
		if err != nil {
			return "", nil, fmt.Errorf("error parsing derivation path: %v", err)
		}
		path = parsedPath
	}

	account, err := wallet.Derive(path, false)
	if err != nil {
		return "", nil, fmt.Errorf("error deriving account: %v", err)
	}

	// Get private key
	privateKey, err := wallet.PrivateKey(account)
	if err != nil {
		return "", nil, fmt.Errorf("error getting private key: %v", err)
	}

	// Get address
	address := account.Address.Hex()

	return address, crypto.FromECDSA(privateKey), nil
}

// processWalletData processes wallet data to extract private key and address
func processWalletData(walletData []byte) (string, string, error) {
	// Parse wallet file
	var wallet WalletFile
	if err := json.Unmarshal(walletData, &wallet); err != nil {
		return "", "", fmt.Errorf("error parsing wallet file: %v", err)
	}

	// Get password
	fmt.Print("Please Enter \033[1;31mAES\033[0m Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", fmt.Errorf("error reading password: %v", err)
	}
	fmt.Println()
	password := string(passwordBytes)

	// Decrypt mnemonic
	mnemonic, err := util.DecryptMnemonic(wallet.EncryptedMnemonic, password)
	if err != nil {
		return "", "", fmt.Errorf("error decrypting mnemonic: %v", err)
	}

	// Ask if a passphrase was used
	fmt.Print("Did you use a BIP39 passphrase for this wallet? (y/n): ")
	var answer string
	fmt.Scanln(&answer)

	var passphrase string
	if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
		fmt.Print("Please Enter \033[1;31mBIP39\033[0m Passphrase: ")
		passphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", "", fmt.Errorf("error reading passphrase: %v", err)
		}
		fmt.Println()
		passphrase = string(passphraseBytes)
	}

	// Determine which derivation path to use
	derivationPath := ""
	if wallet.DerivationPath != "" {
		derivationPath = wallet.DerivationPath
	} else if wallet.HDPath != "" {
		derivationPath = wallet.HDPath
	}

	address, privateKeyBytes, err := getAddressFromMnemonic(mnemonic, passphrase, derivationPath)
	if err != nil {
		return "", "", err
	}

	// Get hex representation of private key
	privateKeyHex := fmt.Sprintf("%x", privateKeyBytes)

	return privateKeyHex, address, nil
}

// getPrivateKeyFromLocalFile retrieves a private key from a local wallet file
func getPrivateKeyFromLocalFile(filePath string) (string, string, error) {
	// Load from local file system using the wrapper function
	walletData, err := getWalletDataFromLocalFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("error loading wallet from local file: %v", err)
	}

	return processWalletData(walletData)
}

// getPrivateKeyFromProvider retrieves a private key from a provider
func getPrivateKeyFromProvider(provider string, name string) (string, string, error) {
	// Check if the provider is a cloud provider
	isCloudProvider := false
	for _, p := range util.CLOUD_PROVIDERS {
		if provider == p {
			isCloudProvider = true
			break
		}
	}

	var walletData []byte
	var err error

	if isCloudProvider {
		// Get from cloud provider using the wrapper function
		walletData, err = getWalletDataFromCloudProvider(provider, name)
		if err != nil {
			return "", "", fmt.Errorf("error loading wallet from %s: %v", provider, err)
		}
	} else {
		// Treat as local file
		walletData, err = getWalletDataFromLocalFile(provider)
		if err != nil {
			return "", "", fmt.Errorf("error loading wallet from local file: %v", err)
		}
	}

	return processWalletData(walletData)
}

// getWalletDataFromLocalFile retrieves wallet data from a local file
func getWalletDataFromLocalFile(filePath string) ([]byte, error) {
	// This is a wrapper around util.Get for better API clarity
	return util.Get(filePath, filePath)
}

// getWalletDataFromCloudProvider retrieves wallet data from a cloud provider
func getWalletDataFromCloudProvider(provider string, name string) ([]byte, error) {
	// This is a wrapper around util.Get for cloud providers
	cloudPath := filepath.Join(util.GetWalletDir(), name+".json")
	return util.Get(provider, cloudPath)
}
