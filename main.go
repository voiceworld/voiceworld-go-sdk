package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"voiceworld-go-sdk/voiceworld"
	"voiceworld-go-sdk/voiceworld/utils"
)

func main() {
	// 音频路径
	audioPath := "C:\\Users\\CN\\Desktop\\temp_audio_bcf0fe73-f347-4bdb-b712-5631e6c46a66.wav"

	// 用户任务id
	userTaskID := "user_task_123"
	// 检查 FFmpeg 是否安装
	if !utils.CheckFFmpegInstalled() {
		fmt.Println("FFmpeg 未安装，请先安装 FFmpeg")
		return
	}

	// 检查音频文件是否存在
	if err := utils.ValidateAudio(audioPath); err != nil {
		fmt.Println("音频文件不存在或格式不支持:", err)
		return
	}

	// 转换音频文件
	convertedFile, err := utils.ConvertAudioFile(audioPath)
	if err != nil {
		fmt.Println("转换失败:", err)
		return
	}
	// 打印文件目录
	fmt.Println("转换后的文件:", convertedFile)

	// 切割音频文件按100mb切割
	chunks, err := utils.SplitAudioFile(convertedFile, 1024*1024*100)
	if err != nil {
		fmt.Println("切割失败:", err)
		return
	}
	fmt.Println("切割成功，生成的文件块:", chunks)

	// 删除原缓存文件
	os.Remove(convertedFile)
	// 创建客户端配置
	clientConfig := voiceworld.ClientConfig{
		APIEndpoint: "http://192.168.110.219:16062",     // 替换为实际的API端点
		AppKey:      "al8cZEHx4LjwN9Qy",                 // 替换为您的AppKey
		AppSecret:   "E31B1DA0F48665B83BBF36C594926196", // 替换为您的AppSecret
	}

	// 创建客户端
	client := voiceworld.NewClient(clientConfig)

	// 创建上下文
	ctx := context.Background()

	// 获取STS临时凭证
	stsResp, err := client.GetSTSCredentials(ctx, "")
	if err != nil {
		fmt.Println("获取STS临时凭证失败:", err)
		return
	}
	fmt.Println("获取STS临时凭证成功")
	fmt.Printf("- 请求ID: %s\n", stsResp.Data.RequestID)
	fmt.Printf("- 存储桶: %s\n", stsResp.Data.Bucket)
	fmt.Printf("- 上传目录: %s\n", stsResp.Data.UploadDir)
	fmt.Printf("- 凭证过期时间: %s\n", stsResp.Data.Credentials.Expiration)

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
	uploadedFiles := make([]string, 0, len(chunks))
	for i, chunkFile := range chunks {
		fmt.Printf("开始上传文件: %s\n", chunkFile)

		// 获取文件名
		fileName := filepath.Base(chunkFile)

		// 生成对象键（OSS中的文件路径）
		// 格式: audio/app_key/20240321123045/audio_part1.wav
		objectKey := fmt.Sprintf("audio/%s/%s/audio_part%d%s",
			clientConfig.AppKey,
			timestamp,
			i+1,
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
		err = utils.UploadFileWithSTSAndCallback(ossConfig, chunkFile, objectKey, progressCallback)
		if err != nil {
			fmt.Printf("上传文件 %s 失败: %v\n", chunkFile, err)
			continue
		}

		// 构建文件URL
		fileURL := fmt.Sprintf("https://%s.%s/%s", ossConfig.BucketName, ossConfig.Endpoint, objectKey)
		uploadedFiles = append(uploadedFiles, fileURL)

		// 文件上传完成，删除对应临时文件
		err := os.Remove(chunkFile)
		if err != nil {
			return
		}

		// 请求处理音频
		runResp, err := client.Run(ctx, stsResp.Data.RequestID, userTaskID, objectKey, "beat2b")
		if err != nil {
			fmt.Println("请求处理音频失败:", err)
			return
		}
		fmt.Println("请求处理音频成功")

		// 循环获取结果，如果Code为201则打印任务执行中，继续获取结果。如果Code为200则打印结果，并退出循环
		for {
			resultResp, err := client.GetResult(ctx, stsResp.Data.RequestID, userTaskID, runResp.Data.TaskID)
			if err != nil {
				fmt.Println("获取结果失败:", err)
				return
			}
			if resultResp.Code == 201 {
				fmt.Println("结果未准备好，等待重试...")
				time.Sleep(2 * time.Second) // 等待2秒后重试
			}
			if resultResp.Code == 200 {
				fmt.Println("获取结果成功")
				fmt.Printf("- 结果: %s\n", resultResp)
				break
			}
		}

	}
}
