package voiceworld

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"voiceworld-go-sdk/voiceworld/utils"
)

// AudioProcessor 音频处理器，封装音频处理的流程
type AudioProcessor struct {
	client *Client
	config ProcessorConfig
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxRetries     int           // 最大重试次数
	RetryInterval  time.Duration // 重试间隔
	ChunkSize      int64         // 文件切割大小（字节）
	ModelType      string        // 模型类型
	DeleteTempFile bool          // 是否删除临时文件
}

// DefaultProcessorConfig 默认处理器配置
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		MaxRetries:     30,
		RetryInterval:  2 * time.Second,
		ChunkSize:      1024 * 1024 * 100, // 100MB
		ModelType:      "beat2b",
		DeleteTempFile: true,
	}
}

// NewAudioProcessor 创建新的音频处理器
func NewAudioProcessor(client *Client, config ProcessorConfig) *AudioProcessor {
	return &AudioProcessor{
		client: client,
		config: config,
	}
}

// ProcessAudio 处理音频文件
func (p *AudioProcessor) ProcessAudio(ctx context.Context, audioPath string, userTaskID string) ([]string, error) {
	// 检查 FFmpeg 是否安装
	if !utils.CheckFFmpegInstalled() {
		return nil, fmt.Errorf("FFmpeg 未安装，请先安装 FFmpeg")
	}

	// 检查音频文件是否存在
	if err := utils.ValidateAudio(audioPath); err != nil {
		return nil, fmt.Errorf("音频文件不存在或格式不支持: %v", err)
	}

	// 转换音频文件
	convertedFile, err := utils.ConvertAudioFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("转换失败: %v", err)
	}
	fmt.Println("转换后的文件:", convertedFile)

	// 切割音频文件
	chunks, err := utils.SplitAudioFile(convertedFile, int(p.config.ChunkSize))
	if err != nil {
		return nil, fmt.Errorf("切割失败: %v", err)
	}
	fmt.Println("切割成功，生成的文件块:", chunks)

	// 删除原缓存文件
	if p.config.DeleteTempFile {
		os.Remove(convertedFile)
	}

	// 获取STS临时凭证
	stsResp, err := p.client.GetSTSCredentials(ctx, userTaskID)

	// 创建OSS配置
	ossConfig := utils.OSSConfig{
		Endpoint:        stsResp.Data.Endpoint,
		AccessKeyID:     stsResp.Data.Credentials.AccessKeyID,
		AccessKeySecret: stsResp.Data.Credentials.AccessKeySecret,
		SecurityToken:   stsResp.Data.Credentials.SecurityToken,
		BucketName:      stsResp.Data.Bucket,
	}

	// 生成当前时间戳，格式为 yyyyMMddHHmmss
	timestamp := time.Now().Format("20060102150405")

	// 上传所有切割后的文件
	results := make([]string, 0)
	for i, chunkFile := range chunks {
		result, err := p.processChunk(ctx, chunkFile, i+1, timestamp, ossConfig, stsResp.Data.RequestID, userTaskID)
		if err != nil {
			fmt.Printf("处理文件块 %s 失败: %v\n", chunkFile, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// processChunk 处理单个文件块
func (p *AudioProcessor) processChunk(
	ctx context.Context,
	chunkFile string,
	chunkIndex int,
	timestamp string,
	ossConfig utils.OSSConfig,
	requestID string,
	userTaskID string,
) (string, error) {
	fmt.Printf("开始处理文件块: %s\n", chunkFile)

	// 获取文件名
	fileName := filepath.Base(chunkFile)

	// 生成对象键（OSS中的文件路径）
	// 格式: audio/app_key/20240321123045/audio_part1.wav
	objectKey := fmt.Sprintf("audio/%s/%s/audio_part%d%s",
		p.client.config.AppKey,
		timestamp,
		chunkIndex,
		filepath.Ext(fileName))

	// 定义进度回调函数
	progressCallback := func(fileName string, totalBytes, uploadedBytes int64, percentage int) {
		// 只在整10%时打印进度，避免输出过多
		if percentage%10 == 0 {
			fmt.Printf("上传进度: %s - %d/%d 字节 (%d%%)\n",
				fileName, uploadedBytes, totalBytes, percentage)
		}
	}

	// 上传文件
	err := utils.UploadFileWithSTSAndCallback(ossConfig, chunkFile, objectKey, progressCallback)
	if err != nil {
		return "", fmt.Errorf("上传文件失败: %v", err)
	}

	// 构建文件URL
	fmt.Printf("文件 %s 上传成功", chunkFile)

	// 验证文件是否真实上传完成
	exists, err := utils.CheckFileExists(ossConfig, objectKey)
	if err != nil {
		return "", fmt.Errorf("检查文件状态失败: %v", err)
	}

	if !exists {
		return "", fmt.Errorf("文件上传失败")
	}

	// 删除临时文件
	if p.config.DeleteTempFile {
		os.Remove(chunkFile)
	}

	// 请求处理音频
	runResp, err := p.client.Run(ctx, requestID, userTaskID, objectKey, p.config.ModelType)
	if err != nil {
		return "", fmt.Errorf("请求处理音频失败: %v", err)
	}
	fmt.Println("请求处理音频成功")

	// 循环获取结果
	var result string
	for i := 0; i < p.config.MaxRetries; i++ {
		fmt.Printf("查询结果 (第 %d 次)...\n", i+1)

		resultResp, err := p.client.GetResult(ctx, requestID, userTaskID, runResp.Data.TaskID)
		if err != nil {
			fmt.Printf("查询结果失败: %v, 将在 %v 后重试\n", err, p.config.RetryInterval)
			time.Sleep(p.config.RetryInterval)
			continue
		}

		// 如果resultResp状态码为200，则获取结果并退出循环
		if resultResp.Code == 200 {
			result = resultResp.Data.Result
			break
		}

		// 如果为201，则等待后重试
		if resultResp.Code == 201 {
			fmt.Printf("结果未准备好，等待 %v 后重试...\n", p.config.RetryInterval)
			time.Sleep(p.config.RetryInterval)
			continue
		}

		// 其他错误码
		return "", fmt.Errorf("查询返回错误: %d - %s", resultResp.Code, resultResp.Msg)
	}

	return result, nil
}
