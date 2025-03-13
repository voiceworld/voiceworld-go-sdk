package main

import (
	"fmt"
	"os"
	"time"
	"voiceworld-go-sdk/voiceworld/client"
)

// 处理音频文件并返回结果
func processAudio(sdk *client.Client, audioFile, taskID string, modelType string, sampleRate, channels int) error {
	// 生成请求ID
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 获取OSS临时凭证
	if err := getOSSToken(sdk, requestID, taskID); err != nil {
		return err
	}

	// 处理音频文件
	if err := splitAudioFile(sdk, audioFile, requestID, taskID, modelType, sampleRate, channels); err != nil {
		return err
	}

	return nil
}

// 获取OSS临时凭证
func getOSSToken(sdk *client.Client, requestID, taskID string) error {
	_, err := sdk.GetOSSToken(requestID, taskID)
	if err != nil {
		fmt.Printf("获取OSS凭证失败: %v\n", err)
		return err
	}

	fmt.Printf("\n=== 系统初始化 ===\n")
	fmt.Printf("状态: 成功\n")
	fmt.Printf("任务ID: %s\n", taskID)
	fmt.Println("===============\n")
	return nil
}

// 分割音频文件
func splitAudioFile(sdk *client.Client, audioFile, requestID, taskID string, modelType string, sampleRate, channels int) error {
	fmt.Printf("开始处理音频文件: %s\n", audioFile)
	fmt.Printf("任务ID: %s\n", taskID)
	fmt.Printf("模型类型: %s\n", modelType)
	fmt.Printf("采样率: %d Hz\n", sampleRate)
	fmt.Printf("声道数: %d\n\n", channels)

	result, err := sdk.SplitAudioFile(audioFile, requestID, taskID)
	if err != nil {
		fmt.Printf("错误信息: %s\n", err.Error())
		if result != nil && result.Success {
			fmt.Println("\n任务已提交，开始处理...\n")
			return waitForResult(sdk, requestID, taskID, modelType, sampleRate, channels)
		}
		return err
	}

	fmt.Printf("=== 文件处理状态 ===\n")
	fmt.Printf("状态: %v\n", result.Success)
	fmt.Printf("消息: %s\n", result.Message)
	fmt.Println("===============\n")

	return waitForResult(sdk, requestID, taskID, modelType, sampleRate, channels)
}

// 等待并获取处理结果
func waitForResult(sdk *client.Client, requestID, taskID string, modelType string, sampleRate, channels int) error {
	fmt.Println("正在处理音频，请稍候...")

	progress, err := sdk.WaitForComplete(requestID, taskID, modelType, sampleRate, channels, 2, 600, nil)
	if err != nil {
		fmt.Printf("\n处理失败: %s\n", err.Error())
		return err
	}

	fmt.Println("\n=== 音频转写结果 ===")
	fmt.Printf("状态: %v\n", progress.Success)
	fmt.Printf("消息: %s\n", progress.Message)

	if progress.Result != nil {
		fmt.Printf("\n结果:\n%s\n", progress.Result)
	}
	fmt.Println("\n=== 处理完成 ===")
	return nil
}

func main() {
	// SDK配置
	appKey := "b2s8R3Yhy9mOOdaK"                                                               // 替换为您的APP_KEY
	appSecret := "FE40B7C15CEFDB69DF1AFFDD7C933EB4"                                            // 替换为您的APP_SECRET
	audioFile := "C:\\Users\\CN\\Desktop\\temp_audio_bcf0fe73-f347-4bdb-b712-5631e6c46a66.wav" // 替换为您的音频文件路径
	taskID := "1234567890"                                                                     // 自定义任务ID

	// 音频处理配置
	modelType := client.ModelTypeBeat2B // 或者使用 client.ModelTypeHighAccur
	sampleRate := 16000                 // 采样率
	channels := 1                       // 声道数

	// 创建配置
	config := &client.ClientConfig{
		BaseURL:   "https://api.voiceworld.cn", // 替换为实际的API地址
		OSSConfig: &client.OSSConfig{},         // OSS配置将通过GetOSSToken获取
	}

	// 创建 SDK 客户端
	sdk := client.NewClient(appKey, appSecret, config)

	// 处理音频文件
	if err := processAudio(sdk, audioFile, taskID, modelType, sampleRate, channels); err != nil {
		os.Exit(1)
	}
}
