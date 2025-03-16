package voiceworld

// 与SDK服务端交互的客户端

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// DefaultAPIEndpoint 默认API端点
	DefaultAPIEndpoint = "http://192.168.110.219:16062"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	APIEndpoint string // API端点
	AppKey      string // 应用密钥
	AppSecret   string // 应用密钥密文
}

// Client SDK客户端
type Client struct {
	config     ClientConfig
	httpClient *http.Client
}

// STSCredentialsRequest STS凭证请求
type STSCredentialsRequest struct {
	AppKey     string `json:"app_key"`                // API密钥
	AppSecret  string `json:"app_secret"`             // API密钥密文
	RequestID  string `json:"request_id"`             // 请求ID，由客户端生成的唯一标识
	UserTaskID string `json:"user_task_id,omitempty"` // 用户自定义任务ID，可用于关联业务系统（可选）
}

// STSCredentialsResponse STS凭证响应
type STSCredentialsResponse struct {
	Code int    `json:"code"` // 状态码：200成功，400参数错误，500服务器错误
	Msg  string `json:"msg"`  // 响应消息
	Data struct {
		RequestID   string `json:"request_id"`   // 请求ID
		UserTaskID  string `json:"user_task_id"` // 用户任务ID
		Credentials struct {
			AccessKeyID     string `json:"access_key_id"`     // 临时访问密钥ID
			AccessKeySecret string `json:"access_key_secret"` // 临时访问密钥
			SecurityToken   string `json:"security_token"`    // 安全令牌
			Expiration      string `json:"expiration"`        // 过期时间
		} `json:"credentials"`
		Bucket    string `json:"bucket"`     // OSS存储桶名称
		Endpoint  string `json:"endpoint"`   // OSS访问域名
		UploadDir string `json:"upload_dir"` // 上传目录
	} `json:"data"`
}

// AudioConfig 音频配置
type AudioConfig struct {
	SampleRate int `json:"sample_rate"`
	Channels   int `json:"channels"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	Type string `json:"type"`
}

// RunRequest 请求处理音频
type RunRequest struct {
	AppKey      string      `json:"app_key"`
	AppSecret   string      `json:"app_secret"`
	RequestID   string      `json:"request_id"`
	UserTaskID  string      `json:"user_task_id,omitempty"`
	AudioConfig AudioConfig `json:"audio_config"`
	ModelConfig ModelConfig `json:"model_config"`
	FilePath    string      `json:"file_path"`
}

type GetResultRequest struct {
	AppKey     string `json:"app_key"`
	AppSecret  string `json:"app_secret"`
	RequestID  string `json:"request_id"`
	UserTaskID string `json:"user_task_id"`
	TaskID     string `json:"task_id"`
}

type GetResultResponse struct {
	Code int    `json:"code"` // 状态码：200成功，400参数错误，500服务器错误
	Msg  string `json:"msg"`  // 响应消息
	Data struct {
		Result     string `json:"result"`       // 结果
		UserTaskID string `json:"user_task_id"` // 系统任务ID
	} `json:"data"`
}

// NewClient 创建新的客户端
func NewClient(config ClientConfig) *Client {
	// 如果未设置API端点，使用默认值
	if config.APIEndpoint == "" {
		config.APIEndpoint = DefaultAPIEndpoint
	}

	// 创建HTTP客户端
	httpClient := &http.Client{}

	return &Client{
		config:     config,
		httpClient: httpClient,
	}
}

// RunResponse 请求处理音频响应
type RunResponse struct {
	Code int    `json:"code"` // 状态码：200成功，400参数错误，500服务器错误
	Msg  string `json:"msg"`  // 响应消息
	Data struct {
		TaskID     string `json:"task_id"`      // 系统任务ID
		UserTaskID string `json:"user_task_id"` // 用户任务ID
		RequestID  string `json:"request_id"`   // 请求ID
	} `json:"data"`
}

// GenerateRequestID 生成请求ID
func GenerateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// GetSTSCredentials 获取STS临时凭证
func (c *Client) GetSTSCredentials(ctx context.Context, userTaskID string) (*STSCredentialsResponse, error) {
	// 创建请求
	requestID := GenerateRequestID()
	request := STSCredentialsRequest{
		AppKey:     c.config.AppKey,
		AppSecret:  c.config.AppSecret,
		RequestID:  requestID,
		UserTaskID: userTaskID,
	}

	// 将请求转换为JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.APIEndpoint+"/sts",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var stsResponse STSCredentialsResponse
	if err := json.Unmarshal(respBody, &stsResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查响应状态码
	if stsResponse.Code != 200 {
		return nil, fmt.Errorf("API错误: %d - %s", stsResponse.Code, stsResponse.Msg)
	}

	return &stsResponse, nil
}

// 请求处理音频 /run
/*
#### 请求地址
```
POST /run
```

#### 请求参数
```json
{
    "app_key": "your_app_key",          // API密钥
    "app_secret": "your_app_secret",    // API密钥密文
    "request_id": "1741878379771629800", // 请求ID，与获取凭证和上传时使用的ID保持一致
    "user_task_id": "user_task_123",    // 用户任务ID，与获取凭证时使用的ID保持一致（可选）
    "audio_config": {
        "sample_rate": 16000,           // 采样率（Hz）
        "channels": 1                   // 声道数
    },
    "model_config": {
        "type": "beat2b"                // 模型类型：beat2b(高性能)或highaccur(高精度)
    },
    "file_path": "audio/app_key/user_task_123/part1.wav"  // OSS中的音频文件路径
}
```

#### 响应格式
```json
{
    "code": 200,                       // 状态码
    "msg": "success",                  // 响应消息
    "data": {
        "task_id": "1900222364545712128", // 系统任务ID
        "user_task_id": "user_task_123",  // 用户任务ID
        "request_id": "1741878379771629800" // 请求ID
    }
}
```
*/
func (c *Client) Run(ctx context.Context, requestID string, userTaskID string, audioURL string, Type string) (*RunResponse, error) {
	// 创建请求
	request := RunRequest{
		AppKey:     c.config.AppKey,
		AppSecret:  c.config.AppSecret,
		RequestID:  requestID,
		UserTaskID: userTaskID,
		AudioConfig: AudioConfig{
			SampleRate: 16000,
			Channels:   1,
		},
		ModelConfig: ModelConfig{
			Type: Type,
		},
		FilePath: audioURL,
	}

	// 将请求转换为JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.APIEndpoint+"/run",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var runResponse RunResponse
	if err := json.Unmarshal(respBody, &runResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查响应状态码
	if runResponse.Code != 200 {
		return nil, fmt.Errorf("API错误: %d - %s", runResponse.Code, runResponse.Msg)
	}

	return &runResponse, nil
}

// GetResult 获取结果
func (c *Client) GetResult(ctx context.Context, requestID string, userTaskID string, taskID string) (*GetResultResponse, error) {
	// 创建请求post
	request := GetResultRequest{
		AppKey:     c.config.AppKey,
		AppSecret:  c.config.AppSecret,
		RequestID:  requestID,
		UserTaskID: userTaskID,
		TaskID:     taskID,
	}

	// 将请求转换为JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.config.APIEndpoint+"/task/result",
		bytes.NewBuffer(requestBody),
	)

	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var getResultResponse GetResultResponse
	if err := json.Unmarshal(respBody, &getResultResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &getResultResponse, nil
}
