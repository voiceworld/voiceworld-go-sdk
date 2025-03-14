package client

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/schollz/progressbar/v3"
)

// OSSConfig OSS配置信息
type OSSConfig struct {
	Endpoint    string      // OSS终端节点
	BucketName  string      // OSS存储桶名称
	Credentials interface{} // OSS凭证信息
}

// OSSCredentials OSS临时凭证信息
type OSSCredentials struct {
	AccessKeyId     string `json:"access_key_id"`     // 临时访问密钥ID
	AccessKeySecret string `json:"access_key_secret"` // 临时访问密钥密码
	SecurityToken   string `json:"security_token"`    // 安全令牌
	Expiration      string `json:"expiration"`        // 过期时间
}

// OSSTokenResponse OSS临时凭证响应
type OSSTokenResponse struct {
	Code int    `json:"code"` // 响应状态码
	Msg  string `json:"msg"`  // 响应消息
	Data struct {
		RequestID   string         `json:"request_id"`  // 请求ID
		TaskID      string         `json:"task_id"`     // 用户任务ID
		Credentials OSSCredentials `json:"credentials"` // 凭证信息
		Bucket      string         `json:"bucket"`      // OSS存储桶名称
		Endpoint    string         `json:"endpoint"`    // OSS访问域名
		UploadDir   string         `json:"upload_dir"`  // 上传目录
	} `json:"data"`
}

// UploadFileResponse 上传文件响应
type UploadFileResponse struct {
	Success   bool   `json:"success"`    // 上传是否成功
	Message   string `json:"message"`    // 响应消息
	URL       string `json:"url"`        // 文件访问URL
	RequestID string `json:"request_id"` // 请求ID
}

// Client 表示 VoiceWorld API 客户端
type Client struct {
	appKey     string       // 应用密钥
	appSecret  string       // 应用密钥
	baseURL    string       // API基础URL地址
	httpClient *http.Client // HTTP客户端实例
	ossConfig  *OSSConfig   // OSS配置信息
	taskID     string       // 当前任务ID
}

// ClientConfig 客户端配置选项
type ClientConfig struct {
	BaseURL   string     // API基础URL地址
	OSSConfig *OSSConfig // OSS配置信息（可选）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL: "http://localhost:16061",
		OSSConfig: &OSSConfig{
			Endpoint:    "https://oss-cn-shanghai.aliyuncs.com",
			BucketName:  "voiceworld",
			Credentials: nil,
		},
	}
}

// NewClient 创建一个新的 VoiceWorld API 客户端实例
// appKey: 应用密钥
// appSecret: 应用密钥
// config: 客户端配置（可选，如果不提供则使用默认配置）
func NewClient(appKey, appSecret string, config ...*ClientConfig) *Client {
	cfg := DefaultConfig()
	if len(config) > 0 && config[0] != nil {
		if config[0].BaseURL != "" {
			cfg.BaseURL = config[0].BaseURL
		}
		if config[0].OSSConfig != nil {
			cfg.OSSConfig = config[0].OSSConfig
		}
	}

	return &Client{
		appKey:     appKey,
		appSecret:  appSecret,
		baseURL:    cfg.BaseURL,
		httpClient: &http.Client{},
		ossConfig:  cfg.OSSConfig,
		taskID:     "", // 初始化为空字符串
	}
}

// GetOSSConfig 获取当前的 OSS 配置
func (c *Client) GetOSSConfig() *OSSConfig {
	return c.ossConfig
}

// SetOSSConfig 设置 OSS 配置
func (c *Client) SetOSSConfig(config *OSSConfig) {
	c.ossConfig = config
}

// ASRRequest 语音识别请求参数结构体
type ASRRequest struct {
	Format              string `json:"format"`               // 音频格式（如 "pcm"、"wav"）
	SampleRate          int    `json:"sample_rate"`          // 采样率（Hz）
	EnablePunctuation   bool   `json:"enable_punctuation"`   // 是否启用标点符号预测
	EnableNormalization bool   `json:"enable_normalization"` // 是否启用文本正规化
	TaskID              string `json:"task_id"`              // 任务ID，用于跟踪识别任务
}

// ASRResponse 语音识别响应结构体
type ASRResponse struct {
	Success bool   `json:"success"` // 识别是否成功
	Message string `json:"message"` // 响应消息
	Result  string `json:"result"`  // 识别结果文本
	TaskID  string `json:"task_id"` // 任务ID
}

// AudioPreprocessRequest 音频预处理请求参数
type AudioPreprocessRequest struct {
	Format      string `json:"format"`       // 目标格式 (wav)
	SampleRate  int    `json:"sample_rate"`  // 采样率 (16000Hz)
	Channels    int    `json:"channels"`     // 声道数 (1=单声道)
	SampleWidth int    `json:"sample_width"` // 采样位数 (2=16bit)
}

// AudioPreprocessResponse 音频预处理响应
type AudioPreprocessResponse struct {
	Code    int    `json:"code"`    // 响应状态码
	Success bool   `json:"success"` // 请求是否成功
	Message string `json:"message"` // 响应消息
	Data    struct {
		URL      string  `json:"url"`       // 处理后的音频文件URL
		Duration int     `json:"duration"`  // 音频时长（秒）
		FileSize float64 `json:"file_size"` // 文件大小（MB）
	} `json:"data"`
}

// AudioValidationError 音频验证错误
type AudioValidationError struct {
	Message string
}

func (e *AudioValidationError) Error() string {
	return e.Message
}

// ValidateAudioFile 验证音频文件
func ValidateAudioFile(filepath string) error {
	// 检查文件是否存在
	info, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AudioValidationError{Message: "文件不存在"}
		}
		return &AudioValidationError{Message: fmt.Sprintf("获取文件信息失败: %v", err)}
	}

	// 检查文件大小（最大5GB）
	const maxSize = 5 * 1024 * 1024 * 1024 // 5GB in bytes
	if info.Size() > maxSize {
		return &AudioValidationError{Message: fmt.Sprintf("文件大小超过限制，最大允许5GB，当前大小%.2fGB", float64(info.Size())/1024/1024/1024)}
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath[strings.LastIndex(filepath, ".")+1:])
	supportedFormats := map[string]bool{
		"wav": true,
		"mp3": true,
		"pcm": true,
		"m4a": true,
		"aac": true,
	}

	if !supportedFormats[ext] {
		return &AudioValidationError{Message: fmt.Sprintf("不支持的音频格式: %s，支持的格式: wav, mp3, pcm, m4a, aac", ext)}
	}

	return nil
}

// 生成认证签名
func (c *Client) generateSignature(timestamp string) string {
	// 签名格式：MD5(appKey + timestamp + appSecret)
	data := c.appKey + timestamp + c.appSecret
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// RecognizeSpeech 执行语音识别
// audioData: 音频数据字节数组
// req: 识别请求参数
// 返回识别结果和可能的错误
func (c *Client) RecognizeSpeech(audioData []byte, req *ASRRequest) (*ASRResponse, error) {
	url := fmt.Sprintf("%s/asr", c.baseURL)

	// 创建请求体
	r := bytes.NewReader(audioData)

	// 创建 HTTP 请求
	request, err := http.NewRequest("POST", url, r)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 生成时间戳和签名
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := c.generateSignature(timestamp)

	// 设置请求头
	request.Header.Set("X-App-Key", c.appKey)
	request.Header.Set("X-Timestamp", timestamp)
	request.Header.Set("X-Signature", signature)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Content-Length", fmt.Sprintf("%d", len(audioData)))

	// 添加查询参数
	q := request.URL.Query()
	q.Add("format", req.Format)
	q.Add("sample_rate", fmt.Sprintf("%d", req.SampleRate))
	q.Add("enable_punctuation", fmt.Sprintf("%v", req.EnablePunctuation))
	q.Add("enable_normalization", fmt.Sprintf("%v", req.EnableNormalization))
	if req.TaskID != "" {
		q.Add("task_id", req.TaskID)
	}
	request.URL.RawQuery = q.Encode()

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var result ASRResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &result, nil
}

// RecognizeFile 从文件执行语音识别（新的方法名，更接近用户习惯）
// filepath: 音频文件路径
// taskID: 任务ID（可选）
// 返回识别结果和可能的错误
func (c *Client) RecognizeFile(filepath string, taskID string) (*ASRResponse, error) {
	// 读取文件
	audioData, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	// 根据文件扩展名判断格式
	format := "pcm"
	if len(filepath) > 4 {
		ext := filepath[len(filepath)-4:]
		if ext == ".wav" {
			format = "wav"
		}
	}

	// 创建请求参数
	req := &ASRRequest{
		Format:              format,
		SampleRate:          16000, // 默认采样率
		EnablePunctuation:   true,
		EnableNormalization: true,
		TaskID:              taskID,
	}

	return c.RecognizeSpeech(audioData, req)
}

// GetOSSToken 获取OSS临时访问凭证
func (c *Client) GetOSSToken(requestID string, taskID string) (*OSSTokenResponse, error) {
	apiURL := fmt.Sprintf("%s/sts", c.baseURL)

	// 创建请求体
	reqBody := map[string]interface{}{
		"app_key":    c.appKey,
		"app_secret": c.appSecret,
		"request_id": requestID,
		"task_id":    taskID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求体失败: %v", err)
	}

	// 创建 HTTP 请求
	request, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	request.Header.Set("Content-Type", "application/json")

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var result OSSTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查响应状态
	if result.Code != 200 {
		return nil, fmt.Errorf("获取STS凭证失败: %s", result.Msg)
	}

	// 更新客户端的 OSS 配置
	if c.ossConfig != nil {
		c.ossConfig.Endpoint = result.Data.Endpoint
		c.ossConfig.BucketName = result.Data.Bucket
		c.ossConfig.Credentials = result.Data.Credentials
	}

	return &result, nil
}

// ProcessAudio 处理音频文件
func (c *Client) ProcessAudio(inputFile string) (string, error) {
	// 创建临时目录，使用filepath包处理路径
	tempDir := "temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}

	// 使用filepath包处理路径，确保跨平台兼容
	outputFile := filepath.Join(tempDir, fmt.Sprintf("processed_%d.wav", time.Now().UnixNano()))

	// 打开源文件
	srcFile, err := os.OpenFile(inputFile, os.O_RDONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("打开音频文件失败: %v", err)
	}
	defer srcFile.Close()

	// 获取源文件大小
	fileInfo, err := srcFile.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 创建目标文件
	dstFile, err := os.OpenFile(outputFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("创建输出文件失败: %v", err)
	}
	defer dstFile.Close()

	// 使用缓冲读写
	bufReader := bufio.NewReaderSize(srcFile, 1024*1024) // 1MB buffer
	bufWriter := bufio.NewWriterSize(dstFile, 1024*1024) // 1MB buffer
	defer bufWriter.Flush()

	// 读取WAV头部（44字节）
	header := make([]byte, 44)
	_, err = io.ReadFull(bufReader, header)
	if err != nil {
		return "", fmt.Errorf("读取WAV头部失败: %v", err)
	}

	// 检查并修改WAV头部
	if string(header[0:4]) == "RIFF" && string(header[8:12]) == "WAVE" {
		// 设置采样率为16000Hz (字节24-27)
		binary.LittleEndian.PutUint32(header[24:28], 16000)

		// 设置为单声道 (字节22-23)
		header[22] = 1
		header[23] = 0

		// 更新每秒数据字节数 (字节28-31)
		// 计算公式：采样率 * 通道数 * 位深度/8
		binary.LittleEndian.PutUint32(header[28:32], 16000*1*16/8)

		// 更新数据块大小 (字节16-19)
		binary.LittleEndian.PutUint32(header[16:20], 16)

		// 更新RIFF块大小 (字节4-7)
		binary.LittleEndian.PutUint32(header[4:8], uint32(fileInfo.Size()-8))
	}

	// 写入修改后的头部
	_, err = bufWriter.Write(header)
	if err != nil {
		return "", fmt.Errorf("写入WAV头部失败: %v", err)
	}

	// 使用较小的缓冲区复制数据，避免占用过多内存
	buf := make([]byte, 256*1024) // 256KB buffer
	_, err = io.CopyBuffer(bufWriter, bufReader, buf)
	if err != nil {
		return "", fmt.Errorf("复制音频数据失败: %v", err)
	}

	// 确保所有数据都写入磁盘
	if err := bufWriter.Flush(); err != nil {
		return "", fmt.Errorf("刷新缓冲区失败: %v", err)
	}

	if err := dstFile.Sync(); err != nil {
		return "", fmt.Errorf("同步文件失败: %v", err)
	}

	return outputFile, nil
}

// UploadFile 上传文件到 OSS
func (c *Client) UploadFile(filepath string, objectName string, requestID string, taskID string) (*UploadFileResponse, error) {
	// 1. 验证音频文件
	if err := ValidateAudioFile(filepath); err != nil {
		return nil, fmt.Errorf("音频文件验证失败: %v", err)
	}

	// 2. 处理音频
	processedFile, err := c.ProcessAudio(filepath)
	if err != nil {
		// 如果处理失败，尝试清理临时目录
		os.RemoveAll("temp")
		return nil, fmt.Errorf("音频处理失败: %v", err)
	}

	// 确保在函数返回时清理所有临时文件和目录
	defer func() {
		os.Remove(processedFile)
		// 尝试删除temp目录
		os.RemoveAll("temp")
	}()

	// 3. 获取 OSS 临时凭证
	tokenResp, err := c.GetOSSToken(requestID, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS 凭证失败: %v", err)
	}

	// 4. 创建 OSS 客户端
	client, err := oss.New(
		c.ossConfig.Endpoint,
		tokenResp.Data.Credentials.AccessKeyId,
		tokenResp.Data.Credentials.AccessKeySecret,
		oss.SecurityToken(tokenResp.Data.Credentials.SecurityToken),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %v", err)
	}

	// 5. 获取存储桶
	bucket, err := client.Bucket(c.ossConfig.BucketName)
	if err != nil {
		return nil, fmt.Errorf("获取存储桶失败: %v", err)
	}

	// 如果没有提供对象名称，则使用文件名
	if objectName == "" {
		// 使用filepath包获取文件名，确保跨平台兼容
		objectName = filepath.Base(filepath)
		// 添加时间戳前缀，使用path包处理OSS路径（使用正斜杠）
		objectName = path.Join("audio", fmt.Sprintf("%d_%s.wav", time.Now().Unix(), strings.TrimSuffix(objectName, filepath.Ext(filepath))))
	}

	// 6. 分片上传处理后的音频文件
	imur, err := bucket.InitiateMultipartUpload(objectName)
	if err != nil {
		return nil, fmt.Errorf("初始化分片上传失败: %v", err)
	}

	// 打开文件并获取信息
	file, err := os.OpenFile(processedFile, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 使用较大的分片大小，减少分片数量
	partSize := int64(20 * 1024 * 1024) // 20MB 分片大小
	numParts := (fileInfo.Size() + partSize - 1) / partSize

	// 创建上传进度条，降低刷新频率
	bar := progressbar.NewOptions64(
		fileInfo.Size(),
		progressbar.OptionSetDescription("上传文件"),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionThrottle(500*time.Millisecond), // 降低进度条更新频率到500ms
	)

	// 创建固定大小的缓冲区，避免频繁的内存分配
	buffer := make([]byte, partSize)

	// 串行上传分片，避免并发带来的资源竞争
	var parts []oss.UploadPart
	for i := int64(1); i <= numParts; i++ {
		start := (i - 1) * partSize
		size := partSize
		if i == numParts {
			if size = fileInfo.Size() - start; size < 0 {
				size = 0
			}
		}

		// 创建分片读取器
		partReader := io.NewSectionReader(file, start, size)

		// 使用固定大小的缓冲区读取数据
		n, err := io.ReadFull(partReader, buffer[:size])
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("读取分片 %d 失败: %v", i, err)
		}

		// 创建缓冲区读取器
		reader := bytes.NewReader(buffer[:n])

		// 上传分片
		part, err := bucket.UploadPart(imur, reader, int64(n), int(i))
		if err != nil {
			bucket.AbortMultipartUpload(imur)
			return nil, fmt.Errorf("上传分片 %d 失败: %v", i, err)
		}

		parts = append(parts, part)
		bar.Add64(size)

		// 添加短暂延时，让出系统资源
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println() // 换行

	// 完成分片上传
	completeResult, err := bucket.CompleteMultipartUpload(imur, parts)
	if err != nil {
		return nil, fmt.Errorf("完成分片上传失败: %v", err)
	}

	// 7. 生成文件访问URL（1小时有效期）
	signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, 3600)
	if err != nil {
		return nil, fmt.Errorf("生成文件访问URL失败: %v", err)
	}

	return &UploadFileResponse{
		Success:   true,
		Message:   fmt.Sprintf("文件上传成功，ETag: %s", completeResult.ETag),
		URL:       signedURL,
		RequestID: requestID,
	}, nil
}

// PreprocessAudio 音频预处理
// filepath: 本地音频文件路径
// req: 预处理参数
// 返回预处理结果和可能的错误
func (c *Client) PreprocessAudio(filepath string, req *AudioPreprocessRequest) (*AudioPreprocessResponse, error) {
	// 验证音频文件
	if err := ValidateAudioFile(filepath); err != nil {
		return nil, err
	}

	// 如果没有提供请求参数，使用默认值
	if req == nil {
		req = &AudioPreprocessRequest{
			Format:      "wav",
			SampleRate:  16000, // 16kHz
			Channels:    1,     // 单声道
			SampleWidth: 2,     // 16bit
		}
	}

	// 创建预处理请求
	preprocessURL := fmt.Sprintf("%s/preprocess_audio", c.baseURL)
	reqBody := map[string]interface{}{
		"filepath":     filepath,
		"format":       req.Format,
		"sample_rate":  req.SampleRate,
		"channels":     req.Channels,
		"sample_width": req.SampleWidth,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求体失败: %v", err)
	}

	// 创建 HTTP 请求
	request, err := http.NewRequest("POST", preprocessURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 生成时间戳和签名
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := c.generateSignature(timestamp)

	// 设置请求头
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-App-Key", c.appKey)
	request.Header.Set("X-Timestamp", timestamp)
	request.Header.Set("X-Signature", signature)

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 打印调试信息
	fmt.Printf("预处理请求URL: %s\n", preprocessURL)
	fmt.Printf("状态码: %d\n", response.StatusCode)
	fmt.Printf("响应内容: %s\n", string(body))

	// 解析响应
	var result AudioPreprocessResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, 响应内容: %s", err, string(body))
	}

	// 检查响应状态
	if !result.Success || result.Code != 200 {
		return nil, fmt.Errorf("预处理失败: %s", result.Message)
	}

	return &result, nil
}

// UploadPreprocessedAudio 上传预处理后的音频文件
func (c *Client) UploadPreprocessedAudio(preprocessedFilePath string, objectName string, requestID string, taskID string) (*UploadFileResponse, error) {
	// 获取 OSS 临时凭证
	tokenResp, err := c.GetOSSToken(requestID, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取 OSS 凭证失败: %v", err)
	}

	// 确保使用原始的 requestID
	if requestID == "" {
		requestID = tokenResp.Data.RequestID // 只在原始 requestID 为空时才使用响应中的 requestID
	}

	// 创建 OSS 客户端
	client, err := oss.New(
		c.ossConfig.Endpoint,
		tokenResp.Data.Credentials.AccessKeyId,
		tokenResp.Data.Credentials.AccessKeySecret,
		oss.SecurityToken(tokenResp.Data.Credentials.SecurityToken),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %v", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(c.ossConfig.BucketName)
	if err != nil {
		return nil, fmt.Errorf("获取存储桶失败: %v", err)
	}

	// 如果没有提供对象名称，则使用文件名
	if objectName == "" {
		objectName = preprocessedFilePath[strings.LastIndex(preprocessedFilePath, "/")+1:]
		if runtime.GOOS == "windows" {
			objectName = preprocessedFilePath[strings.LastIndex(preprocessedFilePath, "\\")+1:]
		}
	}

	// 上传文件
	err = bucket.PutObjectFromFile(objectName, preprocessedFilePath)
	if err != nil {
		return nil, fmt.Errorf("上传文件失败: %v", err)
	}

	// 生成文件访问URL
	signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, 3600) // 1小时有效期
	if err != nil {
		return nil, fmt.Errorf("生成文件访问URL失败: %v", err)
	}

	return &UploadFileResponse{
		Success:   true,
		Message:   "预处理音频文件上传成功",
		URL:       signedURL,
		RequestID: requestID,
	}, nil
}

// UploadPartInfo 分片上传信息
type UploadPartInfo struct {
	PartNumber int    // 分片号
	ETag       string // 分片的ETag
}

// MultipartUploadFile 分片上传文件到OSS
func (c *Client) MultipartUploadFile(filepath string, objectName string, requestID string) (*UploadFileResponse, error) {
	// 首先获取OSS临时凭证
	tokenResp, err := c.GetOSSToken(requestID, "")
	if err != nil {
		return nil, fmt.Errorf("获取OSS凭证失败: %v", err)
	}

	// 确保使用原始的 requestID
	if requestID == "" {
		requestID = tokenResp.Data.RequestID // 只在原始 requestID 为空时才使用响应中的 requestID
	}

	// 创建OSS客户端
	ossClient, err := oss.New(
		c.ossConfig.Endpoint,
		tokenResp.Data.Credentials.AccessKeyId,
		tokenResp.Data.Credentials.AccessKeySecret,
		oss.SecurityToken(tokenResp.Data.Credentials.SecurityToken),
	)
	if err != nil {
		return nil, fmt.Errorf("创建OSS客户端失败: %v", err)
	}

	// 获取存储桶
	bucket, err := ossClient.Bucket(c.ossConfig.BucketName)
	if err != nil {
		return nil, fmt.Errorf("获取存储桶失败: %v", err)
	}

	// 如果没有提供对象名称，则使用文件名
	if objectName == "" {
		objectName = filepath[strings.LastIndex(filepath, "/")+1:]
		if runtime.GOOS == "windows" {
			objectName = filepath[strings.LastIndex(filepath, "\\")+1:]
		}
	}

	// 初始化分片上传
	imur, err := bucket.InitiateMultipartUpload(objectName)
	if err != nil {
		return nil, fmt.Errorf("初始化分片上传失败: %v", err)
	}

	// 获取文件大小
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}
	fileSize := fileInfo.Size()

	// 计算分片大小和数量
	partSize := int64(5 * 1024 * 1024) // 5MB per part
	numParts := (fileSize + partSize - 1) / partSize

	// 打开文件
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 上传分片
	var parts []oss.UploadPart
	for i := int64(1); i <= numParts; i++ {
		start := (i - 1) * partSize
		size := partSize
		if i == numParts {
			if size = fileSize - start; size < 0 {
				size = 0
			}
		}

		part, err := bucket.UploadPart(imur, file, size, int(i))
		if err != nil {
			// 取消分片上传
			bucket.AbortMultipartUpload(imur)
			return nil, fmt.Errorf("上传分片 %d 失败: %v", i, err)
		}
		parts = append(parts, part)
	}

	// 完成分片上传
	completeResult, err := bucket.CompleteMultipartUpload(imur, parts)
	if err != nil {
		return nil, fmt.Errorf("完成分片上传失败: %v", err)
	}

	// 生成文件访问URL
	signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, 3600) // 1小时有效期
	if err != nil {
		return nil, fmt.Errorf("生成文件访问URL失败: %v", err)
	}

	return &UploadFileResponse{
		Success:   true,
		Message:   fmt.Sprintf("文件上传成功，ETag: %s", completeResult.ETag),
		URL:       signedURL,
		RequestID: requestID,
	}, nil
}

// SplitAudioResult 音频文件拆分结果
type SplitAudioResult struct {
	Success    bool     `json:"success"`     // 拆分是否成功
	Message    string   `json:"message"`     // 结果消息
	FilePaths  []string `json:"file_paths"`  // 本地文件路径列表
	TotalParts int      `json:"total_parts"` // 总分片数
}

// SplitAudioLocal 将音频文件拆分到本地
func (c *Client) SplitAudioLocal(filepath string) (*SplitAudioResult, error) {
	// 验证音频文件
	if err := ValidateAudioFile(filepath); err != nil {
		return nil, err
	}

	// 打开源文件
	srcFile, err := os.OpenFile(filepath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开源文件失败: %v", err)
	}
	defer srcFile.Close()

	// 读取WAV头部（44字节）
	header := make([]byte, 44)
	n, err := io.ReadFull(srcFile, header)
	if err != nil {
		return nil, fmt.Errorf("读取WAV头部失败: %v, 实际读取字节数: %d", err, n)
	}

	// 验证WAV格式
	if string(header[0:4]) != "RIFF" {
		return nil, fmt.Errorf("无效的WAV文件格式: 缺少RIFF标识")
	}
	if string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("无效的WAV文件格式: 缺少WAVE标识")
	}

	// 获取文件大小
	fileInfo, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}
	totalSize := fileInfo.Size()
	dataSize := totalSize - 44 // 减去WAV头部大小

	// 计算每个分片的实际音频数据大小（100MB - WAV头部大小）
	chunkDataSize := int64(100*1024*1024) - 44
	numChunks := (dataSize + chunkDataSize - 1) / chunkDataSize

	// 重置文件指针到头部
	_, err = srcFile.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("重置文件指针失败: %v", err)
	}

	// 创建临时目录
	ext := filepath[strings.LastIndex(filepath, "."):]
	tempDir := filepath[:len(filepath)-len(ext)] + "_parts"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %v", err)
	}

	var filePaths []string

	// 拆分文件
	for i := int64(0); i < numChunks; i++ {
		// 计算当前分片的实际数据大小
		currentDataSize := chunkDataSize
		if i == numChunks-1 {
			currentDataSize = dataSize - (i * chunkDataSize)
		}

		// 创建新的WAV头部
		newHeader := make([]byte, 44)
		copy(newHeader, header)

		// 更新头部信息
		// RIFF块大小 = 文件总大小 - 8
		binary.LittleEndian.PutUint32(newHeader[4:8], uint32(currentDataSize+36))
		// 数据块大小
		binary.LittleEndian.PutUint32(newHeader[40:44], uint32(currentDataSize))

		// 获取文件扩展名和基础名称
		ext := filepath[strings.LastIndex(filepath, "."):]
		baseName := filepath[:len(filepath)-len(ext)]

		// 创建分片文件
		partPath := fmt.Sprintf("%s_part_%d.wav", baseName, i+1)
		partFile, err := os.Create(partPath)
		if err != nil {
			return nil, fmt.Errorf("创建分片文件失败: %v", err)
		}

		// 写入WAV头部
		if _, err := partFile.Write(newHeader); err != nil {
			partFile.Close()
			return nil, fmt.Errorf("写入WAV头部失败: %v", err)
		}

		// 复制音频数据
		if _, err := io.CopyN(partFile, srcFile, currentDataSize); err != nil && err != io.EOF {
			partFile.Close()
			return nil, fmt.Errorf("复制音频数据失败: %v", err)
		}

		partFile.Close()
		filePaths = append(filePaths, partPath)
	}

	return &SplitAudioResult{
		Success:    true,
		Message:    fmt.Sprintf("音频文件已成功拆分为 %d 个部分", numChunks),
		FilePaths:  filePaths,
		TotalParts: int(numChunks),
	}, nil
}

// UploadSplitFiles 上传拆分后的文件到OSS
func (c *Client) UploadSplitFiles(filePaths []string, requestID string) ([]string, error) {
	// 获取 OSS 临时凭证
	tokenResp, err := c.GetOSSToken(requestID, "")
	if err != nil {
		return nil, err
	}

	// 创建 OSS 客户端
	client, err := oss.New(
		c.ossConfig.Endpoint,
		tokenResp.Data.Credentials.AccessKeyId,
		tokenResp.Data.Credentials.AccessKeySecret,
		oss.SecurityToken(tokenResp.Data.Credentials.SecurityToken),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %v", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(c.ossConfig.BucketName)
	if err != nil {
		return nil, fmt.Errorf("获取存储桶失败: %v", err)
	}

	var ossUrls []string

	// 上传每个分片文件
	for i, filePath := range filePaths {
		// 生成OSS对象名称
		objectName := fmt.Sprintf("audio/%s/part_%d.wav", requestID, i+1)

		// 上传文件
		err = bucket.PutObjectFromFile(objectName, filePath)
		if err != nil {
			return nil, fmt.Errorf("上传分片 %d 失败: %v", i+1, err)
		}

		// 生成文件访问URL（1小时有效期）
		signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, 3600)
		if err != nil {
			return nil, fmt.Errorf("生成文件访问URL失败: %v", err)
		}
		ossUrls = append(ossUrls, signedURL)
	}

	return ossUrls, nil
}

// SplitAudioFileResult 音频文件拆分结果
type SplitAudioFileResult struct {
	Success    bool     `json:"success"`     // 拆分是否成功
	Message    string   `json:"message"`     // 结果消息
	OssUrls    []string `json:"oss_urls"`    // OSS文件URL列表
	TotalParts int      `json:"total_parts"` // 总分片数
	RequestID  string   `json:"request_id"`  // 请求ID
}

// FileInfo 文件信息结构体
type FileInfo struct {
	FileName   string `json:"file_name"`   // 文件名
	FileURL    string `json:"file_url"`    // OSS URL
	FileSize   int64  `json:"file_size"`   // 文件大小（字节）
	Duration   int64  `json:"duration"`    // 音频时长（毫秒）
	PartIndex  int    `json:"part_index"`  // 分片索引
	TotalParts int    `json:"total_parts"` // 总分片数
}

// FileListRequest 文件列表请求结构体
type FileListRequest struct {
	RequestID  string     `json:"request_id"`  // 请求ID
	TaskID     string     `json:"task_id"`     // 任务ID
	Files      []FileInfo `json:"files"`       // 文件信息列表
	TotalSize  int64      `json:"total_size"`  // 总文件大小
	TotalParts int        `json:"total_parts"` // 总分片数
	ModelType  string     `json:"model_type"`  // 模型类型：beat2b/highaccur
	SampleRate int        `json:"sample_rate"` // 采样率
	Channels   int        `json:"channels"`    // 声道数
}

// FileListResponse 文件列表响应结构体
type FileListResponse struct {
	Code int    `json:"code"` // 响应码
	Data bool   `json:"data"` // 响应数据
	Msg  string `json:"msg"`  // 响应消息
}

// AudioConfig 音频配置
type AudioConfig struct {
	SampleRate int `json:"sample_rate"` // 采样率（Hz）
	Channels   int `json:"channels"`    // 声道数
}

// ModelConfig 模型配置
type ModelConfig struct {
	Type string `json:"type"` // 模型类型：beat2b(高性能)或highaccur(高精度)
}

// ProcessRequest 音频处理请求
type ProcessRequest struct {
	AppKey      string      `json:"app_key"`      // API密钥
	AppSecret   string      `json:"app_secret"`   // API密钥密文
	RequestID   string      `json:"request_id"`   // 请求ID
	TaskID      string      `json:"task_id"`      // 用户任务ID
	AudioConfig AudioConfig `json:"audio_config"` // 音频配置
	ModelConfig ModelConfig `json:"model_config"` // 模型配置
	FilePath    string      `json:"file_path"`    // OSS中的音频文件路径
}

// ProcessResponse 音频处理响应
type ProcessResponse struct {
	Code int    `json:"code"` // 状态码：200成功，400参数错误，500服务器错误
	Msg  string `json:"msg"`  // 响应消息
	Data bool   `json:"data"` // 处理结果：true表示成功，false表示失败
}

// setCommonHeaders 设置通用请求头和认证信息
func (c *Client) setCommonHeaders(request *http.Request, requestID string, taskID string) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := c.generateSignature(timestamp)
	effectiveTaskID := c.getEffectiveTaskID(taskID)

	// 设置通用请求头
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-App-Key", c.appKey)
	request.Header.Set("X-Timestamp", timestamp)
	request.Header.Set("X-Signature", signature)
	request.Header.Set("X-Request-ID", requestID)

	// 总是设置任务ID，即使是空字符串
	request.Header.Set("X-Task-ID", effectiveTaskID)
}

// addCommonFields 添加通用请求体字段
func (c *Client) addCommonFields(reqBody map[string]interface{}, requestID string, taskID string) {
	reqBody["app_key"] = c.appKey
	reqBody["app_secret"] = c.appSecret
	reqBody["request_id"] = requestID

	// 总是添加任务ID字段，即使是空字符串
	effectiveTaskID := c.getEffectiveTaskID(taskID)
	reqBody["task_id"] = effectiveTaskID
}

func (c *Client) SendFileList(req *FileListRequest) (*FileListResponse, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/run", c.baseURL)

	// 创建完整的请求体
	reqBody := map[string]interface{}{
		"files":       req.Files,
		"total_size":  req.TotalSize,
		"total_parts": req.TotalParts,
		"model_type":  req.ModelType,  // 添加模型类型
		"sample_rate": req.SampleRate, // 添加采样率
		"channels":    req.Channels,   // 添加声道数
	}

	// 添加通用字段
	c.addCommonFields(reqBody, req.RequestID, req.TaskID)

	// 将请求体转换为JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 打印完整的请求信息用于调试
	fmt.Printf("发送文件列表请求: %+v\n", reqBody)

	// 创建HTTP请求
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置通用请求头
	c.setCommonHeaders(request, req.RequestID, req.TaskID)

	// 设置额外的请求头
	request.Header.Set("X-Total-Parts", fmt.Sprintf("%d", req.TotalParts))
	request.Header.Set("X-Total-Size", fmt.Sprintf("%d", req.TotalSize))

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var result FileListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, 响应内容: %s", err, string(body))
	}

	// 检查响应状态码
	if result.Code != 200 {
		return nil, fmt.Errorf("服务端拒绝文件列表: %s", result.Msg)
	}

	return &result, nil
}

// SendSingleFile 发送单个文件处理请求
func (c *Client) SendSingleFile(fileInfo FileInfo, requestID string, taskID string, modelType string, sampleRate int, channels int) (*ProcessResponse, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/run", c.baseURL)

	// 构建文件路径
	filePath := fileInfo.FileName

	// 创建请求体
	req := &ProcessRequest{
		AppKey:    c.appKey,
		AppSecret: c.appSecret,
		RequestID: requestID,
		TaskID:    taskID,
		AudioConfig: AudioConfig{
			SampleRate: sampleRate,
			Channels:   channels,
		},
		ModelConfig: ModelConfig{
			Type: modelType,
		},
		FilePath: filePath, // 添加文件路径字段
	}

	// 将请求体转换为JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	request.Header.Set("Content-Type", "application/json")

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var result ProcessResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, 响应内容: %s", err, string(body))
	}

	// 检查响应状态码
	if result.Code != 200 {
		return nil, fmt.Errorf("服务端拒绝文件处理请求: %s", result.Msg)
	}

	return &result, nil
}

// SplitAudioFile 将音频文件拆分并直接上传到OSS
func (c *Client) SplitAudioFile(filepath string, requestID string, taskID string) (*SplitAudioFileResult, error) {
	// 验证音频文件
	if err := ValidateAudioFile(filepath); err != nil {
		return nil, err
	}

	// 获取 OSS 临时凭证
	tokenResp, err := c.GetOSSToken(requestID, taskID)
	if err != nil {
		return nil, err
	}

	// 确保使用原始的 requestID
	if requestID == "" {
		requestID = tokenResp.Data.RequestID
	}

	// 创建 OSS 客户端
	client, err := oss.New(
		c.ossConfig.Endpoint,
		tokenResp.Data.Credentials.AccessKeyId,
		tokenResp.Data.Credentials.AccessKeySecret,
		oss.SecurityToken(tokenResp.Data.Credentials.SecurityToken),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 OSS 客户端失败: %v", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(c.ossConfig.BucketName)
	if err != nil {
		return nil, fmt.Errorf("获取存储桶失败: %v", err)
	}

	// 打开源文件
	srcFile, err := os.OpenFile(filepath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开源文件失败: %v", err)
	}
	defer srcFile.Close()

	// 读取WAV头部（44字节）
	header := make([]byte, 44)
	n, err := io.ReadFull(srcFile, header)
	if err != nil {
		return nil, fmt.Errorf("读取WAV头部失败: %v, 实际读取字节数: %d", err, n)
	}

	// 验证WAV格式
	if string(header[0:4]) != "RIFF" {
		return nil, fmt.Errorf("无效的WAV文件格式: 缺少RIFF标识")
	}
	if string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("无效的WAV文件格式: 缺少WAVE标识")
	}

	// 获取文件大小
	fileInfo, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}
	totalSize := fileInfo.Size()
	dataSize := totalSize - 44 // 减去WAV头部大小

	// 计算每个分片的实际音频数据大小（100MB - WAV头部大小）
	chunkDataSize := int64(100*1024*1024) - 44
	numChunks := (dataSize + chunkDataSize - 1) / chunkDataSize

	// 重置文件指针到头部
	_, err = srcFile.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("重置文件指针失败: %v", err)
	}

	var ossUrls []string
	var fileInfos []FileInfo

	// 拆分并上传文件
	for i := int64(0); i < numChunks; i++ {
		// 计算当前分片的实际数据大小
		currentDataSize := chunkDataSize
		if i == numChunks-1 {
			currentDataSize = dataSize - (i * chunkDataSize)
		}

		// 创建新的WAV头部
		newHeader := make([]byte, 44)
		copy(newHeader, header)

		// 更新头部信息
		binary.LittleEndian.PutUint32(newHeader[4:8], uint32(currentDataSize+36))
		binary.LittleEndian.PutUint32(newHeader[40:44], uint32(currentDataSize))

		// 创建分片缓冲区
		chunkBuffer := bytes.NewBuffer(make([]byte, 0, currentDataSize+44))

		// 写入WAV头部
		chunkBuffer.Write(newHeader)

		// 复制音频数据
		_, err = io.CopyN(chunkBuffer, srcFile, currentDataSize)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("复制音频数据失败: %v", err)
		}

		// 生成OSS对象名称，使用path包处理路径分隔符
		objectName := path.Join("audio", requestID, fmt.Sprintf("part_%d.wav", i+1))

		// 上传到OSS
		err = bucket.PutObject(objectName, bytes.NewReader(chunkBuffer.Bytes()))
		if err != nil {
			return nil, fmt.Errorf("上传分片 %d 失败: %v", i+1, err)
		}

		// 验证文件是否成功上传，重试最多3次
		maxRetries := 3
		uploadSuccess := false
		for retry := 0; retry < maxRetries; retry++ {
			exist, err := bucket.IsObjectExist(objectName)
			if err != nil {
				fmt.Printf("验证文件上传失败，重试 %d/%d: %v\n", retry+1, maxRetries, err)
				time.Sleep(time.Second * 2) // 等待2秒后重试
				continue
			}
			if !exist {
				fmt.Printf("文件不存在，重试 %d/%d\n", retry+1, maxRetries)
				time.Sleep(time.Second * 2) // 等待2秒后重试
				continue
			}
			uploadSuccess = true
			break
		}

		if !uploadSuccess {
			return nil, fmt.Errorf("文件上传验证失败，对象不存在: %s", objectName)
		}

		// 生成文件访问URL（1小时有效期）
		signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, 3600)
		if err != nil {
			return nil, fmt.Errorf("生成文件访问URL失败: %v", err)
		}
		ossUrls = append(ossUrls, signedURL)

		// 创建文件信息
		fileInfo := FileInfo{
			FileName:   objectName,
			FileURL:    signedURL,
			FileSize:   currentDataSize + 44,
			PartIndex:  int(i + 1),
			TotalParts: int(numChunks),
			Duration:   0,
		}
		fileInfos = append(fileInfos, fileInfo)
	}

	// 所有文件上传完成后，发送处理请求
	for _, fileInfo := range fileInfos {
		// 再次验证文件是否存在
		exist, err := bucket.IsObjectExist(fileInfo.FileName)
		if err != nil {
			return nil, fmt.Errorf("处理前验证文件失败: %v", err)
		}
		if !exist {
			return nil, fmt.Errorf("处理前文件不存在: %s", fileInfo.FileName)
		}

		// 发送处理请求
		resp, err := c.SendSingleFile(fileInfo, requestID, taskID, ModelTypeBeat2B, 16000, 1)
		if err != nil {
			return nil, fmt.Errorf("发送文件处理请求失败: %v", err)
		}
		if !resp.Data {
			return nil, fmt.Errorf("服务端处理请求失败: %s", resp.Msg)
		}
		// 等待一小段时间再发送下一个请求
		time.Sleep(500 * time.Millisecond)
	}

	return &SplitAudioFileResult{
		Success:    true,
		Message:    fmt.Sprintf("音频文件已成功拆分为 %d 个部分并上传，所有文件已发送处理请求", numChunks),
		OssUrls:    ossUrls,
		TotalParts: int(numChunks),
		RequestID:  requestID,
	}, nil
}

// min returns the smaller of two int64 values
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// SetTaskID 设置当前任务ID
func (c *Client) SetTaskID(taskID string) {
	c.taskID = taskID
}

// GetTaskID 获取当前任务ID
func (c *Client) GetTaskID() string {
	return c.taskID
}

// getEffectiveTaskID 获取有效的任务ID，如果提供了参数taskID则使用参数值，否则使用客户端默认taskID
func (c *Client) getEffectiveTaskID(taskID string) string {
	if taskID != "" {
		return taskID
	}
	return c.taskID
}

// TaskProgress 任务进度信息
type TaskProgress struct {
	Success    bool        `json:"success"`     // 是否成功
	Message    string      `json:"message"`     // 消息
	Progress   float64     `json:"progress"`    // 处理进度（0-100）
	RequestID  string      `json:"request_id"`  // 请求ID
	TaskID     string      `json:"task_id"`     // 任务ID
	IsComplete bool        `json:"is_complete"` // 是否完成
	Result     interface{} `json:"result"`      // 处理结果，仅在完成时有值
	ModelType  string      `json:"model_type"`  // 模型类型：beat2b/highaccur
	SampleRate int         `json:"sample_rate"` // 采样率
	Channels   int         `json:"channels"`    // 声道数
}

// QueryProgress 查询任务进度
func (c *Client) QueryProgress(requestID string, taskID string, modelType string, sampleRate int, channels int) (*TaskProgress, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/task/result", c.baseURL)

	// 创建请求体
	reqBody := map[string]interface{}{
		"model_type":  modelType,
		"sample_rate": sampleRate,
		"channels":    channels,
	}
	c.addCommonFields(reqBody, requestID, taskID)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置通用请求头
	c.setCommonHeaders(request, requestID, taskID)

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 首先尝试解析为标准响应格式
	var standardResp struct {
		Code int         `json:"code"`
		Data interface{} `json:"data"`
		Msg  string      `json:"msg"`
	}
	if err := json.Unmarshal(body, &standardResp); err == nil {
		// 如果解析成功，检查响应码
		if standardResp.Code != 200 {
			// 返回一个带有服务端消息的 TaskProgress
			return &TaskProgress{
				Success:    false,
				Message:    standardResp.Msg,
				Progress:   0,
				RequestID:  requestID,
				TaskID:     taskID,
				IsComplete: false,
			}, nil
		}
	}

	// 如果是标准响应且成功，或者是直接的进度响应，尝试解析为 TaskProgress
	var result TaskProgress
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, 响应内容: %s", err, string(body))
	}

	// 确保基本字段有值
	if result.RequestID == "" {
		result.RequestID = requestID
	}
	if result.TaskID == "" {
		result.TaskID = taskID
	}

	return &result, nil
}

// WaitForComplete 等待任务完成并显示进度
func (c *Client) WaitForComplete(requestID string, taskID string, modelType string, sampleRate int, channels int, interval int, timeout int, progressCallback func(progress float64)) (*TaskProgress, error) {
	if interval <= 0 {
		interval = 2 // 默认2秒轮询一次
	}

	startTime := time.Now()
	lastProgress := float64(-1) // 初始化为-1以确保第一次一定会显示进度
	failureCount := 0

	for {
		// 检查是否超时
		if timeout > 0 && time.Since(startTime) > time.Duration(timeout)*time.Second {
			return nil, fmt.Errorf("等待任务完成超时")
		}

		// 查询进度
		resp, err := c.QueryTaskResult(requestID, taskID)
		if err != nil {
			failureCount++
			if failureCount >= 3 {
				return nil, fmt.Errorf("连续查询失败3次: %v", err)
			}
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		failureCount = 0 // 重置失败计数器

		// 检查是否有任务数据
		if len(resp.Data) == 0 {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		task := resp.Data[0]
		progress := task.Progress

		// 如果进度发生变化，打印新进度
		if progress != lastProgress {
			// 清除当前行
			fmt.Printf("\r                                        ")
			// 打印新进度
			fmt.Printf("\r当前进度: %.1f%%", progress)
			lastProgress = progress
		}

		// 如果提供了回调函数，调用它
		if progressCallback != nil {
			progressCallback(progress)
		}

		// 检查是否完成
		if task.IsCompleted {
			return &TaskProgress{
				Success:    true,
				Message:    "任务完成",
				Progress:   100,
				RequestID:  requestID,
				TaskID:     taskID,
				IsComplete: true,
				Result:     task.Result,
			}, nil
		}

		// 继续等待
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// 添加模型类型常量
const (
	ModelTypeBeat2B    = "beat2b"
	ModelTypeHighAccur = "highaccur"
)

// TaskResult 任务结果结构体
type TaskResult struct {
	TaskID       string          `json:"task_id"`       // 用户任务ID
	Progress     float64         `json:"progress"`      // 处理进度：0-100
	Result       json.RawMessage `json:"result"`        // 识别结果（JSON字符串）
	IsCompleted  bool            `json:"is_completed"`  // 是否完成
	TimeCreated  int64           `json:"time_created"`  // 创建时间（Unix时间戳）
	TimeModified int64           `json:"time_modified"` // 修改时间（Unix时间戳）
}

// TaskResultResponse 任务结果查询响应
type TaskResultResponse struct {
	Code int          `json:"code"` // 状态码：200成功，400参数错误，500服务器错误
	Msg  string       `json:"msg"`  // 响应消息
	Data []TaskResult `json:"data"` // 任务列表
}

// ConversationResult 对话内容结构体
type ConversationResult struct {
	StartTime string `json:"start_time"` // 开始时间
	EndTime   string `json:"end_time"`   // 结束时间
	Speaker   string `json:"speaker"`    // 说话人
	Text      string `json:"text"`       // 识别文本
}

// RecognitionResult 识别结果结构体
type RecognitionResult struct {
	ID            int64                `json:"id"`            // 系统任务ID
	Type          string               `json:"type"`          // 使用的模型类型
	Speakers      []string             `json:"speakers"`      // 说话人列表
	Conversations []ConversationResult `json:"conversations"` // 对话内容列表
	Summary       string               `json:"summary"`       // 总结（可选）
}

// QueryTaskResult 查询任务结果
func (c *Client) QueryTaskResult(requestID string, taskID string) (*TaskResultResponse, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/task/result", c.baseURL)

	// 创建请求体
	reqBody := map[string]interface{}{
		"app_key":    c.appKey,
		"app_secret": c.appSecret,
		"request_id": requestID,
		"task_id":    taskID,
	}

	// 将请求体转换为JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	request.Header.Set("Content-Type", "application/json")

	// 发送请求
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer response.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析响应
	var result TaskResultResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, 响应内容: %s", err, string(body))
	}

	// 检查响应状态码
	if result.Code != 200 {
		return nil, fmt.Errorf("查询任务结果失败: %s", result.Msg)
	}

	return &result, nil
}

// GetTaskResult 获取任务结果并解析为RecognitionResult
func (c *Client) GetTaskResult(requestID string, taskID string) (*RecognitionResult, error) {
	// 查询任务结果
	resp, err := c.QueryTaskResult(requestID, taskID)
	if err != nil {
		return nil, err
	}

	// 检查是否有任务数据
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("未找到任务数据")
	}

	// 获取第一个任务
	task := resp.Data[0]

	// 检查任务是否完成
	if !task.IsCompleted {
		return nil, fmt.Errorf("任务尚未完成，当前进度: %.1f%%", task.Progress)
	}

	// 解析识别结果
	var result RecognitionResult
	if err := json.Unmarshal(task.Result, &result); err != nil {
		return nil, fmt.Errorf("解析识别结果失败: %v", err)
	}

	return &result, nil
}
