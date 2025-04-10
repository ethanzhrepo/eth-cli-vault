package cmd

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
)

// SignTxCmd creates the transaction signing command
func SignTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign-raw-tx",
		Short: "Sign an Ethereum transaction",
		Long:  `Sign an Ethereum transaction using the specified private key.`,
		RunE:  runSignTx,
	}

	cmd.Flags().String("raw-tx", "", "Raw transaction hex string to sign")
	cmd.Flags().String("raw-tx-file", "", "Path to a file containing raw transaction hex")
	cmd.Flags().StringP("provider", "p", "", "Key provider (e.g., googledrive)")
	cmd.Flags().StringP("name", "n", "", "Name of the wallet file (for cloud storage)")
	cmd.Flags().StringP("file", "f", "", "Local wallet file path")
	cmd.Flags().Bool("broadcast", false, "Broadcast the transaction after signing")

	return cmd
}

func runSignTx(cmd *cobra.Command, args []string) error {
	// Parse flags
	rawTx, _ := cmd.Flags().GetString("raw-tx")
	rawTxFile, _ := cmd.Flags().GetString("raw-tx-file")
	provider, _ := cmd.Flags().GetString("provider")
	name, _ := cmd.Flags().GetString("name")
	filePath, _ := cmd.Flags().GetString("file")
	broadcast, _ := cmd.Flags().GetBool("broadcast")

	// Check for raw transaction source
	if rawTx == "" && rawTxFile == "" {
		return fmt.Errorf("either --raw-tx or --raw-tx-file must be specified")
	}

	if rawTx != "" && rawTxFile != "" {
		return fmt.Errorf("--raw-tx and --raw-tx-file are mutually exclusive, use one or the other")
	}

	// Get raw transaction from file if necessary
	var rawTxHex string
	if rawTxFile != "" {
		var err error
		data, err := util.LoadFromFileSystem(rawTxFile)
		if err != nil {
			return fmt.Errorf("failed to read raw transaction file: %v", err)
		}
		// Trim any whitespace or newlines
		rawTxHex = strings.TrimSpace(string(data))
	} else {
		rawTxHex = rawTx
	}

	// Check mutual exclusivity between provider+name and file
	if (provider != "" || name != "") && filePath != "" {
		return fmt.Errorf("--file and --provider/--name are mutually exclusive, use one or the other")
	}

	// Ensure we have either file or provider
	if provider == "" && filePath == "" {
		return fmt.Errorf("either --provider or --file must be specified")
	}

	// Get RPC URL from config if needed for broadcasting
	var rpcURL string
	var err error
	if broadcast {
		rpcURL, err = initTxConfig()
		if err != nil {
			return err
		}
	}

	// Print provider or file info
	if provider != "" {
		fmt.Printf("Using provider: %s\n", provider)
	} else {
		fmt.Printf("Using wallet file: %s\n", filePath)
	}

	// Get private key from provider or file
	var privateKey string
	var fromAddress string
	if filePath != "" {
		// Use local file
		privateKey, fromAddress, err = getPrivateKeyFromLocalFile(filePath)
	} else {
		// Use provider
		privateKey, fromAddress, err = getPrivateKeyFromProvider(provider, name)
	}
	if err != nil {
		return fmt.Errorf("failed to get private key: %v", err)
	}

	// Sign the transaction
	var signErr error
	signedTx, signErr := util.SignTransaction(rawTxHex, privateKey)
	if signErr != nil {
		return fmt.Errorf("failed to sign transaction: %v", signErr)
	}

	// If broadcast flag is set, broadcast the transaction
	if broadcast {
		// Check if RPC URL is configured
		if rpcURL == "" {
			return fmt.Errorf("RPC URL is required for broadcasting")
		}

		// Try to extract gas details from the raw transaction
		txDetails := "Transaction Details:\n"
		txDetails += fmt.Sprintf("From: %s\n", fromAddress)

		// Try to decode the transaction to extract gas details
		txData, decodeErr := hexutil.Decode(rawTxHex)
		if decodeErr == nil {
			var tx types.Transaction
			unmarshalErr := tx.UnmarshalBinary(txData)
			if unmarshalErr == nil {
				// Display to address if available
				if tx.To() != nil {
					txDetails += fmt.Sprintf("To: %s\n", tx.To().Hex())
				}

				// Display value if it's not zero
				if tx.Value().Cmp(big.NewInt(0)) > 0 {
					ethAmount := new(big.Int).Div(tx.Value(), big.NewInt(1e18))
					remainder := new(big.Int).Mod(tx.Value(), big.NewInt(1e18))
					displayAmount := fmt.Sprintf("%d.%018d", ethAmount, remainder)
					txDetails += fmt.Sprintf("Value: %s ETH\n", displayAmount)
				}

				// Display gas limit
				txDetails += fmt.Sprintf("Gas Limit: %d\n", tx.Gas())

				// Display gas price if available
				gasPrice := tx.GasPrice()
				if gasPrice != nil && gasPrice.Cmp(big.NewInt(0)) > 0 {
					gasPriceGwei := new(big.Int).Div(gasPrice, big.NewInt(1e9))
					gasPriceRemainder := new(big.Int).Mod(gasPrice, big.NewInt(1e9))
					displayGasPrice := fmt.Sprintf("%d.%09d", gasPriceGwei, gasPriceRemainder)
					txDetails += fmt.Sprintf("Gas Price: %s Gwei\n", displayGasPrice)

					// Calculate and display gas fee
					gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(tx.Gas())))
					gasFeeEth := new(big.Int).Div(gasFee, big.NewInt(1e18))
					gasFeeRemainder := new(big.Int).Mod(gasFee, big.NewInt(1e18))
					displayGasFee := fmt.Sprintf("%d.%018d", gasFeeEth, gasFeeRemainder)
					txDetails += fmt.Sprintf("Gas Fee: %s ETH\n", displayGasFee)
				}

				// Display nonce
				txDetails += fmt.Sprintf("Nonce: %d\n", tx.Nonce())
			}
		}

		// Display truncated signed transaction
		txDetails += fmt.Sprintf("Signed Transaction: %s...\n", signedTx[:66]+"...")

		// Display transaction details and ask for confirmation
		fmt.Println(txDetails)
		fmt.Print("Broadcast this transaction? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if !strings.EqualFold(response, "y") {
			fmt.Println("Transaction broadcasting cancelled.")
			return nil
		}

		// Broadcast the transaction
		var broadcastErr error
		txHash, broadcastErr := util.BroadcastTransaction(signedTx, rpcURL)
		if broadcastErr != nil {
			return fmt.Errorf("failed to broadcast transaction: %v", broadcastErr)
		}

		fmt.Printf("Transaction submitted: %s\n", txHash)
	} else {
		// Just display the signed transaction
		fmt.Printf("Signed Transaction: %s\n", signedTx)
	}

	return nil
}
