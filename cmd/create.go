package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/term"
)

// 钱包文件结构
type WalletFile struct {
	Version           int                    `json:"version"`
	EncryptedMnemonic util.EncryptedMnemonic `json:"encrypted_mnemonic"`
	HDPath            string                 `json:"hd_path"`
	DerivationPath    string                 `json:"derivation_path"`
	TestNet           bool                   `json:"testnet"`
}

// CreateCmd 返回 create 命令
func CreateCmd() *cobra.Command {
	var outputLocations string
	var walletName string
	var withPassphrase bool
	var force bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Ethereum wallet",
		Long:  `Create a new Ethereum wallet with BIP39 mnemonic and optional passphrase, save it to local filesystem or cloud storage.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 初始化配置
			initConfig()

			// 检查必要参数
			if outputLocations == "" {
				fmt.Println("Error: --output parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			if walletName == "" {
				fmt.Println("Error: --name parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			// 解析输出位置
			outputs := strings.Split(outputLocations, ",")
			var localPaths []string
			var cloudProviders []string

			for _, output := range outputs {
				output = strings.TrimSpace(output)
				isCloudProvider := false
				for _, provider := range util.CLOUD_PROVIDERS {
					if output == provider {
						cloudProviders = append(cloudProviders, output)
						isCloudProvider = true
						break
					}
				}
				if !isCloudProvider {
					localPaths = append(localPaths, output)
				}
			}

			// 检查是否已存在同名文件
			if !force {
				// 检查本地文件
				for _, path := range localPaths {
					fullPath := path
					if !strings.HasSuffix(path, ".json") {
						// 如果是目录，则添加钱包名和扩展名
						fullPath = filepath.Join(path, walletName+".json")
					}
					if _, err := os.Stat(fullPath); err == nil {
						fmt.Printf("Error: Wallet file already exists at %s. Use -f or --force to overwrite.\n", fullPath)
						os.Exit(1)
					}
				}
			}

			// 获取AES加密密码
			fmt.Println("\nPlease enter \033[1;31mAES Encryption Password\033[0m for extra security.")
			fmt.Println("This password will be used to encrypt your \033[1;31mwallet file\033[0m.")
			fmt.Println("If you forget it, you will not be able to recover your wallet.")
			fmt.Println("Please enter it carefully.")
			fmt.Println("It is recommended to use a strong password: \033[1;31m8 characters or more, including uppercase, lowercase, numbers, and special characters\033[0m.")
			fmt.Println("Example: MyPassword123!")
			fmt.Print("Please Enter \033[1;31mAES Encryption Password\033[0m: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Printf("\nError reading password: %v\n", err)
				os.Exit(1)
			}
			fmt.Print("\nPlease Re-Enter \033[1;31mAES Encryption Password\033[0m: ")
			confirmPasswordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Printf("\nError reading password confirmation: %v\n", err)
				os.Exit(1)
			}
			fmt.Println()

			if string(passwordBytes) != string(confirmPasswordBytes) {
				fmt.Println("Error: Passwords do not match")
				os.Exit(1)
			}
			password := string(passwordBytes)

			// 检查密码强度
			if !isStrongPassword(password) {
				fmt.Println("Error: Password is not strong enough. It must be at least 8 characters and include uppercase, lowercase, numbers, and special characters.")
				os.Exit(1)
			}

			// 如果需要passphrase，则从用户那里获取
			var passphrase string
			if !withPassphrase {
				fmt.Println("\nPlease enter \033[1;31mBIP39 Passphrase\033[0m for extra security.")
				fmt.Println("This passphrase will be used to encrypt your \033[1;31mmnemonic\033[0m.")
				fmt.Println("If you forget it, you will not be able to recover your wallet.")
				fmt.Println("Please enter it carefully.")
				fmt.Println("It is recommended to use a strong passphrase: \033[1;31m8 characters or more, including uppercase, lowercase, numbers, and special characters\033[0m.")
				fmt.Println("Example: MyPassphrase123!")
				fmt.Println("If you don't want to use a passphrase, exit and run the command again with the \033[1;31m--without-passphrase\033[0m flag.")
				fmt.Println()

				fmt.Print("Please Enter BIP39 Passphrase: ")
				passphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					fmt.Printf("\nError reading passphrase: %v\n", err)
					os.Exit(1)
				}
				fmt.Print("\nPlease ReEnter BIP39 Passphrase: ")
				confirmPassphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					fmt.Printf("\nError reading passphrase confirmation: %v\n", err)
					os.Exit(1)
				}
				fmt.Println()

				if string(passphraseBytes) != string(confirmPassphraseBytes) {
					fmt.Println("Error: Passphrases do not match")
					os.Exit(1)
				}
				passphrase = string(passphraseBytes)
			}

			// 生成BIP39助记词
			entropy, err := bip39.NewEntropy(256) // 生成256位熵，对应24个单词
			if err != nil {
				fmt.Printf("Error generating entropy: %v\n", err)
				os.Exit(1)
			}
			mnemonic, err := bip39.NewMnemonic(entropy)
			if err != nil {
				fmt.Printf("Error generating mnemonic: %v\n", err)
				os.Exit(1)
			}

			// 使用AES加密助记词
			encryptedMnemonic, err := util.EncryptMnemonic(mnemonic, password)
			if err != nil {
				fmt.Printf("Error encrypting mnemonic: %v\n", err)
				os.Exit(1)
			}

			// 创建钱包文件对象
			wallet := WalletFile{
				Version:           1,
				EncryptedMnemonic: encryptedMnemonic,
				HDPath:            "m/44'/60'/0'/0",   // Ethereum的标准HD路径
				DerivationPath:    "m/44'/60'/0'/0/0", // 第一个账户的路径
				TestNet:           false,
			}

			// 序列化为JSON
			walletJSON, err := json.MarshalIndent(wallet, "", "  ")
			if err != nil {
				fmt.Printf("Error serializing wallet: %v\n", err)
				os.Exit(1)
			}

			// 保存到指定位置
			// 保存到本地文件系统
			for _, path := range localPaths {
				fullPath := path
				if !strings.HasSuffix(path, ".json") {
					// 如果是目录，则添加钱包名和扩展名
					fullPath = filepath.Join(path, walletName+".json")
				}

				result, err := util.Put(path, walletJSON, fullPath, force)
				if err != nil {
					fmt.Printf("Error saving wallet to %s: %v\n", fullPath, err)
				} else {
					fmt.Println(result)
				}
			}

			// 保存到云存储
			for _, provider := range cloudProviders {
				cloudPath := filepath.Join(util.DEFAULT_CLOUD_FILE_DIR, walletName+".json")
				result, err := util.Put(provider, walletJSON, cloudPath, force)
				if err != nil {
					fmt.Printf("Error saving wallet to %s: %v\n", provider, err)
				} else {
					fmt.Println(result)
				}
			}

			// 输出钱包地址
			seed := bip39.NewSeed(mnemonic, passphrase)
			privateKey, err := crypto.ToECDSA(seed[:32]) // 使用seed的前32字节作为私钥
			if err != nil {
				fmt.Printf("Error deriving private key: %v\n", err)
				os.Exit(1)
			}

			address := crypto.PubkeyToAddress(privateKey.PublicKey)
			fmt.Printf("\nYour wallet address is: \033[1;32m%s\033[0m\n", address.Hex())
			fmt.Println("\nBefore using this wallet, please test it with the getAddress command:")

			if len(localPaths) > 0 {
				fullPath := localPaths[0]
				if !strings.HasSuffix(fullPath, ".json") {
					fullPath = filepath.Join(fullPath, walletName+".json")
				}
				fmt.Printf("  eth-cli get -i %s\n", fullPath)
			}

			if len(cloudProviders) > 0 {
				for _, provider := range cloudProviders {
					fmt.Printf("  eth-cli get -i %s -n %s\n", provider, walletName)
				}
			}

			// 安全提示
			fmt.Println("\n\033[1;31mIMPORTANT: Keep your passwords safe. If you lose them, you'll permanently lose access to your assets.\033[0m")
			fmt.Println("\033[1;31mBoth encryption steps use highly secure algorithms; current technology cannot recover lost passwords.\033[0m")

			// 成功提示
			fmt.Println("\n\033[1;32mSuccess: Wallet created successfully.\033[0m")

		},
	}

	// 添加命令参数
	cmd.Flags().StringVarP(&outputLocations, "output", "o", "", "Comma-separated list of output locations (local path or cloud provider)")
	cmd.Flags().StringVarP(&walletName, "name", "n", "", "Name of the wallet file")
	cmd.Flags().BoolVar(&withPassphrase, "without-passphrase", false, "Skip the BIP39 passphrase step")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite if wallet file already exists")

	cmd.MarkFlagRequired("output")
	cmd.MarkFlagRequired("name")

	return cmd
}

// 检查密码强度
func isStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, c := range password {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasNumber = true
		case c == '!' || c == '@' || c == '#' || c == '$' || c == '%' || c == '^' || c == '&' || c == '*' || c == '(' || c == ')' || c == '-' || c == '_' || c == '+' || c == '=' || c == '{' || c == '}' || c == '[' || c == ']' || c == '|' || c == ':' || c == ';' || c == '"' || c == '\'' || c == '<' || c == '>' || c == ',' || c == '.' || c == '?' || c == '/' || c == '\\' || c == '`' || c == '~':
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}
