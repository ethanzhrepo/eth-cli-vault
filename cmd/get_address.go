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
	"golang.org/x/term"
)

// GetAddressCmd 返回 getAddress 命令
func GetAddressCmd() *cobra.Command {
	var inputLocation string
	var walletName string
	var showMnemonics bool
	var showPrivateKey bool

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get the Ethereum address from a wallet file",
		Long:  `Retrieve the Ethereum address from a local or cloud-stored wallet file.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 初始化配置
			initConfig()

			// 检查必要参数
			if inputLocation == "" {
				fmt.Println("Error: --input parameter is required")
				cmd.Usage()
				os.Exit(1)
			}

			// 判断输入位置是云存储还是本地文件
			var walletData []byte
			var err error
			isCloudProvider := false

			for _, provider := range util.CLOUD_PROVIDERS {
				if inputLocation == provider {
					isCloudProvider = true
					// 从云存储获取钱包文件
					if walletName == "" {
						fmt.Println("Error: --name parameter is required when using cloud storage")
						cmd.Usage()
						os.Exit(1)
					}

					cloudPath := filepath.Join(util.GetWalletDir(), walletName+".json")
					walletData, err = util.Get(provider, cloudPath)
					if err != nil {
						fmt.Printf("Error loading wallet from %s: %v\n", provider, err)
						os.Exit(1)
					}
					break
				}
			}

			if !isCloudProvider {
				// 从本地文件系统加载
				walletData, err = util.Get(inputLocation, inputLocation)
				if err != nil {
					fmt.Printf("Error loading wallet from local file: %v\n", err)
					os.Exit(1)
				}
			}

			// 解析钱包文件
			var wallet WalletFile
			if err := json.Unmarshal(walletData, &wallet); err != nil {
				fmt.Printf("Error parsing wallet file: %v\n", err)
				os.Exit(1)
			}

			// 获取密码
			fmt.Print("Please Enter \033[1;31mAES\033[0m Password: ")
			passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				fmt.Printf("\nError reading password: %v\n", err)
				os.Exit(1)
			}
			fmt.Println()
			password := string(passwordBytes)

			// 解密助记词
			mnemonic, err := util.DecryptMnemonic(wallet.EncryptedMnemonic, password)
			if err != nil {
				fmt.Printf("Error decrypting mnemonic: %v\n", err)
				os.Exit(1)
			}

			// 显示助记词
			if showMnemonics {
				fmt.Printf("Decrypted Mnemonic: \033[1;32m%s\033[0m\n", mnemonic)
				fmt.Printf("HD Path: \033[1;32m%s\033[0m\n", wallet.HDPath)
				fmt.Printf("Derivation Path: \033[1;32m%s\033[0m\n", wallet.DerivationPath)
			}

			// 询问是否使用了passphrase
			fmt.Print("Did you use a BIP39 passphrase for this wallet? (y/n): ")
			var answer string
			fmt.Scanln(&answer)

			var passphrase string
			if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
				fmt.Print("Please Enter \033[1;31mBIP39\033[0m Passphrase: ")
				passphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					fmt.Printf("\nError reading passphrase: %v\n", err)
					os.Exit(1)
				}
				fmt.Println()
				passphrase = string(passphraseBytes)
			}

			// 使用共用函数获取地址和私钥
			addressHex, privateKeyBytes, err := getAddressFromMnemonic(mnemonic, passphrase)
			if err != nil {
				fmt.Printf("Error generating address: %v\n", err)
				os.Exit(1)
			}

			// 输出地址
			fmt.Printf("Wallet Address: \033[1;32m%s\033[0m\n", addressHex)

			// 如果开启显示私钥参数，则输出私钥
			if showPrivateKey {
				privateKeyHex := fmt.Sprintf("%x", privateKeyBytes)
				fmt.Printf("Private Key: \033[1;31m%s\033[0m\n", privateKeyHex)
			}
		},
	}

	// 添加命令参数
	cmd.Flags().StringVarP(&inputLocation, "input", "i", "", "Input location (local file path or cloud provider)")
	cmd.Flags().StringVarP(&walletName, "name", "n", "", "Name of the wallet file (required for cloud storage)")
	cmd.Flags().BoolVar(&showMnemonics, "show-mnemonics", false, "Display the decrypted mnemonic phrase")
	cmd.Flags().BoolVar(&showPrivateKey, "show-private-key", false, "Display the hex-encoded private key")

	cmd.MarkFlagRequired("input")

	return cmd
}
