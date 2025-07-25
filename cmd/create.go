package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/term"
)

// CreateCmd 返回 create 命令
func CreateCmd() *cobra.Command {
	var outputLocations string
	var walletName string
	var withPassphrase bool
	var force bool
	var fsPath string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Ethereum wallet",
		Long: `Create a new Ethereum wallet with BIP39 mnemonic and optional passphrase, save it to local filesystem or cloud storage.

Supported storage options:
- Local file: Use "--output fs --path /path/to/file.json"
- Cloud storage: Use "--output provider1,provider2 --name walletName"
  Supported providers: google, dropbox, s3, box, keychain (macOS only)
- Mixed: Use "--output /local/path,google,dropbox --name walletName"

Examples:
  eth-cli create --output fs --path /tmp/wallet.json
  eth-cli create --output google,dropbox --name myWallet
  eth-cli create --output /home/user/wallets,google --name myWallet`,
		Run: func(cmd *cobra.Command, args []string) {
			// 初始化配置
			initConfig()

			// 检查必要参数
			if outputLocations == "" {
				fmt.Println("Error: --output parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			// 处理新的fs模式
			if outputLocations == "fs" {
				if fsPath == "" {
					fmt.Println("Error: --path parameter is required when using --output fs")
					cmd.Usage()
					os.Exit(1)
				}
			} else if walletName == "" {
				// 对于非fs模式，仍然需要name参数
				fmt.Println("Error: --name parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			// 解析输出位置
			outputs := strings.Split(outputLocations, ",")
			var localPaths []string
			var cloudProviders []string

			// 处理fs模式
			if outputLocations == "fs" {
				localPaths = append(localPaths, fsPath)
			} else {
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
			}

			// 检查是否已存在同名文件
			if !force {
				// 检查本地文件
				for _, path := range localPaths {
					fullPath := path
					if outputLocations != "fs" && !strings.HasSuffix(path, ".json") {
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

			// 询问用户是否要设置BIP39 passphrase
			var passphrase string
			if !withPassphrase {
				fmt.Println("\nDo you want to set a \033[1;31mBIP39 Passphrase\033[0m for extra security?")
				fmt.Println("The passphrase will be used to encrypt your \033[1;31mmnemonic\033[0m.")
				fmt.Println("If you forget it, you will not be able to recover your wallet.")
				fmt.Println("If you choose not to set a passphrase, your wallet will use an empty passphrase.")
				fmt.Print("Set BIP39 Passphrase? (y/n): ")

				var answer string
				fmt.Scanln(&answer)

				if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
					fmt.Println("\nPlease enter \033[1;31mBIP39 Passphrase\033[0m for extra security.")
					fmt.Println("It is recommended to use a strong passphrase: \033[1;31m8 characters or more, including uppercase, lowercase, numbers, and special characters\033[0m.")
					fmt.Println("Example: MyPassphrase123!")
					fmt.Println()

					fmt.Print("Please Enter BIP39 Passphrase: ")
					passphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
					if err != nil {
						fmt.Printf("\nError reading passphrase: %v\n", err)
						os.Exit(1)
					}
					fmt.Print("\nPlease Re-Enter BIP39 Passphrase: ")
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
					fmt.Println("BIP39 Passphrase set successfully.")
				} else {
					fmt.Println("BIP39 Passphrase not set (using empty passphrase).")
					passphrase = ""
				}
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
				if outputLocations != "fs" && !strings.HasSuffix(path, ".json") {
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
				cloudPath := filepath.Join(util.GetWalletDir(), walletName+".json")
				result, err := util.Put(provider, walletJSON, cloudPath, force)
				if err != nil {
					fmt.Printf("Error saving wallet to %s: %v\n", provider, err)
				} else {
					fmt.Println(result)
				}
			}

			// 获取钱包地址
			addressHex, _, err := getAddressFromMnemonic(mnemonic, passphrase, wallet.DerivationPath)
			if err != nil {
				fmt.Printf("Error generating address: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("\nYour wallet address is: \033[1;32m%s\033[0m\n", addressHex)
			fmt.Println("\nBefore using this wallet, please test it with the getAddress command:")

			if len(localPaths) > 0 {
				if outputLocations == "fs" {
					fmt.Printf("  eth-cli get -i %s\n", fsPath)
				} else {
					fullPath := localPaths[0]
					if !strings.HasSuffix(fullPath, ".json") {
						fullPath = filepath.Join(fullPath, walletName+".json")
					}
					fmt.Printf("  eth-cli get -i %s\n", fullPath)
				}
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
	cmd.Flags().StringVarP(&outputLocations, "output", "o", "", "Output location: 'fs' for local file, or comma-separated list of cloud providers (supported: google, dropbox, s3, box, keychain)")
	cmd.Flags().StringVarP(&walletName, "name", "n", "", "Name of the wallet file (required except when using --output fs)")
	cmd.Flags().StringVarP(&fsPath, "path", "p", "", "File path for wallet when using --output fs")
	cmd.Flags().BoolVar(&withPassphrase, "without-passphrase", false, "Skip the BIP39 passphrase step")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite if wallet file already exists")

	cmd.MarkFlagRequired("output")

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
