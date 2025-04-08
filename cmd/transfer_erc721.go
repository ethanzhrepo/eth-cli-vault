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

// TransferERC721Cmd creates the ERC721 transfer command
func TransferERC721Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transferERC721",
		Short: "Transfer ERC721 (NFT) tokens to another address",
		Long:  `Transfer ERC721 (NFT) tokens to another Ethereum address.`,
		RunE:  runTransferERC721,
	}

	cmd.Flags().String("id", "", "ID of the NFT token to transfer")
	cmd.Flags().StringP("to", "t", "", "Destination address")
	cmd.Flags().String("token", "", "ERC721 token contract address")
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

	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("to")
	cmd.MarkFlagRequired("token")

	return cmd
}

func runTransferERC721(cmd *cobra.Command, args []string) error {
	// Parse flags
	tokenIDStr, _ := cmd.Flags().GetString("id")
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

	// Parse token ID
	tokenID, ok := new(big.Int).SetString(tokenIDStr, 0) // 0 means auto-detect base
	if !ok {
		return fmt.Errorf("invalid token ID format: %s", tokenIDStr)
	}

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
	var nftName string

	if !dryRun {
		var dialErr error
		client, dialErr = ethclient.Dial(rpcURL)
		if dialErr != nil {
			return fmt.Errorf("failed to connect to Ethereum node: %v", dialErr)
		}
		fmt.Printf("Using RPC: %s\n", rpcURL)

		// Get NFT contract name (optional)
		var nameErr error
		nftName, nameErr = getNFTName(client, tokenAddress)
		if nameErr != nil {
			nftName = "NFT" // Default name if we can't get it
		}
	} else {
		nftName = "NFT" // Default for dry run
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
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &toAddr, nil, nil)
		if gasEstimateErr != nil {
			return fmt.Errorf("failed to estimate gas: %v", gasEstimateErr)
		}
	} else if gasLimit == 0 {
		gasLimit = 100000 // Default gas limit for ERC721 transfers
	}

	// Create raw transaction
	rawTx, err := util.CreateERC721TransferTx(
		fromAddress,
		tokenAddress,
		to,
		tokenID,
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
		var gasEstimateErr error
		gasLimit, gasEstimateErr = util.EstimateGas(client, fromAddr, &toAddr, tx.Value(), tx.Data())
		if gasEstimateErr != nil {
			return fmt.Errorf("failed to estimate gas: %v", gasEstimateErr)
		}

		// Recreate the transaction with the estimated gas limit
		var recreateErr error
		rawTx, recreateErr = util.CreateERC721TransferTx(
			fromAddress,
			tokenAddress,
			to,
			tokenID,
			nonce,
			gasPrice,
			gasLimit,
			chainID,
		)
		if recreateErr != nil {
			return fmt.Errorf("failed to create transaction with estimated gas: %v", recreateErr)
		}
	} else if gasLimit == 0 {
		gasLimit = 150000 // Default gas limit for ERC721 transfers

		// Recreate the transaction with the default gas limit
		var defaultGasErr error
		rawTx, defaultGasErr = util.CreateERC721TransferTx(
			fromAddress,
			tokenAddress,
			to,
			tokenID,
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
		// Convert gas price to Gwei
		gasPriceGwei := new(big.Int).Div(gasPrice, big.NewInt(1e9))
		gasPriceRemainder := new(big.Int).Mod(gasPrice, big.NewInt(1e9))
		displayGasPrice := fmt.Sprintf("%d.%09d", gasPriceGwei, gasPriceRemainder)

		// Calculate gas fee in Wei
		gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
		gasFeeEth := new(big.Int).Div(gasFee, big.NewInt(1e18))
		gasFeeRemainder := new(big.Int).Mod(gasFee, big.NewInt(1e18))
		displayGasFee := fmt.Sprintf("%d.%018d", gasFeeEth, gasFeeRemainder)

		fmt.Println("Transaction Details:")
		fmt.Printf("From: %s\n", fromAddress)
		fmt.Printf("To: %s\n", to)
		fmt.Printf("NFT Contract: %s (%s)\n", tokenAddress, nftName)
		fmt.Printf("Token ID: %s\n", tokenID.String())
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

// getNFTName gets the name of an NFT contract
func getNFTName(client *ethclient.Client, contractAddress string) (string, error) {
	address := common.HexToAddress(contractAddress)
	callData := []byte{0x06, 0xfd, 0xde, 0x03} // keccak256("name()")[:4]

	msg := ethereum.CallMsg{
		To:   &address,
		Data: callData,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "NFT", nil
	}

	// Simple parsing: assuming result is a bytes32 string
	name := ""
	if len(result) > 32 {
		offset := new(big.Int).SetBytes(result[0:32]).Int64()
		if offset < int64(len(result)) {
			length := new(big.Int).SetBytes(result[offset : offset+32]).Int64()
			if offset+32+length <= int64(len(result)) {
				nameBytes := result[offset+32 : offset+32+length]
				name = string(nameBytes)
			}
		}
	} else if len(result) > 0 {
		name = string(result)
	}

	return name, nil
}
