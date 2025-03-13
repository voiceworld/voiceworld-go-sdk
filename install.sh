#!/bin/bash

# 设置错误处理
set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_error() {
    echo -e "${RED}[ERROR] $1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        print_error "$1 is not installed"
        return 1
    fi
}

echo "==================================="
echo "VoiceWorld Go SDK Installation Tool"
echo "==================================="

# 检查 Go 安装
echo "Checking Go installation and version..."
if ! check_command go; then
    print_error "Please install Go from https://golang.org/dl/"
    exit 1
fi

# 检查 Go 版本
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Found Go version: $GO_VERSION"
if [[ "$(printf '%s\n' "1.16" "$GO_VERSION" | sort -V | head -n1)" == "$GO_VERSION" ]]; then
    print_error "Go version 1.16 or higher is required"
    exit 1
fi

# 检查 protoc 安装
echo "Checking protoc installation..."
if ! check_command protoc; then
    print_error "Protocol Buffers compiler (protoc) is not installed"
    echo "Please install it from your package manager or"
    echo "download from: https://github.com/protocolbuffers/protobuf/releases"
    exit 1
fi

# 检查 protoc 版本
PROTOC_VERSION=$(protoc --version | awk '{print $2}')
echo "Found protoc version: $PROTOC_VERSION"

# 创建必要的目录
echo "Creating necessary directories..."
mkdir -p voiceworld/proto/pb

# 下载依赖
echo "Installing required Go packages..."
if ! go mod download; then
    print_error "Failed to download Go dependencies"
    echo "Please check your internet connection and try again"
    exit 1
fi

# 安装 protoc-gen-go
echo "Installing protoc-gen-go..."
if ! go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; then
    print_error "Failed to install protoc-gen-go"
    echo "Please check your internet connection and try again"
    exit 1
fi

# 生成 protobuf 代码
echo "Generating protobuf code..."
if ! protoc --proto_path=voiceworld/proto \
    --go_out=. --go_opt=module=voiceworld-go-sdk \
    voiceworld/proto/voiceworld.proto; then
    print_error "Failed to generate protobuf code"
    echo "Please ensure protoc and protoc-gen-go are properly installed"
    exit 1
fi

# 构建项目
echo "Building the project..."
if ! go build -o voiceworld; then
    print_error "Failed to build the project"
    echo "Please check the error messages above"
    exit 1
fi

# 设置执行权限
chmod +x voiceworld

print_success "==================================="
print_success "Installation completed successfully!"
print_success "-----------------------------------"
echo "Next steps:"
echo "1. Configure your APP_KEY and APP_SECRET"
echo "2. Run ./voiceworld to start using the SDK"
print_success "===================================" 