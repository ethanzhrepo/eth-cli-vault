package util

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// ERC20TransferSignature is the function signature for the ERC20 transfer function
const ERC20TransferSignature = "transfer(address,uint256)"

// ERC20ApproveSignature is the function signature for the ERC20 approve function
const ERC20ApproveSignature = "approve(address,uint256)"

// ERC721TransferFromSignature is the function signature for the ERC721 transferFrom function
const ERC721TransferFromSignature = "transferFrom(address,address,uint256)"

// ERC721ApproveSignature is the function signature for the ERC721 approve function
const ERC721ApproveSignature = "approve(address,uint256)"

func GetNonce(client *ethclient.Client, address common.Address) (uint64, error) {
	nonce, err := client.PendingNonceAt(context.Background(), address)
	if err != nil {
		return 0, fmt.Errorf("get nonce failed: %v", err)
	}
	return nonce, nil
}

// SignTransaction 签署交易
// 函数1: 输入原始交易字符串、私钥，返回签署后的16进制数据
func SignTransaction(rawTxHex string, privateKeyHex string) (string, error) {
	// 解码原始交易
	rawTxData, err := hex.DecodeString(strings.TrimPrefix(rawTxHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("decode raw transaction failed: %v", err)
	}

	var tx types.Transaction
	err = tx.UnmarshalBinary(rawTxData)
	if err != nil {
		return "", fmt.Errorf("unmarshal transaction failed: %v", err)
	}

	// 解析私钥
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid private key: %v", err)
	}

	// 获取链ID
	chainID := tx.ChainId()

	// 使用私钥签署交易
	signedTx, err := types.SignTx(&tx, types.NewLondonSigner(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("sign transaction failed: %v", err)
	}

	// 将签署后的交易编码为字节
	signedTxData, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal signed transaction failed: %v", err)
	}

	// 返回十六进制字符串
	return "0x" + hex.EncodeToString(signedTxData), nil
}

// CreateEthTransferTx 构造ETH转账交易
// 函数2: 构造原始eth转账交易数据（未签署，原始交易）
func CreateEthTransferTx(fromAddress, toAddress string, amountInWei *big.Int, nonce uint64, gasPrice *big.Int, gasLimit uint64, chainID *big.Int) (string, error) {
	// 转换地址
	to := common.HexToAddress(toAddress)

	// 创建交易对象，包含链ID
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasPrice, // 1 Gwei tip
		GasFeeCap: gasPrice, // 最大总费用（包括基础费和小费）
		Gas:       gasLimit,
		To:        &to,
		Value:     amountInWei,
		Data:      []byte{},
	})

	// 将交易编码为字节
	txData, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal transaction failed: %v", err)
	}

	// 返回十六进制字符串
	return "0x" + hex.EncodeToString(txData), nil
}

// CreateERC20TransferTx 构造ERC20 Transfer交易
// 函数3: 构造原始的erc20 transfer交易数据（未签署，原始交易）
func CreateERC20TransferTx(fromAddress, tokenAddress, toAddress string, amount *big.Int, nonce uint64, gasPrice *big.Int, gasLimit uint64, chainID *big.Int) (string, error) {
	// 解析合约和接收者地址
	contract := common.HexToAddress(tokenAddress)
	to := common.HexToAddress(toAddress)

	// 创建ERC20 transfer的函数签名（前4字节）和参数
	transferFnSignature := crypto.Keccak256Hash([]byte(ERC20TransferSignature)).Bytes()[:4]

	// 将地址和数量填充到32字节
	paddedAddress := common.LeftPadBytes(to.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	// 组合数据
	var data []byte
	data = append(data, transferFnSignature...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	// 创建交易对象
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasPrice,
		GasFeeCap: gasPrice,
		Gas:       gasLimit,
		To:        &contract,
		Value:     big.NewInt(0), // ERC20转账不包含ETH
		Data:      data,
	})

	// 将交易编码为字节
	txData, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal transaction failed: %v", err)
	}

	// 返回十六进制字符串
	return "0x" + hex.EncodeToString(txData), nil
}

// CreateERC20ApproveTx 构造ERC20 Approve交易
// 函数4: 构造原始的erc20 approve交易数据（未签署，原始交易）
func CreateERC20ApproveTx(fromAddress, tokenAddress, spenderAddress string, amount *big.Int, nonce uint64, gasPrice *big.Int, gasLimit uint64, chainID *big.Int) (string, error) {
	// 解析合约和授权者地址
	contract := common.HexToAddress(tokenAddress)
	spender := common.HexToAddress(spenderAddress)

	// 创建ERC20 approve的函数签名（前4字节）和参数
	approveFnSignature := crypto.Keccak256Hash([]byte(ERC20ApproveSignature)).Bytes()[:4]

	// 将地址和数量填充到32字节
	paddedAddress := common.LeftPadBytes(spender.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	// 组合数据
	var data []byte
	data = append(data, approveFnSignature...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	// 创建交易对象
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasPrice,
		GasFeeCap: gasPrice,
		Gas:       gasLimit,
		To:        &contract,
		Value:     big.NewInt(0), // Approve不包含ETH
		Data:      data,
	})

	// 将交易编码为字节
	txData, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal transaction failed: %v", err)
	}

	// 返回十六进制字符串
	return "0x" + hex.EncodeToString(txData), nil
}

// CreateERC721TransferTx 构造ERC721转账交易
// 函数5: 构造原始的erc721的转账交易
func CreateERC721TransferTx(fromAddress, contractAddress, toAddress string, tokenID *big.Int, nonce uint64, gasPrice *big.Int, gasLimit uint64, chainID *big.Int) (string, error) {
	// 解析地址
	contract := common.HexToAddress(contractAddress)
	from := common.HexToAddress(fromAddress)
	to := common.HexToAddress(toAddress)

	// 创建ERC721 transferFrom的函数签名（前4字节）和参数
	transferFnSignature := crypto.Keccak256Hash([]byte(ERC721TransferFromSignature)).Bytes()[:4]

	// 将参数填充到32字节
	paddedFromAddress := common.LeftPadBytes(from.Bytes(), 32)
	paddedToAddress := common.LeftPadBytes(to.Bytes(), 32)
	paddedTokenID := common.LeftPadBytes(tokenID.Bytes(), 32)

	// 组合数据
	var data []byte
	data = append(data, transferFnSignature...)
	data = append(data, paddedFromAddress...)
	data = append(data, paddedToAddress...)
	data = append(data, paddedTokenID...)

	// 创建交易对象
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasPrice,
		GasFeeCap: gasPrice,
		Gas:       gasLimit,
		To:        &contract,
		Value:     big.NewInt(0), // NFT转账不包含ETH
		Data:      data,
	})

	// 将交易编码为字节
	txData, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal transaction failed: %v", err)
	}

	// 返回十六进制字符串
	return "0x" + hex.EncodeToString(txData), nil
}

// CreateERC721ApproveTx 构造ERC721授权交易
// 函数6: 构造原始的erc721的授权交易
func CreateERC721ApproveTx(fromAddress, contractAddress, approvedAddress string, tokenID *big.Int, nonce uint64, gasPrice *big.Int, gasLimit uint64, chainID *big.Int) (string, error) {
	// 解析地址
	contract := common.HexToAddress(contractAddress)
	approved := common.HexToAddress(approvedAddress)

	// 创建ERC721 approve的函数签名（前4字节）和参数
	approveFnSignature := crypto.Keccak256Hash([]byte(ERC721ApproveSignature)).Bytes()[:4]

	// 将参数填充到32字节
	paddedApprovedAddress := common.LeftPadBytes(approved.Bytes(), 32)
	paddedTokenID := common.LeftPadBytes(tokenID.Bytes(), 32)

	// 组合数据
	var data []byte
	data = append(data, approveFnSignature...)
	data = append(data, paddedApprovedAddress...)
	data = append(data, paddedTokenID...)

	// 创建交易对象
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasPrice,
		GasFeeCap: gasPrice,
		Gas:       gasLimit,
		To:        &contract,
		Value:     big.NewInt(0), // 授权不包含ETH
		Data:      data,
	})

	// 将交易编码为字节
	txData, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal transaction failed: %v", err)
	}

	// 返回十六进制字符串
	return "0x" + hex.EncodeToString(txData), nil
}

// EstimateGas 估算交易需要的gas limit
func EstimateGas(client *ethclient.Client, from common.Address, to *common.Address, value *big.Int, data []byte) (uint64, error) {
	msg := ethereum.CallMsg{
		From:  from,
		To:    to,
		Value: value,
		Data:  data,
	}

	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return 0, fmt.Errorf("estimate gas failed: %v", err)
	}

	return gasLimit, nil
}

// EstimateGasAndSimulateTx 预执行交易，估算gas并模拟交易执行
// 函数7: 预执行， 输入：被签署的交易、rpc，执行estimateGas，预执行，识别错误。
func EstimateGasAndSimulateTx(signedTxHex string, rpcURL string) (uint64, string, error) {
	// 连接到以太坊节点
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return 0, "", fmt.Errorf("connect to ethereum node failed: %v", err)
	}

	// 解码已签名的交易
	signedTxData, err := hexutil.Decode(signedTxHex)
	if err != nil {
		return 0, "", fmt.Errorf("decode signed transaction failed: %v", err)
	}

	var tx types.Transaction
	err = tx.UnmarshalBinary(signedTxData)
	if err != nil {
		return 0, "", fmt.Errorf("unmarshal transaction failed: %v", err)
	}

	// 获取发送方地址
	signer := types.LatestSignerForChainID(tx.ChainId())
	sender, err := signer.Sender(&tx)
	if err != nil {
		return 0, "", fmt.Errorf("get sender address failed: %v", err)
	}

	// 创建交易消息
	msg := ethereum.CallMsg{
		From:     sender,
		To:       tx.To(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice(),
		Value:    tx.Value(),
		Data:     tx.Data(),
	}

	// 估算gas
	estimatedGas, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return 0, "", fmt.Errorf("estimate gas failed: %v", err)
	}

	// 模拟交易执行
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return estimatedGas, "", fmt.Errorf("transaction simulation failed: %v", err)
	}

	// 如果result为空，通常意味着交易只是简单的ETH转账或可能成功执行
	if len(result) == 0 {
		return estimatedGas, "Transaction will likely succeed", nil
	}

	// 返回预执行结果
	return estimatedGas, "0x" + hex.EncodeToString(result), nil
}

// BroadcastTransaction 广播交易到网络
// 函数8: 广播交易
func BroadcastTransaction(signedTxHex string, rpcURL string) (string, error) {
	// 连接到以太坊节点
	rpcClient, err := rpc.Dial(rpcURL)
	if err != nil {
		return "", fmt.Errorf("connect to ethereum node failed: %v", err)
	}
	client := ethclient.NewClient(rpcClient)

	// 解码已签名的交易
	signedTxData, err := hexutil.Decode(signedTxHex)
	if err != nil {
		return "", fmt.Errorf("decode signed transaction failed: %v", err)
	}

	var tx types.Transaction
	err = tx.UnmarshalBinary(signedTxData)
	if err != nil {
		return "", fmt.Errorf("unmarshal transaction failed: %v", err)
	}

	// 发送交易到网络
	err = client.SendTransaction(context.Background(), &tx)
	if err != nil {
		return "", fmt.Errorf("send transaction failed: %v", err)
	}

	// 返回交易哈希
	return tx.Hash().Hex(), nil
}

// SignMessage signs a message with the Ethereum personal_sign method
// If hexMessage is true, the message is expected to be a hex string
// Otherwise it's treated as a plain text message
func SignMessage(message string, privateKeyHex string, hexMessage bool) (string, error) {
	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid private key: %v", err)
	}

	// Process message based on type
	var messageBytes []byte
	if hexMessage {
		// Verify hex string format
		if !strings.HasPrefix(message, "0x") {
			return "", fmt.Errorf("hex message must start with 0x")
		}

		// Decode hex string
		messageBytes, err = hex.DecodeString(strings.TrimPrefix(message, "0x"))
		if err != nil {
			return "", fmt.Errorf("invalid hex message: %v", err)
		}
	} else {
		// Use plain text message
		messageBytes = []byte(message)
	}

	// Create Ethereum specific message
	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageBytes), messageBytes)
	prefixedHash := crypto.Keccak256Hash([]byte(prefixedMessage))

	// Sign the message
	signature, err := crypto.Sign(prefixedHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %v", err)
	}

	// Adjust v value (last byte) in signature: v = 27 + v
	signature[64] += 27

	// Return hex-encoded signature
	return "0x" + hex.EncodeToString(signature), nil
}
