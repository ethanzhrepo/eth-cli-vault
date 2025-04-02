package cmd

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// GasPriceCmd 返回 gas-price 命令
func GasPriceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gas-price",
		Short: "Get current gas price from the Ethereum network",
		Long:  `Retrieve the current gas price from the Ethereum network using the configured RPC endpoint.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 初始化配置
			initConfig()

			// 从配置中获取 RPC URL
			rpcURL := viper.GetString("rpc")
			if rpcURL == "" {
				fmt.Println("Error: RPC URL not configured. Please set it with 'eth-cli-wallet config set rpc YOUR_RPC_URL'")
				os.Exit(1)
			}

			// 连接以太坊客户端
			client, err := ethclient.Dial(rpcURL)
			if err != nil {
				fmt.Printf("Error connecting to Ethereum node: %v\n", err)
				os.Exit(1)
			}
			defer client.Close()

			// 获取当前 gas 价格
			gasPrice, err := client.SuggestGasPrice(context.Background())
			if err != nil {
				fmt.Printf("Error getting gas price: %v\n", err)
				os.Exit(1)
			}

			// 计算不同单位的 gas 价格
			gweiPrice := new(big.Float).Quo(
				new(big.Float).SetInt(gasPrice),
				new(big.Float).SetInt(big.NewInt(params.GWei)),
			)
			etherPrice := new(big.Float).Quo(
				new(big.Float).SetInt(gasPrice),
				new(big.Float).SetInt(big.NewInt(params.Ether)),
			)
			// 输出rpc
			fmt.Printf("RPC URL: %s\n", rpcURL)
			// 输出 gas 价格
			fmt.Printf("Current Gas Price:\n")
			fmt.Printf("  %s Wei\n", gasPrice.String())
			fmt.Printf("  %.2f Gwei\n", gweiPrice)
			fmt.Printf("  %.9f ETH\n", etherPrice)
		},
	}

	return cmd
}
