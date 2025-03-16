package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 临时音频目录（使用项目根目录）
const tempAudioDir = "./temp_audio"

// ValidateAudio 验证音频是否符合要求
func ValidateAudio(audioFile string) error {
	// 检验音频是否存在
	if _, err := os.Stat(audioFile); os.IsNotExist(err) {
		return fmt.Errorf("音频文件不存在: %s", audioFile)
	}

	// 检验是否是音频文件，支持更多格式
	supportedFormats := []string{".wav", ".mp3", ".flac", ".aac", ".ogg", ".m4a", ".wma", ".alac", ".ape"}
	isSupported := false
	for _, format := range supportedFormats {
		if strings.HasSuffix(strings.ToLower(audioFile), format) {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("音频文件格式不支持: %s", audioFile)
	}

	return nil
}

// ConvertAudioFile 使用 FFmpeg 将任何音频文件转换为 wav 格式
func ConvertAudioFile(audioFile string) (string, error) {
	// 检查文件是否存在
	if _, err := os.Stat(audioFile); os.IsNotExist(err) {
		return "", fmt.Errorf("音频文件不存在: %s", audioFile)
	}

	// 获取文件扩展名
	// 确保临时目录存在
	if err := os.MkdirAll(tempAudioDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 生成输出文件名为时间戳
	outputFile := filepath.Join(tempAudioDir, fmt.Sprintf("%d.wav", time.Now().UnixNano()/1000000))

	// 使用 FFmpeg 转换音频文件为 WAV 格式
	cmd := exec.Command("ffmpeg",
		"-i", audioFile,
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		"-y",
		outputFile)

	// 执行命令
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("转换音频失败: %w, 输出: %s", err, string(output))
	}

	return outputFile, nil
}

// 将音频文件按文件大小切割
func SplitAudioFile(audioFile string, audioSize int) ([]string, error) {
	// 检查文件是否存在
	if _, err := os.Stat(audioFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("音频文件不存在: %s", audioFile)
	}

	// 打开音频文件
	file, err := os.Open(audioFile)
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	fileSize := fileInfo.Size()

	// 确保临时目录存在
	if err := os.MkdirAll(tempAudioDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 计算需要切割的次数
	numChunks := int(fileSize / int64(audioSize))
	if fileSize%int64(audioSize) != 0 {
		numChunks++
	}

	// 切割音频文件, 并保存到临时目录，文件名为part_0, part_1, part_2, ...
	chunks := make([]string, 0, numChunks)
	buffer := make([]byte, audioSize)

	for i := 0; i < numChunks; i++ {
		offset := int64(i) * int64(audioSize)

		// 计算当前块的实际大小
		currentSize := audioSize
		if offset+int64(audioSize) > fileSize {
			currentSize = int(fileSize - offset)
		}

		// 读取数据
		n, err := file.ReadAt(buffer[:currentSize], offset)
		if err != nil && err.Error() != "EOF" && n <= 0 {
			return chunks, fmt.Errorf("读取音频文件失败: %w", err)
		}

		// 创建输出文件
		chunkPath := filepath.Join(tempAudioDir, fmt.Sprintf("part_%d.wav", i))
		err = os.WriteFile(chunkPath, buffer[:n], 0644)
		if err != nil {
			return chunks, fmt.Errorf("保存音频文件失败: %w", err)
		}

		chunks = append(chunks, chunkPath)
	}

	return chunks, nil
}

// CheckFFmpegInstalled 检查系统是否安装了 FFmpeg
func CheckFFmpegInstalled() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
