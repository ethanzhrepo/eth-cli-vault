VERSION=0.1.0
BINARY=eth-cli
RELEASE_DIR=release

# Read environment variables from .env file
include .env

# LDFLAGS with environment variables
LDFLAGS=-ldflags "\
-X main.version=${VERSION} \
-X main.googleOAuthClientID=${GOOGLE_OAUTH_CLIENT_ID} \
-X main.googleOAuthClientSecret=${GOOGLE_OAUTH_CLIENT_SECRET} \
-X main.dropboxAppKey=${DROPBOX_APP_KEY} \
-X main.awsAccessKeyID=${AWS_ACCESS_KEY_ID} \
-X main.awsSecretAccessKey=${AWS_SECRET_ACCESS_KEY} \
-X main.awsS3Bucket=${AWS_S3_BUCKET} \
-X main.awsRegion=${AWS_REGION} \
-X main.boxClientID=${BOX_CLIENT_ID} \
-X main.boxClientSecret=${BOX_CLIENT_SECRET}"

.PHONY: all clean build-all build-macos-arm build-macos-intel build-linux-x64 build-linux-amd64 build-linux-arm64 build-windows

all: build-all

build-all: clean build-macos-arm build-macos-intel build-linux-x64 build-linux-amd64 build-linux-arm64 build-windows

clean:
	@echo "Cleaning release directory..."
	@rm -rf $(RELEASE_DIR)
	@mkdir -p $(RELEASE_DIR)

build-macos-arm:
	@echo "Building for macOS ARM (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(VERSION)-darwin-Silicon main.go

build-macos-intel:
	@echo "Building for macOS Intel..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(VERSION)-darwin-intel main.go

build-linux-x64:
	@echo "Building for Linux x64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(VERSION)-linux-x64 main.go

build-linux-amd64:
	@echo "Building for Linux amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(VERSION)-linux-amd64 main.go

build-linux-arm64:
	@echo "Building for Linux arm64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(VERSION)-linux-arm64 main.go

build-windows:
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY)-$(VERSION)-windows-amd64.exe main.go
