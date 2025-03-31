# 以太坊命令行钱包

![GitHub commit activity](https://img.shields.io/github/commit-activity/w/ethanzhrepo/eth-cli-vault)
![GitHub Release](https://img.shields.io/github/v/release/ethanzhrepo/eth-cli-vault)
![GitHub Repo stars](https://img.shields.io/github/stars/ethanzhrepo/eth-cli-vault)

<a href="https://t.me/ethanatca"><img alt="" src="https://img.shields.io/badge/Telegram-%40ethanatca-blue" /></a>
<a href="https://x.com/intent/follow?screen_name=0x99_Ethan">
<img alt="X (formerly Twitter) Follow" src="https://img.shields.io/twitter/follow/0x99_Ethan">
</a>

一个安全的以太坊命令行钱包，支持多种密钥存储提供商。

## 解决场景

助记词不知道该怎么备份更安全？抄在纸上？刻在钢板上？打乱顺序？第25个助记词？密码云存储器？硬件钱包？
- 物理备份容易丢失、损毁
- 存在云盘又怕被盗

安全实践：使用aes和passpharse双重保护后在多个云盘备份。只需要记住两个密码，一个用来解密24个助记词，一个用来结合24个助记词还原密钥。

[英文](./README.md) | [中文](./README_cn.md) 

## 重要安全提示

**所有数据文件和凭证始终完全由您自行控制。** 这个钱包通过自我托管让您完全控制您的资产：

- 钱包文件在存储前已用您的密码加密
- 私钥永远不会与任何第三方共享
- 云存储提供商无法访问您的未加密数据
- 您需要负责安全地存储您的钱包文件并记住您的密码
- 如果您丢失了加密文件或密码，没有任何恢复机制可用

始终保持多个加密钱包文件的备份，并确保您永远不会忘记密码。

## 安全特性

- BIP39 助记词生成（24个单词）
- 可选 BIP39 密码短语支持
- 使用 Argon2id 密钥派生的 AES-256-GCM 加密
- 通过 OAuth 支持云存储（Google Drive、Dropbox、OneDrive、AWS S3）
- 本地钱包存储选项

## 安装

### 二进制安装（最简单）

```bash
# 从发布页面下载最新版本
# macOS系统
curl -L -o eth-cli https://github.com/ethanzhrepo/eth-cli-wallet/releases/download/v1.0.0/eth-cli-darwin-amd64
chmod +x eth-cli

# Linux系统
curl -L -o eth-cli https://github.com/ethanzhrepo/eth-cli-wallet/releases/download/v1.0.0/eth-cli-linux-amd64
chmod +x eth-cli

# Windows系统
# 从发布页面下载并重命名为eth-cli.exe
```

### 从源代码构建

```bash
# 从源代码安装
git clone https://github.com/ethanzhrepo/eth-cli-wallet
cd eth-cli-wallet
go build -o eth-cli
```

### 云存储的环境变量配置

如果您计划使用云存储，需要配置OAuth凭证。设置以下环境变量：

```bash
# Google Drive
export GOOGLE_OAUTH_CLIENT_ID="your-client-id"
export GOOGLE_OAUTH_CLIENT_SECRET="your-client-secret"

# Dropbox
export DROPBOX_APP_KEY="your-app-key"

# OneDrive
export ONEDRIVE_CLIENT_ID="your-client-id"

# AWS S3
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="your-region"
export AWS_S3_BUCKET="your-bucket-name"
```

**注意：** 如果您不想设置云存储凭证，仍然可以仅使用本地文件。钱包文件已经过加密，可以手动上传到任何您选择的云存储服务。AES加密可以保护您的钱包数据，即使存储在不受信任的位置。

## 配置

```bash
# 设置 RPC URL（区块链操作必需）
./eth-cli config set rpc https://your-ethereum-rpc-url

# 列出所有配置设置
./eth-cli config list

# 获取特定配置值
./eth-cli config get rpc

# 删除配置值
./eth-cli config delete rpc
```

## 创建钱包

```bash
# 存储到云端（不保存到本地）
./eth-cli create --output google,onedrive,dropbox --name myWallet [--force]
# 将保存到云存储的 /MyWallet/{name}.json 中

# 存储到云端和本地文件
./eth-cli create --output /path/to/save/myWallet.json,google,onedrive,dropbox --name myWallet
# 将保存到云存储和指定的本地路径

# 仅保存到本地文件（如果您不想使用云存储）
./eth-cli create --output /path/to/save/myWallet.json --name myWallet
# 您可以手动将此加密文件上传到任何云存储
```

创建钱包时：
1. 您将被要求输入 BIP39 密码短语（可选，增加额外安全性）
2. 您需要设置一个强密码来加密 24 个助记词

**重要提示：** BIP39 密码短语和 AES 密码对于访问您的钱包至关重要。如果丢失，由于使用了强加密算法，您的资产将永久无法访问。

## 管理钱包

```bash
# 列出存储提供商中的所有钱包
./eth-cli list --input google
./eth-cli list --input onedrive
./eth-cli list --input dropbox

# 获取钱包地址
./eth-cli get --input google --name myWallet
./eth-cli get --input /path/to/wallet.json

# 获取钱包地址及其他选项
./eth-cli get --input google --name myWallet --show-mnemonics --show-private-key
```

## 获取 Gas 价格

```bash
./eth-cli gas-price
```

## 交易

### 转账 ETH

```bash
./eth-cli transfer --amount 1.0eth --to 0xDestinationAddress --provider google --name myWallet [选项]

# 选项：
# --encodeOnly        仅创建并显示原始交易，不进行广播
# --gasOnly           仅显示gas估算，不创建交易
# --yes, -y           自动确认交易，不提示
# --gasPrice 3gwei    指定自定义gas价格
# --gasLimit 21000    指定自定义gas限制
# --sync              等待交易确认
# --file /path/to/wallet.json    使用本地钱包文件而非云提供商
```

### 转账 ERC20 代币

```bash
./eth-cli transferERC20 --amount 120.23 --to 0xDestinationAddress --token 0xTokenContractAddress --provider google --name myWallet [选项]
```

### 授权 ERC20 代币

```bash
./eth-cli approveERC20 --amount 120.23 --to 0xSpenderAddress --token 0xTokenContractAddress --provider google --name myWallet [选项]
# 将 amount 设置为 0 可撤销授权
```

### 转账 ERC721 NFT

```bash
./eth-cli transferERC721 --id tokenId --to 0xDestinationAddress --token 0xNFTContractAddress --provider google --name myWallet [选项]
```

### 授权 ERC721 NFT

```bash
./eth-cli approveERC721 --id tokenId --to 0xOperatorAddress --token 0xNFTContractAddress --provider google --name myWallet [选项]
# 使用 0x0000000000000000000000000000000000000000 地址可撤销授权
```

## 签名操作

### 签署原始交易

```bash
./eth-cli sign-raw-tx --raw-tx 0xRawTransactionHex --provider google --name myWallet [--broadcast]
# 或使用文件：
./eth-cli sign-raw-tx --raw-tx-file /path/to/tx.txt --provider google --name myWallet [--broadcast]
```

### 签署消息

```bash
./eth-cli sign-message --data "Hello, Ethereum!" --provider google --name myWallet
# 或使用十六进制：
./eth-cli sign-message --hex --data 0x48656c6c6f2c20457468657265756d21 --provider google --name myWallet
# 或从文件：
./eth-cli sign-message --data-file /path/to/message.txt --provider google --name myWallet
```

## 安全建议

1. 将您的 BIP39 密码短语和 AES 密码存储在不同的安全位置
2. 在转移大量资产前，使用 `get` 命令测试钱包访问
3. 考虑使用多个存储提供商以实现冗余
4. 确认前始终验证交易详情
5. 在不同位置保存多个加密钱包文件的备份
6. 该钱包遵循自我托管原则 - 只有您自己控制并对您的密钥负责

## 许可证

[许可证信息]
