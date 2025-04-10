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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

// ApproveERC20Cmd creates the ERC20 approve command
func ApproveERC20Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approveERC20",
		Short: "Approve ERC20 tokens for another address",
		Long:  `Approve ERC20 tokens to be spent by another Ethereum address.`,
		RunE:  runApproveERC20,
	}

	cmd.Flags().StringP("amount", "a", "", "Amount of tokens to approve (decimal format)")
	cmd.Flags().StringP("to", "t", "", "Spender address")
	cmd.Flags().String("token", "", "ERC20 token contract address")
	cmd.Flags().StringP("provider", "p", "", "Key provider (e.g., googledrive)")
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
	cmd.MarkFlagRequired("token")

	return cmd
}

func runApproveERC20(cmd *cobra.Command, args []string) error {
	// Parse flags
	amountStr, _ := cmd.Flags().GetString("amount")
	to, _ := cmd.Flags().GetString("to")
	tokenAddress, _ := cmd.Flags().GetString("token")
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

	// Check if we need RPC
	if !dryRun {
		if rpcURL == "" {
			return fmt.Errorf("RPC URL is required when not using --dry-run")
		}
	}

	// Connect to Ethereum client if needed
	var client *ethclient.Client
	var tokenSymbol string
	var tokenDecimals uint8
	var amount *big.Int

	if !dryRun {
		var dialErr error
		client, dialErr = ethclient.Dial(rpcURL)
		if dialErr != nil {
			return fmt.Errorf("failed to connect to Ethereum node: %v", dialErr)
		}
		fmt.Printf("Using RPC: %s\n", rpcURL)

		// Get token info
		tokenContract := NewERC20Contract(client, common.HexToAddress(tokenAddress))

		// Get token symbol
		var symbolErr error
		tokenSymbol, symbolErr = tokenContract.Symbol(context.Background())
		if symbolErr != nil {
			return fmt.Errorf("failed to get token symbol: %v", symbolErr)
		}

		// Get token decimals
		var decimalsErr error
		tokenDecimals, decimalsErr = tokenContract.Decimals(context.Background())
		if decimalsErr != nil {
			return fmt.Errorf("failed to get token decimals: %v", decimalsErr)
		}

		// Convert amount to token units
		amount, err = util.ParseTokenAmount(amountStr, tokenDecimals)
		if err != nil {
			return fmt.Errorf("failed to parse token amount: %v", err)
		}
	} else {
		// For dry run, just use a default for the preview
		tokenSymbol = "TOKEN"
		tokenDecimals = 18

		// Parse amount directly
		amount, err = util.ParseTokenAmount(amountStr, tokenDecimals)
		if err != nil {
			return fmt.Errorf("failed to parse token amount: %v", err)
		}
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

		if dryRun && nonceValue == 0 {
			return fmt.Errorf("--nonce is required when using --dry-run")
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
		contractAddr := common.HexToAddress(tokenAddress)
		spenderAddr := common.HexToAddress(to)

		// Create ERC20 approve function call data
		approveFnSignature := crypto.Keccak256Hash([]byte(util.ERC20ApproveSignature)).Bytes()[:4]
		paddedAddress := common.LeftPadBytes(spenderAddr.Bytes(), 32)
		paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

		// Combine data
		var data []byte
		data = append(data, approveFnSignature...)
		data = append(data, paddedAddress...)
		data = append(data, paddedAmount...)

		var gasEstimateErr error
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &contractAddr, nil, data)
		if gasEstimateErr != nil {
			fmt.Printf("WARNING: Failed to estimate gas: %v\n", gasEstimateErr)
			fmt.Printf("Using default gas limit for ERC20 approval\n")
			gasLimit = 100000 // Default gas limit for ERC20 approvals
		} else {
			// Add a small buffer to ensure transaction success
			gasLimit = uint64(float64(gasLimit) * 1.1)
		}
	} else if gasLimit == 0 {
		gasLimit = 100000 // Default gas limit for ERC20 approvals
	}

	// Create raw transaction
	rawTx, err := util.CreateERC20ApproveTx(
		fromAddress,
		tokenAddress,
		to,
		amount,
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
		fmt.Printf("Raw Transaction: %s\n", rawTx)
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
		// Convert amount to token units for display
		decimalDivisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenDecimals)), nil)
		amountInt := new(big.Int).Div(amount, decimalDivisor)
		amountRemainder := new(big.Int).Mod(amount, decimalDivisor)
		displayAmount := fmt.Sprintf("%d.%0*d", amountInt, tokenDecimals, amountRemainder)

		// Convert gas price to Gwei
		gasPriceGwei := new(big.Int).Div(gasPrice, big.NewInt(1e9))
		gasPriceRemainder := new(big.Int).Mod(gasPrice, big.NewInt(1e9))
		displayGasPrice := fmt.Sprintf("%d.%09d", gasPriceGwei, gasPriceRemainder)

		// Calculate gas fee in Wei
		gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
		gasFeeEth := new(big.Int).Div(gasFee, big.NewInt(1e18))
		gasFeeRemainder := new(big.Int).Mod(gasFee, big.NewInt(1e18))
		displayGasFee := fmt.Sprintf("%d.%018d", gasFeeEth, gasFeeRemainder)

		approveType := "Approval"
		if amount.Cmp(big.NewInt(0)) == 0 {
			approveType = "Revocation of approval"
		}

		fmt.Println("Transaction Details:")
		fmt.Printf("Type: %s\n", approveType)
		fmt.Printf("From: %s\n", fromAddress)
		fmt.Printf("Spender: %s\n", to)
		fmt.Printf("Token: %s (%s)\n", tokenAddress, tokenSymbol)
		fmt.Printf("Amount: %s %s\n", displayAmount, tokenSymbol)
		fmt.Printf("Gas Limit: %d\n", gasLimit)
		fmt.Printf("Gas Price: %s Gwei\n", displayGasPrice)
		fmt.Printf("Gas Fee: %s ETH\n", displayGasFee)
		fmt.Printf("Nonce: %d\n", nonce)

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
