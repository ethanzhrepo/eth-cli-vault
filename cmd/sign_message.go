package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ethanzhrepo/eth-cli-wallet/util"
	"github.com/spf13/cobra"
)

// SignMessageCmd creates the message signing command
func SignMessageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign-message",
		Short: "Sign an Ethereum message",
		Long:  `Sign a text or hex message using the specified private key.`,
		RunE:  runSignMessage,
	}

	// Add flags
	cmd.Flags().BoolP("hex", "x", false, "Interpret message as hex (must start with 0x)")
	cmd.Flags().StringP("data", "d", "", "Message to sign (text or hex)")
	cmd.Flags().String("data-file", "", "Path to file containing message to sign")
	cmd.Flags().StringP("provider", "p", "", "Key provider (e.g., google)")
	cmd.Flags().StringP("name", "n", "", "Name of the wallet file (for cloud storage)")
	cmd.Flags().StringP("file", "f", "", "Local wallet file path")

	return cmd
}

func runSignMessage(cmd *cobra.Command, args []string) error {
	// Parse flags
	isHex, _ := cmd.Flags().GetBool("hex")
	message, _ := cmd.Flags().GetString("data")
	dataFile, _ := cmd.Flags().GetString("data-file")
	provider, _ := cmd.Flags().GetString("provider")
	name, _ := cmd.Flags().GetString("name")
	filePath, _ := cmd.Flags().GetString("file")

	// Check for message source
	if message == "" && dataFile == "" {
		return fmt.Errorf("either --data or --data-file must be specified")
	}

	if message != "" && dataFile != "" {
		return fmt.Errorf("--data and --data-file are mutually exclusive, use one or the other")
	}

	// Get message from file if necessary
	if dataFile != "" {
		data, err := os.ReadFile(dataFile)
		if err != nil {
			return fmt.Errorf("failed to read data file: %v", err)
		}
		// Trim any whitespace or newlines
		message = strings.TrimSpace(string(data))
	}

	// Check mutual exclusivity between provider+name and file
	if (provider != "" || name != "") && filePath != "" {
		return fmt.Errorf("--file and --provider/--name are mutually exclusive, use one or the other")
	}

	// Ensure we have either file or provider
	if provider == "" && filePath == "" {
		return fmt.Errorf("either --provider or --file must be specified")
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
	var err error
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

	// Check if hex message is valid
	if isHex && !strings.HasPrefix(message, "0x") {
		return fmt.Errorf("hex message must start with 0x")
	}

	// Sign the message
	signature, err := util.SignMessage(message, privateKey, isHex)
	if err != nil {
		return fmt.Errorf("failed to sign message: %v", err)
	}

	// Display the signed message details
	fmt.Printf("Message: %s\n", message)
	fmt.Printf("Signer Address: %s\n", fromAddress)
	fmt.Printf("Signature: %s\n", signature)

	return nil
}
