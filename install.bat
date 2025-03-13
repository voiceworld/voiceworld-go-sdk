@echo off
setlocal enabledelayedexpansion

echo ===================================
echo VoiceWorld Go SDK Installation Tool
echo ===================================

:: Check Go installation and version
echo Checking Go installation and version...
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go is not installed. Please install Go from https://golang.org/dl/
    pause
    exit /b 1
)

:: Check Go version
for /f "tokens=3" %%v in ('go version') do set GO_VERSION=%%v
echo Found Go version: %GO_VERSION%
if "%GO_VERSION%" lss "go1.16" (
    echo [ERROR] Go version 1.16 or higher is required
    pause
    exit /b 1
)

:: Setup Go environment
echo Setting up Go environment...
for /f "tokens=*" %%i in ('go env GOPATH') do set "GOPATH=%%i"
set "PATH=%GOPATH%\bin;%PATH%"
echo Added %GOPATH%\bin to PATH

:: Create necessary directories
echo Creating necessary directories...
if not exist "voiceworld\proto\pb" (
    mkdir "voiceworld\proto\pb"
    if !ERRORLEVEL! NEQ 0 (
        echo [ERROR] Failed to create required directories
        pause
        exit /b 1
    )
)

:: Download dependencies
echo Installing required Go packages...
go mod download
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to download Go dependencies
    echo Please check your internet connection and try again
    pause
    exit /b 1
)

:: Install protoc-gen-go
echo Installing protoc-gen-go...
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to install protoc-gen-go
    echo Please check your internet connection and try again
    pause
    exit /b 1
)

:: Check protoc installation
echo Checking protoc installation...
where protoc >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Protocol Buffers compiler (protoc) is not installed or not in PATH
    echo Please download and install it from: https://github.com/protocolbuffers/protobuf/releases
    echo After installation, add the protoc bin directory to your PATH
    pause
    exit /b 1
)

:: Check protoc version
for /f "tokens=2 delims= " %%v in ('protoc --version') do set PROTOC_VERSION=%%v
echo Found protoc version: %PROTOC_VERSION%

:: Generate protobuf code
echo Generating protobuf code...
protoc --proto_path=voiceworld/proto --go_out=. --go_opt=module=voiceworld-go-sdk voiceworld/proto/voiceworld.proto
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to generate protobuf code
    echo Please ensure protoc and protoc-gen-go are properly installed
    echo You might need to restart your terminal after installation
    pause
    exit /b 1
)

:: Build the project
echo Building the project...
go build -o voiceworld.exe
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to build the project
    echo Please check the error messages above
    pause
    exit /b 1
)

echo ===================================
echo Installation completed successfully!
echo -----------------------------------
echo Next steps:
echo 1. Configure your APP_KEY and APP_SECRET
echo 2. Run voiceworld.exe to start using the SDK
echo ===================================
pause