package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/term"
)

// CreateSpecialCmd 返回 create-special 命令，用于生成靓号地址
func CreateSpecialCmd() *cobra.Command {
	var outputLocations string
	var walletName string
	var force bool
	var fsPath string
	var pattern string
	var displayMnemonic bool

	cmd := &cobra.Command{
		Use:   "create-special",
		Short: "Create a new Ethereum wallet with vanity address",
		Long: `Create a new Ethereum wallet with vanity address that matches a specific pattern.

The pattern parameter accepts regular expressions to match desired address formats.
This command will generate wallets until it finds an address matching your pattern.

Examples:
  eth-cli create-special --pattern "^0x999[a-fA-F0-9]+999$" --output fs --path /tmp/wallet.json
  eth-cli create-special --pattern "^0x999999[a-fA-F0-9]+999999$" --output google,dropbox --name myVanityWallet
  eth-cli create-special --pattern "^0x[aA]+[0-9]{10}" --output google,dropbox --name myVanityWallet

Warning: Generating vanity addresses can take a very long time depending on the complexity of your pattern.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 初始化配置
			initConfig()

			// 检查必要参数
			if pattern == "" {
				fmt.Println("Error: --pattern parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			if outputLocations == "" {
				fmt.Println("Error: --output parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			// 验证正则表达式
			regex, err := regexp.Compile(pattern)
			if err != nil {
				fmt.Printf("Error: Invalid regex pattern: %v\n", err)
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

			// 开始生成靓号地址
			fmt.Printf("\n\033[1;33mSearching for vanity address matching pattern: %s\033[0m\n", pattern)
			fmt.Println("This may take a while depending on the complexity of your pattern...")
			fmt.Println("\033[1;31mNote: Passphrase will be set to empty for vanity address generation to ensure address consistency.\033[0m")
			fmt.Println("Press Ctrl+C to cancel at any time.\n")

			var mnemonic string
			var addressHex string
			attempts := 0

			for {
				attempts++

				// 生成BIP39助记词
				entropy, err := bip39.NewEntropy(256) // 生成256位熵，对应24个单词
				if err != nil {
					fmt.Printf("Error generating entropy: %v\n", err)
					continue
				}

				tempMnemonic, err := bip39.NewMnemonic(entropy)
				if err != nil {
					fmt.Printf("Error generating mnemonic: %v\n", err)
					continue
				}

				// 生成地址（使用空的passphrase进行初步检查）
				tempAddressHex, _, err := getAddressFromMnemonic(tempMnemonic, "", "m/44'/60'/0'/0/0")
				if err != nil {
					continue
				}

				// 实时显示当前地址和尝试次数
				fmt.Printf("\rTrying address %d: %s", attempts, tempAddressHex)

				// 检查是否匹配pattern
				if regex.MatchString(tempAddressHex) {
					mnemonic = tempMnemonic
					addressHex = tempAddressHex
					break
				}
			}

			fmt.Printf("\n\n\033[1;32m🎉 Found matching address after %d attempts!\033[0m\n", attempts)
			fmt.Printf("Address: \033[1;32m%s\033[0m\n", addressHex)

			// 如果启用了显示助记词选项，则显示助记词
			if displayMnemonic {
				fmt.Printf("Mnemonic: \033[1;33m%s\033[0m\n", mnemonic)
			}
			fmt.Println()

			// 询问用户是否使用这个地址
			fmt.Print("Do you want to use this address? (Y/n): ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading response: %v\n", err)
				os.Exit(1)
			}
			response = strings.TrimSpace(strings.ToLower(response))

			// 默认为yes，只有明确输入no或n才取消
			if response == "n" || response == "no" {
				fmt.Println("Address generation cancelled.")
				os.Exit(0)
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

			// 设置空的passphrase以确保地址一致性
			var passphrase string = ""
			fmt.Println("\n\033[1;33mUsing empty passphrase for vanity address generation to ensure address consistency.\033[0m")

			// 重新生成地址以确保使用用户提供的passphrase
			finalAddressHex, _, err := getAddressFromMnemonic(mnemonic, passphrase, "m/44'/60'/0'/0/0")
			if err != nil {
				fmt.Printf("Error generating final address: %v\n", err)
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

			fmt.Printf("\nYour vanity wallet address is: \033[1;32m%s\033[0m\n", finalAddressHex)
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
			fmt.Printf("\n\033[1;32mSuccess: Vanity wallet created successfully after %d attempts.\033[0m\n", attempts)
		},
	}

	// 添加命令参数
	cmd.Flags().StringVar(&pattern, "pattern", "", "Regular expression pattern for vanity address (required)")
	cmd.Flags().StringVarP(&outputLocations, "output", "o", "", "Output location: 'fs' for local file, or comma-separated list of cloud providers (supported: google, dropbox, s3, box, keychain)")
	cmd.Flags().StringVarP(&walletName, "name", "n", "", "Name of the wallet file (required except when using --output fs)")
	cmd.Flags().StringVarP(&fsPath, "path", "p", "", "File path for wallet when using --output fs")
	cmd.Flags().BoolVar(&displayMnemonic, "display-mnemonic", false, "Display the mnemonic phrase when a matching address is found")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force overwrite if wallet file already exists")

	cmd.MarkFlagRequired("pattern")
	cmd.MarkFlagRequired("output")

	return cmd
}
