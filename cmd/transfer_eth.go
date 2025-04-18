package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

// Conversion factors for different units
var (
	weiPerEth  = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil) // 10^18
	weiPerGwei = new(big.Int).Exp(big.NewInt(10), big.NewInt(9), nil)  // 10^9
	weiPerWei  = big.NewInt(1)                                         // 10^0
)

// TransferETHCmd creates the ETH transfer command
func TransferETHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Transfer ETH to another address",
		Long:  `Transfer ETH to another Ethereum address.`,
		RunE:  runTransferETH,
	}

	cmd.Flags().StringP("amount", "a", "", "Amount of ETH to transfer (with unit e.g., 1.0eth, 10gwei)")
	cmd.Flags().StringP("to", "t", "", "Destination address")
	cmd.Flags().StringP("provider", "p", "", "Key provider (e.g., google)")
	cmd.Flags().StringP("name", "n", "", "Name of the wallet file (for cloud storage)")
	cmd.Flags().StringP("file", "f", "", "Local wallet file path")
	cmd.Flags().Bool("dry-run", false, "Only encode the transaction, do not broadcast")
	cmd.Flags().Bool("estimate-only", false, "Only display gas estimation")
	cmd.Flags().BoolP("yes", "y", false, "Automatically confirm the transaction")
	cmd.Flags().String("gas-price", "", "Gas price (e.g., 3gwei)")
	cmd.Flags().Uint64("gas-limit", 0, "Gas limit")
	cmd.Flags().Uint64("chain-id", 1, "Chain ID to use in dry-run mode (default: 1)")
	cmd.Flags().Uint64("nonce", 0, "Nonce to use in dry-run mode (required when chain-id is specified)")
	cmd.Flags().Bool("sync", false, "Wait for transaction confirmation")

	cmd.MarkFlagRequired("amount")
	cmd.MarkFlagRequired("to")

	return cmd
}

func runTransferETH(cmd *cobra.Command, args []string) error {
	// Parse flags
	amountStr, _ := cmd.Flags().GetString("amount")
	to, _ := cmd.Flags().GetString("to")
	provider, _ := cmd.Flags().GetString("provider")
	name, _ := cmd.Flags().GetString("name")
	filePath, _ := cmd.Flags().GetString("file")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	estimateOnly, _ := cmd.Flags().GetBool("estimate-only")
	autoConfirm, _ := cmd.Flags().GetBool("yes")
	gasPriceStr, _ := cmd.Flags().GetString("gas-price")
	gasLimit, _ := cmd.Flags().GetUint64("gas-limit")
	sync, _ := cmd.Flags().GetBool("sync")

	// Check mutual exclusivity between provider+name and file
	if (provider != "" || name != "") && filePath != "" {
		return fmt.Errorf("--file and --provider/--name are mutually exclusive, use one or the other")
	}

	// Ensure we have either file or provider
	if provider == "" && filePath == "" {
		return fmt.Errorf("either --provider or --file must be specified")
	}

	// Get RPC URL from config
	rpcURL, err := initTxConfig()
	if err != nil && !dryRun {
		return err
	}

	// Print provider or file info
	if provider != "" {
		fmt.Printf("Using provider: %s\n", provider)
	} else {
		fmt.Printf("Using wallet file: %s\n", filePath)
	}

	// Parse amount with unit
	amountInWei, err := parseEthAmount(amountStr)
	if err != nil {
		return err
	}

	// Check if we need RPC
	if !dryRun {
		if rpcURL == "" {
			return fmt.Errorf("RPC URL is required when not using --dry-run")
		}
	}

	// Connect to Ethereum client if needed
	var client *ethclient.Client
	if !dryRun {
		var dialErr error
		client, dialErr = ethclient.Dial(rpcURL)
		if dialErr != nil {
			return fmt.Errorf("failed to connect to Ethereum node: %v", dialErr)
		}
		fmt.Printf("Using RPC: %s\n", rpcURL)
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

	// Get chain ID and nonce
	var chainID *big.Int
	var nonce uint64
	if !dryRun {
		var chainErr error
		chainID, chainErr = client.NetworkID(context.Background())
		if chainErr != nil {
			return fmt.Errorf("failed to get chain ID: %v", chainErr)
		}
		fromAddr := common.HexToAddress(fromAddress)
		nonce, err = util.GetNonce(client, fromAddr)
		if err != nil {
			return fmt.Errorf("failed to get nonce: %v", err)
		}
	} else {
		chainIDValue, _ := cmd.Flags().GetUint64("chain-id")
		chainID = big.NewInt(int64(chainIDValue))
		nonceValue, _ := cmd.Flags().GetUint64("nonce")

		if chainIDValue != 1 && nonceValue == 0 {
			return fmt.Errorf("--nonce is required when --chain-id is specified")
		}

		nonce = nonceValue
		fmt.Printf("\033[33mWARNING: Using chain ID %d and nonce %d for dry run.\033[0m\n", chainIDValue, nonce)
	}

	// Get gas price
	var gasPrice *big.Int
	if gasPriceStr != "" {
		var gasPriceErr error
		gasPrice, gasPriceErr = parseEthAmount(gasPriceStr)
		if gasPriceErr != nil {
			return gasPriceErr
		}
	} else if !dryRun {
		var suggestErr error
		gasPrice, suggestErr = client.SuggestGasPrice(context.Background())
		if suggestErr != nil {
			return fmt.Errorf("failed to get suggested gas price: %v", suggestErr)
		}
	} else {
		gasPrice = big.NewInt(1000000000) // Default 1 Gwei if dry run
	}

	// Get gas limit
	if gasLimit == 0 && !dryRun {
		fromAddr := common.HexToAddress(fromAddress)
		toAddr := common.HexToAddress(to)
		var gasEstimateErr error
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &toAddr, amountInWei, nil)
		if gasEstimateErr != nil {
			return fmt.Errorf("failed to estimate gas: %v", gasEstimateErr)
		}
	} else if gasLimit == 0 {
		gasLimit = 21000 // Default gas limit for ETH transfers
	}

	// Create raw transaction
	rawTx, err := util.CreateEthTransferTx(
		fromAddress,
		to,
		amountInWei,
		nonce,
		gasPrice,
		gasLimit,
		chainID,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %v", err)
	}

	// If gas only, just display and exit
	if estimateOnly {
		fmt.Printf("Estimated Gas Limit: %d\n", gasLimit)
		fmt.Printf("Suggested Gas Price: %s Gwei\n", new(big.Float).Quo(
			new(big.Float).SetInt(gasPrice),
			new(big.Float).SetInt(big.NewInt(1000000000)),
		).Text('f', 9))
		fmt.Printf("Estimated Gas Fee: %s ETH\n", new(big.Float).Quo(
			new(big.Float).SetInt(new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))),
			new(big.Float).SetInt(big.NewInt(1000000000000000000)),
		).Text('f', 18))
		return nil
	}

	// If dry run, just display the raw transaction and exit
	if dryRun {
		displayTransactionDetails(fromAddress, to, amountInWei, gasLimit, gasPrice, nil, nonce, chainID, true)
		fmt.Printf("\n\033[1;36mRaw Transaction:\033[0m %s\n", rawTx)
		return nil
	}

	// Sign the transaction
	var signErr error
	signedTx, signErr := util.SignTransaction(rawTx, privateKey)
	if signErr != nil {
		return fmt.Errorf("failed to sign transaction: %v", signErr)
	}

	// Display transaction details for confirmation
	if !autoConfirm {
		displayTransactionDetails(fromAddress, to, amountInWei, gasLimit, gasPrice, nil, nonce, chainID, false)

		// Ask for confirmation
		fmt.Print("Confirm transaction? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if !strings.EqualFold(response, "y") {
			fmt.Println("Transaction cancelled.")
			return nil
		}
	}

	// Broadcast the transaction
	var broadcastErr error
	txHash, broadcastErr := util.BroadcastTransaction(signedTx, rpcURL)
	if broadcastErr != nil {
		return fmt.Errorf("failed to broadcast transaction: %v", broadcastErr)
	}

	fmt.Printf("Transaction submitted: %s\n", txHash)

	// Wait for confirmation if requested
	if sync {
		fmt.Println("Waiting for transaction confirmation...")

		// Wait for transaction to be mined
		var receipt *types.Receipt
		for {
			var receiptErr error
			receipt, receiptErr = client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
			if receiptErr == nil {
				break
			}
			if receiptErr != nil && receiptErr.Error() != "not found" {
				return fmt.Errorf("failed to get transaction receipt: %v", receiptErr)
			}
			time.Sleep(2 * time.Second)
		}

		if receipt.Status == 1 {
			fmt.Println("Transaction confirmed successfully!")
		} else {
			fmt.Println("Transaction failed!")
		}
		fmt.Printf("Block Number: %d\n", receipt.BlockNumber)
		fmt.Printf("Gas Used: %d\n", receipt.GasUsed)
	}

	return nil
}

// displayTransactionDetails formats and displays transaction details
func displayTransactionDetails(from, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, gasFeePredefined *big.Int, nonce uint64, chainID *big.Int, colorize bool) {
	// Convert Wei to ETH for display using big.Int
	ethAmount := new(big.Int).Div(amount, big.NewInt(1e18))
	remainder := new(big.Int).Mod(amount, big.NewInt(1e18))
	displayAmount := fmt.Sprintf("%d.%018d", ethAmount, remainder)

	// Convert gas price to Gwei
	gasPriceGwei := new(big.Int).Div(gasPrice, big.NewInt(1e9))
	gasPriceRemainder := new(big.Int).Mod(gasPrice, big.NewInt(1e9))
	displayGasPrice := fmt.Sprintf("%d.%09d", gasPriceGwei, gasPriceRemainder)

	// Calculate gas fee in Wei if not provided
	var gasFee *big.Int
	if gasFeePredefined != nil {
		gasFee = gasFeePredefined
	} else {
		gasFee = new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	}
	gasFeeEth := new(big.Int).Div(gasFee, big.NewInt(1e18))
	gasFeeRemainder := new(big.Int).Mod(gasFee, big.NewInt(1e18))
	displayGasFee := fmt.Sprintf("%d.%018d", gasFeeEth, gasFeeRemainder)

	if colorize {
		fmt.Println("\033[1;36mTransaction Details:\033[0m")
		fmt.Printf("\033[1;33mFrom:\033[0m %s\n", from)
		fmt.Printf("\033[1;33mTo:\033[0m %s\n", to)
		fmt.Printf("\033[1;33mAmount:\033[0m \033[1;32m%s ETH\033[0m\n", displayAmount)
		fmt.Printf("\033[1;33mGas Limit:\033[0m %d\n", gasLimit)
		fmt.Printf("\033[1;33mGas Price:\033[0m %s Gwei\n", displayGasPrice)
		fmt.Printf("\033[1;33mGas Fee:\033[0m %s ETH\n", displayGasFee)
		fmt.Printf("\033[1;33mNonce:\033[0m %d\n", nonce)
		fmt.Printf("\033[1;33mChain ID:\033[0m %d\n", chainID)
	} else {
		fmt.Println("Transaction Details:")
		fmt.Printf("From: %s\n", from)
		fmt.Printf("To: %s\n", to)
		fmt.Printf("Amount: %s ETH\n", displayAmount)
		fmt.Printf("Gas Limit: %d\n", gasLimit)
		fmt.Printf("Gas Price: %s Gwei\n", displayGasPrice)
		fmt.Printf("Gas Fee: %s ETH\n", displayGasFee)
		fmt.Printf("Nonce: %d\n", nonce)
	}
}

// parseEthAmount parses ETH amount with units (e.g., "1.0eth", "10gwei")
func parseEthAmount(amount string) (*big.Int, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return nil, fmt.Errorf("amount cannot be empty")
	}

	// Default unit is wei if no unit specified
	unit := "wei"
	value := amount

	// Check for unit in the string
	lowerAmount := strings.ToLower(amount)
	for _, u := range []string{"eth", "gwei", "wei"} {
		if strings.HasSuffix(lowerAmount, u) {
			unit = u
			// Remove the unit suffix (case insensitive)
			value = amount[:len(amount)-len(u)]
			value = strings.TrimSpace(value)
			break
		}
	}

	// Split into integer and decimal parts
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid amount format: %s", amount)
	}

	// Parse integer part
	intPart := parts[0]
	if intPart == "" {
		intPart = "0"
	}
	intVal := new(big.Int)
	if _, ok := intVal.SetString(intPart, 10); !ok {
		return nil, fmt.Errorf("invalid integer part: %s", intPart)
	}

	// Get the appropriate multiplier based on unit
	var multiplier *big.Int
	switch unit {
	case "eth":
		multiplier = weiPerEth
	case "gwei":
		multiplier = weiPerGwei
	case "wei":
		multiplier = weiPerWei
	default:
		return nil, fmt.Errorf("unsupported unit: %s", unit)
	}

	// Multiply integer part by the multiplier
	result := new(big.Int).Mul(intVal, multiplier)

	// If there's a decimal part, handle it
	if len(parts) == 2 {
		decimalPart := parts[1]
		if decimalPart != "" {
			// Calculate the decimal multiplier (10^decimalPlaces)
			decimalPlaces := len(decimalPart)
			decimalMultiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimalPlaces)), nil)

			// Parse decimal part
			decimalVal := new(big.Int)
			if _, ok := decimalVal.SetString(decimalPart, 10); !ok {
				return nil, fmt.Errorf("invalid decimal part: %s", decimalPart)
			}

			// Calculate the decimal contribution
			// (decimalVal * multiplier) / decimalMultiplier
			decimalContribution := new(big.Int).Mul(decimalVal, multiplier)
			decimalContribution.Div(decimalContribution, decimalMultiplier)

			// Add decimal contribution to result
			result.Add(result, decimalContribution)
		}
	}

	// Check for negative values
	if result.Sign() < 0 {
		return nil, fmt.Errorf("amount cannot be negative: %s", amount)
	}

	return result, nil
}
