package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/api"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// AudioDownloadService 音频下载服务
type AudioDownloadService struct {
	apiClient *api.Client
	outputDir string
}

// NewAudioDownloadService 创建音频下载服务
func NewAudioDownloadService(apiClient *api.Client, outputDir string) *AudioDownloadService {
	return &AudioDownloadService{
		apiClient: apiClient,
		outputDir: outputDir,
	}
}

// DownloadResult 下载结果
type DownloadResult struct {
	VideoID   string `json:"video_id"`   // 视频ID
	Title     string `json:"title"`      // 视频标题
	AudioPath string `json:"audio_path"` // 音频文件路径
	Duration  int    `json:"duration"`   // 音频时长(秒)
	FileSize  int64  `json:"file_size"`  // 文件大小(字节)
	AudioURL  string `json:"audio_url"`  // 原始音频流地址
}

// DownloadAudio 下载视频音频
func (s *AudioDownloadService) DownloadAudio(ctx context.Context, videoID string) (*DownloadResult, error) {
	logger.Infof("开始下载音频 - 视频ID: %s", videoID)

	// 获取视频信息
	videoInfo, err := s.apiClient.GetVideoInfo(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取视频信息失败")
	}

	if videoInfo.Code != 0 {
		return nil, errors.Errorf("获取视频信息失败: %s (code: %d)", videoInfo.Message, videoInfo.Code)
	}

	// 获取播放地址
	playUrl, err := s.apiClient.GetPlayUrl(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取播放地址失败")
	}

	if playUrl.Code != 0 {
		return nil, errors.Errorf("获取播放地址失败: %s (code: %d)", playUrl.Message, playUrl.Code)
	}

	// 检查是否有音频流
	if len(playUrl.Data.Dash.Audio) == 0 {
		return nil, errors.New("该视频没有可用的音频流")
	}

	// 选择最佳音频流（通常选择带宽最高的）
	bestAudio := playUrl.Data.Dash.Audio[0]
	for _, audio := range playUrl.Data.Dash.Audio {
		if audio.Bandwidth > bestAudio.Bandwidth {
			bestAudio = audio
		}
	}

	// 清理文件名
	cleanTitle := sanitizeFilename(videoInfo.Data.Title)

	// 确保输出目录存在
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return nil, errors.Wrap(err, "创建输出目录失败")
	}

	// 生成文件路径
	filename := fmt.Sprintf("%s_%s.m4a", cleanTitle, videoID)
	audioPath := filepath.Join(s.outputDir, filename)

	// 检查文件是否已存在
	if _, err := os.Stat(audioPath); err == nil {
		logger.Infof("音频文件已存在: %s", audioPath)

		// 获取文件大小
		fileInfo, _ := os.Stat(audioPath)

		return &DownloadResult{
			VideoID:   videoID,
			Title:     videoInfo.Data.Title,
			AudioPath: audioPath,
			Duration:  playUrl.Data.Dash.Duration,
			FileSize:  fileInfo.Size(),
			AudioURL:  bestAudio.BaseURL,
		}, nil
	}

	// 下载音频流
	logger.Infof("开始下载音频流: %s", bestAudio.BaseURL)

	fileSize, err := s.downloadAudioStream(ctx, bestAudio.BaseURL, audioPath, videoID)
	if err != nil {
		return nil, errors.Wrap(err, "下载音频流失败")
	}

	logger.Infof("音频下载完成: %s (大小: %.2f MB)", audioPath, float64(fileSize)/(1024*1024))

	return &DownloadResult{
		VideoID:   videoID,
		Title:     videoInfo.Data.Title,
		AudioPath: audioPath,
		Duration:  playUrl.Data.Dash.Duration,
		FileSize:  fileSize,
		AudioURL:  bestAudio.BaseURL,
	}, nil
}

// downloadAudioStream 下载音频流
func (s *AudioDownloadService) downloadAudioStream(ctx context.Context, audioURL, outputPath, videoID string) (int64, error) {
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", audioURL, nil)
	if err != nil {
		return 0, errors.Wrap(err, "创建请求失败")
	}

	// 设置必要的请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://www.bilibili.com/video/%s", videoID))
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")

	// 发送请求
	client := &http.Client{
		Timeout: 10 * time.Minute, // 10分钟超时，足够下载大文件
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "HTTP请求失败")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errors.Errorf("HTTP请求失败: %d %s", resp.StatusCode, resp.Status)
	}

	// 创建临时文件
	tempPath := outputPath + ".downloading"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return 0, errors.Wrap(err, "创建临时文件失败")
	}
	defer tempFile.Close()

	// 获取文件大小用于进度显示
	contentLength := resp.ContentLength
	if contentLength > 0 {
		logger.Infof("开始下载音频文件，大小: %.2f MB", float64(contentLength)/(1024*1024))
	} else {
		logger.Infof("开始下载音频文件，大小未知")
	}

	// 复制数据
	written, err := io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempPath)
		return 0, errors.Wrap(err, "下载数据失败")
	}

	logger.Infof("音频文件下载完成，实际大小: %.2f MB", float64(written)/(1024*1024))

	tempFile.Close()

	// 重命名为最终文件
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return 0, errors.Wrap(err, "重命名文件失败")
	}

	return written, nil
}
