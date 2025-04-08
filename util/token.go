package util

import (
	"fmt"
	"math/big"
	"strings"
)

// ParseTokenAmount converts a decimal string amount to the token's smallest unit using big.Int
func ParseTokenAmount(amountStr string, decimals uint8) (*big.Int, error) {
	amountStr = strings.TrimSpace(amountStr)
	if amountStr == "" {
		return nil, fmt.Errorf("amount cannot be empty")
	}

	// Split into integer and decimal parts
	parts := strings.Split(amountStr, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid amount format: %s", amountStr)
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

	// Calculate the multiplier for the integer part
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	result := new(big.Int).Mul(intVal, multiplier)

	// If there's a decimal part, handle it
	if len(parts) == 2 {
		decimalPart := parts[1]
		if decimalPart != "" {
			// Pad or truncate decimal part to match token decimals
			if len(decimalPart) > int(decimals) {
				decimalPart = decimalPart[:decimals]
			} else if len(decimalPart) < int(decimals) {
				decimalPart = decimalPart + strings.Repeat("0", int(decimals)-len(decimalPart))
			}

			// Parse decimal part
			decimalVal := new(big.Int)
			if _, ok := decimalVal.SetString(decimalPart, 10); !ok {
				return nil, fmt.Errorf("invalid decimal part: %s", decimalPart)
			}

			// Add decimal contribution to result
			result.Add(result, decimalVal)
		}
	}

	// Check for negative values
	if result.Sign() < 0 {
		return nil, fmt.Errorf("amount cannot be negative: %s", amountStr)
	}

	return result, nil
}
