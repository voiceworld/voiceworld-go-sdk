package main

import (
	"context"
	"fmt"
	"time"
	"voiceworld-go-sdk/voiceworld"
)

func main() {
	// 音频路径
	audioPath := "C:\\Users\\CN\\Desktop\\temp_audio_bcf0fe73-f347-4bdb-b712-5631e6c46a66.wav"

	// 创建客户端配置
	clientConfig := voiceworld.ClientConfig{
		APIEndpoint: "http://192.168.110.219:16062", // 替换为实际的API端点
		AppKey:      "userAppKey",                   // 替换为您的实际AppKey
		AppSecret:   "userAppSecret",                // 替换为您的实际AppSecret
	}

	// 请确保替换上面的AppKey和AppSecret为您的实际凭据，否则将导致API调用失败

	// 生成用户任务ID（可选）
	userTaskID := ""

	// 创建客户端
	client := voiceworld.NewClient(clientConfig)

	// 创建处理器配置
	processorConfig := voiceworld.DefaultProcessorConfig()
	processorConfig.ModelType = "beat2b"            // 设置模型类型
	processorConfig.ChunkSize = 1024 * 1024 * 100   // 设置切割大小为100MB
	processorConfig.MaxRetries = 30                 // 设置最大重试次数
	processorConfig.RetryInterval = 2 * time.Second // 设置重试间隔
	processorConfig.DeleteTempFile = true           // 设置是否删除临时文件

	// 创建音频处理器
	processor := voiceworld.NewAudioProcessor(client, processorConfig)

	// 创建上下文
	ctx := context.Background()

	// 处理音频
	fmt.Println("开始处理音频文件:", audioPath)
	results, err := processor.ProcessAudio(ctx, audioPath, userTaskID)
	if err != nil {
		fmt.Printf("处理音频失败: %v\n", err)
		return
	}

	// 打印处理结果
	fmt.Println("\n处理结果:")
	for i, result := range results {
		fmt.Printf("%d. %s\n", i+1, result)
	}

	fmt.Println("\n处理完成")
}
