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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
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
		offset := new(big.Int).SetBytes(result[0:32]).Int64()
		if offset < int64(len(result)) {
			length := new(big.Int).SetBytes(result[offset : offset+32]).Int64()
			if offset+32+length <= int64(len(result)) {
				symbolBytes := result[offset+32 : offset+32+length]
				symbol = string(symbolBytes)
			}
		}
	} else if len(result) > 0 {
		// Some tokens return the symbol directly
		symbol = string(result)
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
		return 18, nil // Default to 18 if not specified
	}

	// Extract decimal places
	decimals := uint8(new(big.Int).SetBytes(result).Uint64())
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

		fmt.Printf("Token Symbol: %s\n", tokenSymbol)
		fmt.Printf("Token Decimals: %d\n", tokenDecimals)

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
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &toAddr, amount, nil)
		if gasEstimateErr != nil {
			return fmt.Errorf("failed to estimate gas: %v", gasEstimateErr)
		}
	} else if gasLimit == 0 {
		gasLimit = 100000 // Default gas limit for ERC20 transfers
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

	// Estimate gas if needed
	if gasLimit == 0 && !dryRun {
		// Decode the transaction to get tx data
		txData, decodeErr := hexutil.Decode(rawTx)
		if decodeErr != nil {
			return fmt.Errorf("decode transaction failed: %v", decodeErr)
		}

		var tx types.Transaction
		unmarshalErr := tx.UnmarshalBinary(txData)
		if unmarshalErr != nil {
			return fmt.Errorf("unmarshal transaction failed: %v", unmarshalErr)
		}

		fromAddr := common.HexToAddress(fromAddress)
		toAddr := *tx.To()
		gasEstimateErr := error(nil)
		fmt.Printf("From Address: %s\n", fromAddr.Hex())
		fmt.Printf("To Address: %s\n", toAddr.Hex())
		fmt.Printf("Value: %s\n", tx.Value().String())
		fmt.Printf("Data: %s\n", hexutil.Encode(tx.Data()))
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &toAddr, tx.Value(), tx.Data())
		if gasEstimateErr != nil {
			// Print detailed error information to console
			fmt.Printf("ERROR: Failed to estimate gas: %v\n", gasEstimateErr)

			// Try to get more information about the error
			msg := ethereum.CallMsg{
				From:  fromAddr,
				To:    &toAddr,
				Value: tx.Value(),
				Data:  tx.Data(),
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

			// If balance is insufficient, report it
			balance, balErr := client.BalanceAt(context.Background(), fromAddr, nil)
			if balErr == nil {
				fmt.Printf("INFO: Current account balance: %s ETH\n",
					new(big.Float).Quo(
						new(big.Float).SetInt(balance),
						new(big.Float).SetInt(big.NewInt(1000000000000000000)),
					).Text('f', 18))
			}

			return fmt.Errorf("failed to estimate gas: %v", gasEstimateErr)
		}

		// Recreate the transaction with the estimated gas limit
		recreateErr := error(nil)
		rawTx, recreateErr = util.CreateERC20TransferTx(
			fromAddress,
			tokenAddress,
			to,
			amount,
			nonce,
			gasPrice,
			gasLimit,
			chainID,
		)
		if recreateErr != nil {
			return fmt.Errorf("failed to create transaction with estimated gas: %v", recreateErr)
		}
	} else if gasLimit == 0 {
		gasLimit = 100000 // Default gas limit for ERC20 transfers

		// Recreate the transaction with the default gas limit
		defaultGasErr := error(nil)
		rawTx, defaultGasErr = util.CreateERC20TransferTx(
			fromAddress,
			tokenAddress,
			to,
			amount,
			nonce,
			gasPrice,
			gasLimit,
			chainID,
		)
		if defaultGasErr != nil {
			return fmt.Errorf("failed to create transaction with default gas: %v", defaultGasErr)
		}
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
		// Format amount for display with token's decimals
		displayAmount := new(big.Float).SetInt(amount)
		divisor := new(big.Float).SetInt(new(big.Int).Exp(
			big.NewInt(10),
			big.NewInt(int64(tokenDecimals)),
			nil,
		))
		displayAmount.Quo(displayAmount, divisor)

		fmt.Println("Transaction Details:")
		fmt.Printf("From: %s\n", fromAddress)
		fmt.Printf("To: %s\n", to) // Highlighted in the terminal
		fmt.Printf("Token: %s (%s)\n", tokenAddress, tokenSymbol)
		fmt.Printf("Amount: %s %s\n", displayAmount.Text('f', int(tokenDecimals)), tokenSymbol) // Highlighted in the terminal
		fmt.Printf("Gas Limit: %d\n", gasLimit)
		fmt.Printf("Gas Price: %s Gwei\n", new(big.Float).Quo(
			new(big.Float).SetInt(gasPrice),
			new(big.Float).SetInt(big.NewInt(1000000000)),
		).Text('f', 9))
		fmt.Printf("Gas Fee: %s ETH\n", new(big.Float).Quo(
			new(big.Float).SetInt(new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))),
			new(big.Float).SetInt(big.NewInt(1000000000000000000)),
		).Text('f', 18))
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
