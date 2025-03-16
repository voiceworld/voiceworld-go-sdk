package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

// OSSConfig 存储OSS配置信息
type OSSConfig struct {
	Endpoint        string
	AccessKeyID     string // 使用STS前缀避免混淆
	AccessKeySecret string // 使用STS前缀避免混淆
	SecurityToken   string // STS临时凭证的安全令牌
	BucketName      string
}

// ProgressListener 用于跟踪上传进度
type ProgressListener struct {
	TotalBytes     int64
	UploadedBytes  int64
	LastPercentage int
	FileName       string
}

// 进度回调函数类型
type ProgressCallback func(increment, transferred, total int64)

// UploadFileWithSTS 使用STS临时凭证上传文件到OSS
func UploadFileWithSTS(config OSSConfig, localFilePath, objectKey string) error {
	// 创建OSS配置
	ossConfig := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.AccessKeySecret,
			config.SecurityToken,
		)).WithEndpoint(config.Endpoint).WithRegion("cn-shanghai")

	// 创建OSS客户端
	client := oss.NewClient(ossConfig)

	// 打开本地文件
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 文件名
	fileName := filepath.Base(localFilePath)
	totalBytes := fileInfo.Size()
	var uploadedBytes int64 = 0
	lastPercentage := 0

	// 创建进度回调函数
	progressFn := func(increment, transferred, total int64) {
		uploadedBytes = transferred
		percentage := int(float64(uploadedBytes) * 100 / float64(totalBytes))
		if percentage > lastPercentage && percentage%10 == 0 {
			lastPercentage = percentage
			fmt.Printf("上传进度: %s - %d%%\n", fileName, percentage)
		}
	}

	// 创建上传对象的请求
	putRequest := &oss.PutObjectRequest{
		Bucket: oss.Ptr(config.BucketName),
		Key:    oss.Ptr(objectKey),

		ProgressFn: progressFn,
	}

	// 设置上下文超时
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 上传文件
	_, err = client.PutObjectFromFile(ctx, putRequest, localFilePath)
	if err != nil {
		return fmt.Errorf("上传文件失败: %v", err)
	}

	fmt.Printf("文件上传完成: %s\n", fileName)
	return nil
}

// UploadFileWithSTSAndCallback 使用STS临时凭证上传文件到OSS，并通过回调函数报告进度
func UploadFileWithSTSAndCallback(
	config OSSConfig,
	localFilePath, objectKey string,
	progressCallback func(fileName string, totalBytes, uploadedBytes int64, percentage int),
) error {
	// 创建OSS配置
	ossConfig := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.AccessKeySecret,
			config.SecurityToken,
		)).WithEndpoint(config.Endpoint).WithRegion("cn-shanghai")

	// 创建OSS客户端
	client := oss.NewClient(ossConfig)

	// 打开本地文件
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 自定义进度监听器
	fileName := filepath.Base(localFilePath)
	totalBytes := fileInfo.Size()
	var uploadedBytes int64 = 0
	lastPercentage := 0

	// 创建进度回调函数
	progressFn := func(increment, transferred, total int64) {
		uploadedBytes = transferred
		percentage := int(float64(uploadedBytes) * 100 / float64(totalBytes))
		if percentage > lastPercentage {
			lastPercentage = percentage
			if progressCallback != nil {
				progressCallback(fileName, totalBytes, uploadedBytes, percentage)
			}
		}
	}

	// 创建上传对象的请求
	putRequest := &oss.PutObjectRequest{
		Bucket:     oss.Ptr(config.BucketName),
		Key:        oss.Ptr(objectKey),
		ProgressFn: progressFn,
	}

	// 设置上下文超时
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 上传文件
	_, err = client.PutObjectFromFile(ctx, putRequest, localFilePath)
	if err != nil {
		return fmt.Errorf("上传文件失败: %v", err)
	}

	// 上传完成回调
	if progressCallback != nil {
		progressCallback(fileName, totalBytes, totalBytes, 100)
	}

	return nil
}

// CheckFileExists 检查OSS中是否存在指定的文件
func CheckFileExists(config OSSConfig, objectKey string) (bool, error) {
	// 创建OSS配置
	ossConfig := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.AccessKeySecret,
			config.SecurityToken,
		)).
		WithEndpoint(config.Endpoint).WithRegion("cn-shanghai")

	// 创建OSS客户端
	client := oss.NewClient(ossConfig)

	// 创建检查对象是否存在的请求
	headRequest := &oss.HeadObjectRequest{
		Bucket: oss.Ptr(config.BucketName),
		Key:    oss.Ptr(objectKey),
	}

	// 检查文件是否存在
	ctx := context.Background()
	_, err := client.HeadObject(ctx, headRequest)
	if err != nil {
		// 如果是404错误，表示文件不存在
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("检查文件是否存在失败: %v", err)
	}

	return true, nil
}
