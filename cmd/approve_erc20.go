package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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
	cmd.Flags().Bool("encodeOnly", false, "Only encode the transaction, do not broadcast")
	cmd.Flags().Bool("gasOnly", false, "Only display gas estimation")
	cmd.Flags().BoolP("yes", "y", false, "Automatically confirm the transaction")
	cmd.Flags().String("gasPrice", "", "Gas price (e.g., 3gwei)")
	cmd.Flags().Uint64("gasLimit", 0, "Gas limit")
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
	encodeOnly, _ := cmd.Flags().GetBool("encodeOnly")
	gasOnly, _ := cmd.Flags().GetBool("gasOnly")
	autoConfirm, _ := cmd.Flags().GetBool("yes")
	gasPriceStr, _ := cmd.Flags().GetString("gasPrice")
	gasLimit, _ := cmd.Flags().GetUint64("gasLimit")
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
	if err != nil && !encodeOnly {
		return err
	}

	// Print provider or file info
	if provider != "" {
		fmt.Printf("Using provider: %s\n", provider)
	} else {
		fmt.Printf("Using wallet file: %s\n", filePath)
	}

	// Check if we need RPC
	if !encodeOnly {
		if rpcURL == "" {
			return fmt.Errorf("RPC URL is required when not using --encodeOnly")
		}
	}

	// Connect to Ethereum client if needed
	var client *ethclient.Client
	var tokenSymbol string
	var tokenDecimals uint8
	var amount *big.Int

	if !encodeOnly {
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
		// For encode only, just use a default for the preview
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

	// Get chain ID
	var chainID *big.Int
	if !encodeOnly {
		var chainErr error
		chainID, chainErr = client.NetworkID(context.Background())
		if chainErr != nil {
			return fmt.Errorf("failed to get chain ID: %v", chainErr)
		}
	} else {
		chainID = big.NewInt(1) // Default to Ethereum mainnet if encode only
	}

	// Get gas price
	var gasPrice *big.Int
	if gasPriceStr != "" {
		var gasPriceErr error
		gasPrice, gasPriceErr = parseEthAmount(gasPriceStr)
		if gasPriceErr != nil {
			return gasPriceErr
		}
	} else if !encodeOnly {
		var suggestErr error
		gasPrice, suggestErr = client.SuggestGasPrice(context.Background())
		if suggestErr != nil {
			return fmt.Errorf("failed to get suggested gas price: %v", suggestErr)
		}
	} else {
		gasPrice = big.NewInt(1000000000) // Default 1 Gwei if encode only
	}

	// Get nonce
	var nonce uint64
	if !encodeOnly {
		var nonceErr error
		fromAddr := common.HexToAddress(fromAddress)
		nonce, nonceErr = util.GetNonce(client, fromAddr)
		if nonceErr != nil {
			return fmt.Errorf("failed to get nonce: %v", nonceErr)
		}
	}

	// Create raw transaction
	var createErr error
	rawTx, createErr := util.CreateERC20ApproveTx(
		fromAddress,
		tokenAddress,
		to,
		amount,
		nonce,
		gasPrice,
		gasLimit,
		chainID,
	)
	if createErr != nil {
		return fmt.Errorf("failed to create transaction: %v", createErr)
	}

	// Estimate gas if needed
	if gasLimit == 0 && !encodeOnly {
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
		var gasEstimateErr error
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &toAddr, tx.Value(), tx.Data())
		if gasEstimateErr != nil {
			return fmt.Errorf("failed to estimate gas: %v", gasEstimateErr)
		}

		// Recreate the transaction with the estimated gas limit
		var recreateErr error
		rawTx, recreateErr = util.CreateERC20ApproveTx(
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
		gasLimit = 60000 // Default gas limit for ERC20, slightly lower than transfer

		// Recreate the transaction with the default gas limit
		var defaultGasErr error
		rawTx, defaultGasErr = util.CreateERC20ApproveTx(
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
	if gasOnly {
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

	// If encode only, just display the raw transaction and exit
	if encodeOnly {
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

		approveType := "Approval"
		if amount.Cmp(big.NewInt(0)) == 0 {
			approveType = "Revocation of approval"
		}

		fmt.Println("Transaction Details:")
		fmt.Printf("Type: %s\n", approveType)
		fmt.Printf("From: %s\n", fromAddress)
		fmt.Printf("Spender: %s\n", to) // Highlighted in the terminal
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
