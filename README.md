# VoiceWorld Go SDK

VoiceWorld Go SDK 是一个用于音频处理的 Go 语言开发工具包，提供了简单而强大的接口来使用 VoiceWorld 的语音处理服务。

## 功能特点

- 支持音频文件分割处理
- 提供 OSS 文件上传功能
- 支持实时进度查询
- 提供多种音频处理模型选择
- 支持自定义采样率和声道配置

## 系统要求

- Go 1.16 或更高版本
- Protocol Buffers 编译器 (protoc)
- Windows/Linux/MacOS 系统

## 安装步骤

### Windows 用户

1. 安装 Go
   - 访问 https://golang.org/dl/ 下载最新版本的 Go
   - 运行安装程序并按照提示完成安装
   - 验证安装：打开命令提示符，运行 `go version`

2. 安装 Protocol Buffers
   - 访问 https://github.com/protocolbuffers/protobuf/releases
   - 下载适用于 Windows 的最新版本
   - 解压文件，并将 bin 目录添加到系统环境变量 PATH 中
   - 验证安装：打开命令提示符，运行 `protoc --version`

3. 安装 SDK
   ```bash
   # 克隆仓库
   git clone https://github.com/your-username/voiceworld-go-sdk.git
   cd voiceworld-go-sdk

   # 运行安装脚本
   .\install.bat
   ```

### Linux/MacOS 用户

1. 安装 Go
   ```bash
   # 下载并安装 Go
   wget https://golang.org/dl/go1.24.1.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
   echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc
   source ~/.bashrc
   ```

2. 安装 Protocol Buffers
   ```bash
   # Ubuntu/Debian
   sudo apt-get install protobuf-compiler

   # MacOS
   brew install protobuf
   ```

3. 安装 SDK
   ```bash
   # 克隆仓库
   git clone https://github.com/your-username/voiceworld-go-sdk.git
   cd voiceworld-go-sdk

   # 运行安装脚本
   chmod +x install.sh
   ./install.sh
   ```

## 快速开始

### 1. 初始化客户端

```go
import "voiceworld-go-sdk/voiceworld/client"

// 创建配置
config := &client.ClientConfig{
    BaseURL:   "http://your-api-endpoint",
    OSSConfig: &client.OSSConfig{},
}

// 创建 SDK 客户端
sdk := client.NewClient("YOUR_APP_KEY", "YOUR_APP_SECRET", config)
```

### 2. 处理音频文件

```go
// 生成请求ID
requestID := fmt.Sprintf("%d", time.Now().UnixNano())
taskID := "your_task_id"

// 获取OSS上传凭证
_, err := sdk.GetOSSToken(requestID, taskID)
if err != nil {
    log.Fatal(err)
}

// 音频处理配置
modelType := client.ModelTypeBeat2B  // 或使用 client.ModelTypeHighAccur
sampleRate := 16000                  // 采样率
channels := 1                        // 声道数

// 处理音频文件
result, err := sdk.SplitAudioFile("path/to/audio.wav", requestID, taskID)
if err != nil {
    log.Fatal(err)
}

// 等待处理完成并获取结果
progress, err := sdk.WaitForComplete(requestID, taskID, modelType, sampleRate, channels, 2, 600, nil)
if err != nil {
    log.Fatal(err)
}
```

## 配置说明

### 客户端配置

- `BaseURL`: API 服务器地址
- `OSSConfig`: OSS 配置信息（通过 GetOSSToken 自动获取）

### 音频处理参数

- `ModelType`: 处理模型类型
  - `ModelTypeBeat2B`: 标准模型
  - `ModelTypeHighAccur`: 高精度模型
- `SampleRate`: 采样率（默认 16000）
- `Channels`: 声道数（默认 1）

## 错误处理

SDK 中的所有方法都会返回错误信息，建议在生产环境中妥善处理这些错误：

```go
result, err := sdk.SomeOperation()
if err != nil {
    // 记录错误日志
    log.Printf("操作失败: %v", err)
    // 根据错误类型进行处理
    switch err.(type) {
    case *client.APIError:
        // 处理 API 错误
    case *client.NetworkError:
        // 处理网络错误
    default:
        // 处理其他错误
    }
    return
}
```

## 注意事项

1. 请确保正确配置 APP_KEY 和 APP_SECRET
2. 音频文件支持的格式：WAV
3. 建议使用 16kHz 采样率和单声道配置以获得最佳效果
4. 处理大文件时注意设置合适的超时时间
5. 在生产环境中实现适当的重试机制

## 常见问题

1. Q: 如何处理上传超时？
   A: 可以通过调整 WaitForComplete 方法的超时参数来处理

2. Q: 支持哪些音频格式？
   A: 目前主要支持 WAV 格式，建议使用 16kHz 采样率

3. Q: 如何选择合适的模型？
   A: ModelTypeBeat2B 适合一般场景，ModelTypeHighAccur 适合需要更高精度的场景

## 技术支持

如果您在使用过程中遇到任何问题，请通过以下方式获取支持：

- 提交 GitHub Issue
- 发送邮件至技术支持邮箱
- 查阅在线文档

## 许可证

本项目采用 [许可证类型] 许可证，详情请参见 LICENSE 文件。 