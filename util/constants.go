package util

import (
	"github.com/spf13/viper"
)

const (
	ConfigDir               = ".eth-cli-wallet"
	ConfigFile              = "config.json"
	DEFAULT_CLOUD_FILE_DIR  = "/MyWallet"
	DEFAULT_CLOUD_FILE_NAME = "wallet.json"
)

var CLOUD_PROVIDERS = []string{"google", "dropbox", "s3", "box", "keychain"}

// GetWalletDir returns the wallet directory from config or default value
func GetWalletDir() string {
	if dir := viper.GetString("wallet.dir"); dir != "" {
		return dir
	}
	return DEFAULT_CLOUD_FILE_DIR
}
