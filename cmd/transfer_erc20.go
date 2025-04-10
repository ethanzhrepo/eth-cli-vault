package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

// Constants for gas estimation and token handling
const (
	DefaultGasLimitERC20  = 100000 // Default gas limit for ERC20 transfers
	GasEstimationBuffer   = 1.2    // Multiply estimated gas by this factor
	DefaultTokenDecimals  = 18     // Default decimal places for tokens
	DefaultDryRunGasPrice = 1e9    // Default gas price for dry run (1 Gwei)
	MaxSaneTokenDecimals  = 24     // Maximum reasonable number of decimals for a token
	GweiToWei             = 1e9    // Conversion factor from Gwei to Wei
	EthToWei              = 1e18   // Conversion factor from ETH to Wei
)

// ERC20Contract is the minimal interface needed for ERC20 operations
type ERC20Contract struct {
	client  *ethclient.Client
	address common.Address
}

// Symbol returns the token's symbol
func (e *ERC20Contract) Symbol(ctx context.Context) (string, error) {
	// This is a simplified version; in a real implementation, you'd use ABI binding
	callData := []byte{0x95, 0xd8, 0x9b, 0x41} // keccak256("symbol()")[:4]
	msg := ethereum.CallMsg{
		To:   &e.address,
		Data: callData,
	}
	result, err := e.client.CallContract(ctx, msg, nil)
	if err != nil {
		return "", err
	}

	// Simple parsing: Assuming result is a bytes32 string
	// In real implementation, properly decode according to ABI
	symbol := ""
	if len(result) > 32 {
		// Handle dynamic string
		offset := new(big.Int).SetBytes(result[0:32]).Int64()
		if offset < int64(len(result)) {
			length := new(big.Int).SetBytes(result[offset : offset+32]).Int64()
			if offset+32+length <= int64(len(result)) {
				symbolBytes := result[offset+32 : offset+32+length]
				symbol = string(symbolBytes)
			}
		}
	} else if len(result) > 0 {
		// Some older tokens return the symbol directly as bytes32
		// Remove trailing zeros
		i := 0
		for i < len(result) && result[i] != 0 {
			i++
		}
		symbol = string(result[:i])
	}

	return symbol, nil
}

// Decimals returns the token's decimal places
func (e *ERC20Contract) Decimals(ctx context.Context) (uint8, error) {
	// This is a simplified version; in a real implementation, you'd use ABI binding
	callData := []byte{0x31, 0x3c, 0xe5, 0x67} // keccak256("decimals()")[:4]
	msg := ethereum.CallMsg{
		To:   &e.address,
		Data: callData,
	}
	result, err := e.client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		fmt.Printf("Warning: Token contract returned empty result for decimals, using default value (18)\n")
		return 18, nil
	}

	// Extract decimal places - handle different response formats
	var decimals uint8
	if len(result) == 32 {
		// Standard uint8 response, but packed in a uint256
		decimals = uint8(new(big.Int).SetBytes(result).Uint64())
	} else if len(result) == 1 {
		// Direct uint8 response
		decimals = uint8(result[0])
	} else {
		// Try to parse as uint256 anyway and hope for the best
		decimals = uint8(new(big.Int).SetBytes(result).Uint64())
	}

	// Sanity check: Decimals usually between 0 and 24
	if decimals > MaxSaneTokenDecimals {
		fmt.Printf("Warning: Unusual decimals value: %d, using default value (%d)\n", decimals, DefaultTokenDecimals)
		return DefaultTokenDecimals, nil
	}

	return decimals, nil
}

// NewERC20Contract creates a new ERC20 contract instance
func NewERC20Contract(client *ethclient.Client, address common.Address) *ERC20Contract {
	return &ERC20Contract{
		client:  client,
		address: address,
	}
}

// TransferERC20Cmd creates the ERC20 transfer command
func TransferERC20Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transferERC20",
		Short: "Transfer ERC20 tokens to another address",
		Long:  `Transfer ERC20 tokens to another Ethereum address.`,
		RunE:  runTransferERC20,
	}

	cmd.Flags().StringP("amount", "a", "", "Amount of tokens to transfer (decimal format)")
	cmd.Flags().StringP("to", "t", "", "Destination address")
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

// setupClientAndTokenInfo sets up the client and gets token information
func setupClientAndTokenInfo(rpcURL, tokenAddress string) (*ethclient.Client, string, uint8, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to connect to Ethereum node: %v", err)
	}
	fmt.Printf("Using RPC: %s\n", rpcURL)

	// Get token info
	tokenContract := NewERC20Contract(client, common.HexToAddress(tokenAddress))

	// Get token symbol
	tokenSymbol, err := tokenContract.Symbol(context.Background())
	if err != nil {
		return client, "", 0, fmt.Errorf("failed to get token symbol: %v", err)
	}

	// Get token decimals
	tokenDecimals, err := tokenContract.Decimals(context.Background())
	if err != nil {
		return client, tokenSymbol, 0, fmt.Errorf("failed to get token decimals: %v", err)
	}

	fmt.Printf("Token Symbol: %s\n", tokenSymbol)
	fmt.Printf("Token Decimals: %d\n", tokenDecimals)

	return client, tokenSymbol, tokenDecimals, nil
}

// determineGasParameters gets gas price and estimates gas limit for an ERC20 transfer
func determineGasParameters(client *ethclient.Client, fromAddress, tokenAddress, to string, amount *big.Int, gasLimit uint64, gasPriceStr string, dryRun bool) (uint64, *big.Int, error) {
	// Get gas price
	var gasPrice *big.Int
	var err error

	if gasPriceStr != "" {
		gasPrice, err = parseEthAmount(gasPriceStr)
		if err != nil {
			return 0, nil, err
		}
	} else if !dryRun {
		gasPrice, err = client.SuggestGasPrice(context.Background())
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get suggested gas price: %v", err)
		}
		fmt.Printf("Suggested Gas Price: %s Gwei\n", new(big.Float).Quo(
			new(big.Float).SetInt(gasPrice),
			new(big.Float).SetInt(big.NewInt(GweiToWei)),
		).Text('f', 9))
	} else {
		gasPrice = big.NewInt(DefaultDryRunGasPrice) // Default 1 Gwei if dry run
	}

	// Get gas limit
	if gasLimit == 0 && !dryRun {
		fromAddr := common.HexToAddress(fromAddress)
		tokenContractAddr := common.HexToAddress(tokenAddress)
		recipientAddr := common.HexToAddress(to)

		// Prepare ERC20 transfer data
		transferFnSignature := []byte{0xa9, 0x05, 0x9c, 0xbb} // keccak256("transfer(address,uint256)")[:4]
		paddedAddress := common.LeftPadBytes(recipientAddr.Bytes(), 32)
		paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

		var data []byte
		data = append(data, transferFnSignature...)
		data = append(data, paddedAddress...)
		data = append(data, paddedAmount...)

		// Estimate gas using the proper ERC20 transfer parameters
		gasLimit, err = util.EstimateGas(client, fromAddr, &tokenContractAddr, big.NewInt(0), data)

		if err != nil {
			// Print detailed error information
			fmt.Printf("WARNING: Failed to estimate gas: %v\n", err)

			// Try to get more information by simulating the transaction
			msg := ethereum.CallMsg{
				From:  fromAddr,
				To:    &tokenContractAddr,
				Value: big.NewInt(0),
				Data:  data,
			}

			// Try to simulate the transaction to get more error details
			result, callErr := client.CallContract(context.Background(), msg, nil)
			if callErr != nil {
				fmt.Printf("ERROR: Transaction simulation details: %v\n", callErr)
				if strings.Contains(callErr.Error(), "revert") {
					revertReason := callErr.Error()
					if strings.Contains(revertReason, "execution reverted: ") {
						parts := strings.Split(revertReason, "execution reverted: ")
						if len(parts) > 1 {
							fmt.Printf("REVERT REASON: %s\n", parts[1])
						}
					}
				}
			} else if len(result) > 0 {
				fmt.Printf("ERROR: Contract call result: 0x%x\n", result)
			}

			// Check account balance
			balance, balErr := client.BalanceAt(context.Background(), fromAddr, nil)
			if balErr == nil {
				fmt.Printf("INFO: Current account balance: %s ETH\n",
					new(big.Float).Quo(
						new(big.Float).SetInt(balance),
						new(big.Float).SetInt(big.NewInt(EthToWei)),
					).Text('f', 18))
			}

			// Fall back to default gas limit
			fmt.Printf("Using default gas limit for ERC20 transfer: %d\n", DefaultGasLimitERC20)
			gasLimit = DefaultGasLimitERC20
		} else {
			// Add buffer to estimated gas
			gasLimit = uint64(float64(gasLimit) * GasEstimationBuffer)
			fmt.Printf("Estimated gas with buffer: %d\n", gasLimit)
		}
	} else if gasLimit == 0 && dryRun {
		return 0, nil, fmt.Errorf("gas limit is required when --dry-run is true")
	}

	return gasLimit, gasPrice, nil
}

// formatAndDisplayTxDetails formats and displays transaction details for user confirmation
func formatAndDisplayTxDetails(
	fromAddress, to, tokenAddress, tokenSymbol string,
	amount *big.Int, tokenDecimals uint8,
	gasLimit uint64, gasPrice *big.Int, nonce uint64) {

	// Convert amount to token units for display using the token's decimal places
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenDecimals)), nil)
	amountInt := new(big.Int).Div(amount, divisor)
	amountRemainder := new(big.Int).Mod(amount, divisor)

	// Format with the correct number of decimal places
	displayAmount := fmt.Sprintf("%d.%0*d", amountInt, tokenDecimals, amountRemainder)

	// Convert gas price to Gwei
	gasPriceGwei := new(big.Int).Div(gasPrice, big.NewInt(GweiToWei))
	gasPriceRemainder := new(big.Int).Mod(gasPrice, big.NewInt(GweiToWei))
	displayGasPrice := fmt.Sprintf("%d.%09d", gasPriceGwei, gasPriceRemainder)

	// Calculate gas fee in Wei
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	gasFeeEth := new(big.Int).Div(gasFee, big.NewInt(EthToWei))
	gasFeeRemainder := new(big.Int).Mod(gasFee, big.NewInt(EthToWei))
	displayGasFee := fmt.Sprintf("%d.%018d", gasFeeEth, gasFeeRemainder)

	fmt.Println("Transaction Details:")
	fmt.Printf("From: %s\n", fromAddress)
	fmt.Printf("To: %s\n", to)
	fmt.Printf("Token: %s (%s)\n", tokenAddress, tokenSymbol)
	fmt.Printf("Amount: %s %s\n", displayAmount, tokenSymbol)
	fmt.Printf("Gas Limit: %d\n", gasLimit)
	fmt.Printf("Gas Price: %s Gwei\n", displayGasPrice)
	fmt.Printf("Gas Fee: %s ETH\n", displayGasFee)
	fmt.Printf("Nonce: %d\n", nonce)
}

// waitForConfirmation waits for a transaction to be confirmed
func waitForConfirmation(client *ethclient.Client, txHash string) error {
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

	return nil
}

func runTransferERC20(cmd *cobra.Command, args []string) error {
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

	// Connect to Ethereum client and get token info if not dry run
	var client *ethclient.Client
	var tokenSymbol string
	var tokenDecimals uint8
	var amount *big.Int

	if !dryRun {
		var setupErr error
		client, tokenSymbol, tokenDecimals, setupErr = setupClientAndTokenInfo(rpcURL, tokenAddress)
		if setupErr != nil {
			return setupErr
		}

		// Convert amount to token units
		amount, err = util.ParseTokenAmount(amountStr, tokenDecimals)
		if err != nil {
			return fmt.Errorf("failed to parse token amount: %v", err)
		}
	} else {
		// For dry run, just use a default for the preview
		tokenSymbol = "TOKEN"
		tokenDecimals = DefaultTokenDecimals

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

		if chainIDValue != 1 && nonceValue == 0 {
			return fmt.Errorf("--nonce is required when --chain-id is specified")
		}

		nonce = nonceValue
		fmt.Printf("\033[33mWARNING: Using chain ID %d and nonce %d for dry run.\033[0m\n", chainIDValue, nonce)
	}

	// Determine gas parameters
	gasLimit, gasPrice, err := determineGasParameters(client, fromAddress, tokenAddress, to, amount, gasLimit, gasPriceStr, dryRun)
	if err != nil {
		return err
	}

	// Create raw transaction
	rawTx, err := util.CreateERC20TransferTx(
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
			new(big.Float).SetInt(big.NewInt(GweiToWei)),
		).Text('f', 9))
		fmt.Printf("Estimated Gas Fee: %s ETH\n", new(big.Float).Quo(
			new(big.Float).SetInt(new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))),
			new(big.Float).SetInt(big.NewInt(EthToWei)),
		).Text('f', 18))
		return nil
	}

	// If dry run, just display the raw transaction and exit
	if dryRun {
		fmt.Printf("Raw Transaction: %s\n", rawTx)
		return nil
	}

	// Sign the transaction
	signedTx, err := util.SignTransaction(rawTx, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Display transaction details for confirmation
	if !autoConfirm {
		formatAndDisplayTxDetails(
			fromAddress, to, tokenAddress, tokenSymbol,
			amount, tokenDecimals,
			gasLimit, gasPrice, nonce,
		)

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
	txHash, err := util.BroadcastTransaction(signedTx, rpcURL)
	if err != nil {
		return fmt.Errorf("failed to broadcast transaction: %v", err)
	}

	fmt.Printf("Transaction submitted: %s\n", txHash)

	// Wait for confirmation if requested
	if sync {
		return waitForConfirmation(client, txHash)
	}

	return nil
}
