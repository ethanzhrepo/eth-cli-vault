package cmd

import (
	"fmt"
	"strings"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
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

		// Ask for confirmation
		fmt.Println("Transaction Details:")
		fmt.Printf("From: %s\n", fromAddress)
		fmt.Printf("Signed Transaction: %s...\n", signedTx[:66]+"...")
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
