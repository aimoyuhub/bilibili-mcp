package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/api"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// VideoStreamData 兼容类型别名
type VideoStreamData = api.VideoStreamData

// MediaType 媒体类型
type MediaType string

const (
	MediaTypeAudio  MediaType = "audio"
	MediaTypeVideo  MediaType = "video"
	MediaTypeMerged MediaType = "merged" // 音视频合并
)

// ProgressTracker 进度跟踪器
type ProgressTracker struct {
	filename   string
	totalSize  int64
	downloaded int64
	startTime  time.Time
	lastUpdate time.Time
	lastLogged int64
}

// NewProgressTracker 创建进度跟踪器
func NewProgressTracker(filename string, totalSize int64) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		filename:   filename,
		totalSize:  totalSize,
		downloaded: 0,
		startTime:  now,
		lastUpdate: now,
		lastLogged: 0,
	}
}

// Update 更新进度并输出日志
func (p *ProgressTracker) Update(downloaded int64) {
	atomic.StoreInt64(&p.downloaded, downloaded)
	now := time.Now()

	// 每2秒或进度变化超过5%时输出一次日志
	progressPercent := float64(downloaded) * 100 / float64(p.totalSize)
	lastProgressPercent := float64(p.lastLogged) * 100 / float64(p.totalSize)

	if now.Sub(p.lastUpdate) >= 2*time.Second || progressPercent-lastProgressPercent >= 5 {
		p.logProgress(downloaded, now)
		p.lastUpdate = now
		p.lastLogged = downloaded
	}
}

// logProgress 输出进度日志
func (p *ProgressTracker) logProgress(downloaded int64, now time.Time) {
	if p.totalSize <= 0 {
		// 未知文件大小
		elapsed := now.Sub(p.startTime)
		speed := float64(downloaded) / elapsed.Seconds()
		logger.Infof("[下载进度] %s: 已下载 %.2f MB, 速度: %.2f MB/s, 用时: %v",
			p.filename,
			float64(downloaded)/(1024*1024),
			speed/(1024*1024),
			elapsed.Round(time.Second))
	} else {
		// 已知文件大小
		progressPercent := float64(downloaded) * 100 / float64(p.totalSize)
		elapsed := now.Sub(p.startTime)
		speed := float64(downloaded) / elapsed.Seconds()

		// 预估剩余时间
		remaining := time.Duration(0)
		if speed > 0 {
			remainingBytes := p.totalSize - downloaded
			remaining = time.Duration(float64(remainingBytes)/speed) * time.Second
		}

		logger.Infof("[下载进度] %s: %.1f%% (%.2f/%.2f MB), 速度: %.2f MB/s, 剩余时间: %v",
			p.filename,
			progressPercent,
			float64(downloaded)/(1024*1024),
			float64(p.totalSize)/(1024*1024),
			speed/(1024*1024),
			remaining.Round(time.Second))
	}
}

// Finish 完成下载时的日志
func (p *ProgressTracker) Finish(downloaded int64) {
	elapsed := time.Since(p.startTime)
	avgSpeed := float64(downloaded) / elapsed.Seconds()

	logger.Infof("[下载完成] %s: %.2f MB, 平均速度: %.2f MB/s, 总用时: %v",
		p.filename,
		float64(downloaded)/(1024*1024),
		avgSpeed/(1024*1024),
		elapsed.Round(time.Second))
}

// ProgressReader 带进度跟踪的Reader
type ProgressReader struct {
	reader  io.Reader
	tracker *ProgressTracker
	total   int64
}

// NewProgressReader 创建带进度跟踪的Reader
func NewProgressReader(reader io.Reader, tracker *ProgressTracker) *ProgressReader {
	return &ProgressReader{
		reader:  reader,
		tracker: tracker,
		total:   0,
	}
}

// Read 实现io.Reader接口，同时跟踪进度
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.total += int64(n)
		pr.tracker.Update(pr.total)
	}
	return n, err
}

// MediaDownloadService 媒体下载服务
type MediaDownloadService struct {
	apiClient *api.Client
	outputDir string
}

// NewMediaDownloadService 创建媒体下载服务
func NewMediaDownloadService(apiClient *api.Client, outputDir string) *MediaDownloadService {
	return &MediaDownloadService{
		apiClient: apiClient,
		outputDir: outputDir,
	}
}

// QualityInfo 清晰度信息
type QualityInfo struct {
	Quality     int    `json:"quality"`     // 清晰度代码
	Description string `json:"description"` // 清晰度描述
	Width       int    `json:"width"`       // 宽度
	Height      int    `json:"height"`      // 高度
	HasAudio    bool   `json:"has_audio"`   // 是否包含音频
	Available   bool   `json:"available"`   // 是否可用
}

// MediaDownloadResult 媒体下载结果
type MediaDownloadResult struct {
	VideoID     string    `json:"video_id"`     // 视频ID
	Title       string    `json:"title"`        // 视频标题
	MediaType   MediaType `json:"media_type"`   // 媒体类型
	Quality     int       `json:"quality"`      // 清晰度
	QualityDesc string    `json:"quality_desc"` // 清晰度描述
	Duration    int       `json:"duration"`     // 时长(秒)

	// 文件路径（全路径）
	AudioPath  string `json:"audio_path,omitempty"`  // 音频文件路径
	VideoPath  string `json:"video_path,omitempty"`  // 视频文件路径
	MergedPath string `json:"merged_path,omitempty"` // 合并后文件路径

	// 文件信息
	AudioSize  int64 `json:"audio_size,omitempty"`  // 音频文件大小
	VideoSize  int64 `json:"video_size,omitempty"`  // 视频文件大小
	MergedSize int64 `json:"merged_size,omitempty"` // 合并文件大小

	// 流信息
	AudioURL string `json:"audio_url,omitempty"` // 音频流地址
	VideoURL string `json:"video_url,omitempty"` // 视频流地址

	// 清晰度信息
	CurrentQuality     QualityInfo   `json:"current_quality"`     // 当前下载的清晰度信息
	AvailableQualities []QualityInfo `json:"available_qualities"` // 所有可用清晰度

	// 提示信息
	MergeRequired bool   `json:"merge_required"`          // 是否需要合并
	MergeCommand  string `json:"merge_command,omitempty"` // 合并命令
	Notes         string `json:"notes,omitempty"`         // 提示信息
}

// DownloadOptions 下载选项
type DownloadOptions struct {
	MediaType MediaType // 媒体类型
	Quality   int       // 清晰度 (0=自动选择最佳)
	CID       int64     // 视频分P的CID
}

// DownloadMedia 下载媒体文件
func (s *MediaDownloadService) DownloadMedia(ctx context.Context, videoID string, opts DownloadOptions) (*MediaDownloadResult, error) {
	logger.Infof("🚀 开始下载媒体 - 视频ID: %s, 类型: %s, 清晰度: %d, CID: %d",
		videoID, opts.MediaType, opts.Quality, opts.CID)

	// 获取视频信息
	logger.Infof("📋 正在获取视频信息...")
	videoInfo, err := s.apiClient.GetVideoInfo(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取视频信息失败")
	}

	if videoInfo.Code != 0 {
		return nil, errors.Errorf("获取视频信息失败: %s (code: %d)", videoInfo.Message, videoInfo.Code)
	}

	logger.Infof("✅ 视频信息获取成功: %s", videoInfo.Data.Title)

	// 如果没有指定CID，使用第一个分P的CID
	cid := opts.CID
	if cid == 0 && len(videoInfo.Data.Pages) > 0 {
		cid = videoInfo.Data.Pages[0].Cid
	}

	if cid == 0 {
		return nil, errors.New("无法获取视频CID")
	}

	logger.Infof("🔗 正在获取播放地址...")

	// 智能选择下载策略
	var streamData *VideoStreamData
	var currentQuality QualityInfo
	var availableQualities []QualityInfo

	if opts.MediaType == MediaTypeMerged {
		// 对于合并类型，优先尝试获取包含音频的完整视频
		streamResult, err := s.getOptimalStream(videoID, cid, opts.Quality)
		if err != nil {
			return nil, errors.Wrap(err, "获取播放地址失败")
		}
		streamData = streamResult.StreamData
		currentQuality = streamResult.CurrentQuality
		availableQualities = streamResult.AvailableQualities
	} else {
		// 对于单独的音频或视频，使用DASH格式
		playUrlResp, err := s.apiClient.GetPlayUrl(videoID)
		if err != nil {
			return nil, errors.Wrap(err, "获取播放地址失败")
		}
		if playUrlResp.Code != 0 {
			return nil, errors.Errorf("获取播放地址失败: %s (code: %d)", playUrlResp.Message, playUrlResp.Code)
		}
		streamData = convertPlayUrlToStreamData(playUrlResp)

		// 为单独的音频或视频创建简单的质量信息
		currentQuality = QualityInfo{
			Quality:     streamData.Quality,
			Description: getQualityDescription(streamData.Quality),
			HasAudio:    false, // DASH格式音视频分离
			Available:   true,
		}

		// 尝试获取可用清晰度信息
		availableQualities, _ = s.getAvailableQualities(videoID, cid)
	}

	logger.Infof("✅ 播放地址获取成功")

	// 创建结果对象
	result := &MediaDownloadResult{
		VideoID:            videoID,
		Title:              videoInfo.Data.Title,
		MediaType:          opts.MediaType,
		Quality:            streamData.Quality,
		QualityDesc:        getQualityDescription(streamData.Quality),
		Duration:           int(streamData.TimeLength / 1000), // 转换为秒
		CurrentQuality:     currentQuality,
		AvailableQualities: availableQualities,
	}

	// 确保输出目录存在
	logger.Infof("📁 准备输出目录: %s", s.outputDir)
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return nil, errors.Wrap(err, "创建输出目录失败")
	}

	// 清理文件名
	cleanTitle := sanitizeFilename(videoInfo.Data.Title)
	logger.Infof("📝 处理文件名: %s -> %s", videoInfo.Data.Title, cleanTitle)

	// 根据媒体类型下载
	logger.Infof("⬇️ 开始下载 %s 类型的媒体文件...", opts.MediaType)
	switch opts.MediaType {
	case MediaTypeAudio:
		return s.downloadAudioOnly(ctx, result, streamData, cleanTitle)
	case MediaTypeVideo:
		return s.downloadVideoOnly(ctx, result, streamData, cleanTitle)
	case MediaTypeMerged:
		return s.downloadMerged(ctx, result, streamData, cleanTitle)
	default:
		return nil, errors.Errorf("不支持的媒体类型: %s", opts.MediaType)
	}
}

// downloadAudioOnly 仅下载音频
func (s *MediaDownloadService) downloadAudioOnly(ctx context.Context, result *MediaDownloadResult, streamData *VideoStreamData, cleanTitle string) (*MediaDownloadResult, error) {
	if streamData.DASH == nil || len(streamData.DASH.Audio) == 0 {
		return nil, errors.New("该视频没有可用的音频流")
	}

	logger.Infof("🎵 选择最佳音频流...")
	// 选择最佳音频流
	bestAudio := streamData.DASH.Audio[0]
	for _, audio := range streamData.DASH.Audio {
		if audio.Bandwidth > bestAudio.Bandwidth {
			bestAudio = audio
		}
	}
	logger.Infof("✅ 已选择音频流: 带宽 %d kbps", bestAudio.Bandwidth/1000)

	// 生成文件路径
	filename := fmt.Sprintf("%s_%s_audio.m4a", cleanTitle, result.VideoID)
	audioPath := filepath.Join(s.outputDir, filename)

	// 转换为绝对路径
	absPath, err := filepath.Abs(audioPath)
	if err != nil {
		return nil, errors.Wrap(err, "获取绝对路径失败")
	}
	result.AudioPath = absPath
	result.AudioURL = bestAudio.BaseURL

	// 检查文件是否已存在
	if fileInfo, err := os.Stat(absPath); err == nil {
		logger.Infof("音频文件已存在: %s", absPath)
		result.AudioSize = fileInfo.Size()
		result.Notes = "文件已存在，跳过下载"
		return result, nil
	}

	// 下载音频
	fileSize, err := s.downloadStream(ctx, bestAudio.BaseURL, absPath, result.VideoID)
	if err != nil {
		return nil, errors.Wrap(err, "下载音频失败")
	}

	result.AudioSize = fileSize
	result.Notes = "音频下载完成"

	logger.Infof("音频下载完成: %s (大小: %.2f MB)", absPath, float64(fileSize)/(1024*1024))

	return result, nil
}

// downloadVideoOnly 仅下载视频
func (s *MediaDownloadService) downloadVideoOnly(ctx context.Context, result *MediaDownloadResult, streamData *VideoStreamData, cleanTitle string) (*MediaDownloadResult, error) {
	if streamData.DASH == nil || len(streamData.DASH.Video) == 0 {
		return nil, errors.New("该视频没有可用的视频流")
	}

	// 选择匹配清晰度的视频流
	var bestVideo *api.DASHStream
	for i, video := range streamData.DASH.Video {
		if video.ID == result.Quality {
			bestVideo = &streamData.DASH.Video[i]
			break
		}
	}

	// 如果没找到匹配的清晰度，选择第一个
	if bestVideo == nil {
		bestVideo = &streamData.DASH.Video[0]
	}

	// 生成文件路径
	filename := fmt.Sprintf("%s_%s_video_%s.m4v", cleanTitle, result.VideoID, result.QualityDesc)
	videoPath := filepath.Join(s.outputDir, filename)

	// 转换为绝对路径
	absPath, err := filepath.Abs(videoPath)
	if err != nil {
		return nil, errors.Wrap(err, "获取绝对路径失败")
	}
	result.VideoPath = absPath
	result.VideoURL = bestVideo.BaseURL

	// 检查文件是否已存在
	if fileInfo, err := os.Stat(absPath); err == nil {
		logger.Infof("视频文件已存在: %s", absPath)
		result.VideoSize = fileInfo.Size()
		result.Notes = "文件已存在，跳过下载"
		return result, nil
	}

	// 下载视频
	fileSize, err := s.downloadStream(ctx, bestVideo.BaseURL, absPath, result.VideoID)
	if err != nil {
		return nil, errors.Wrap(err, "下载视频失败")
	}

	result.VideoSize = fileSize
	result.Notes = "视频下载完成（仅视频轨道，无音频）"

	logger.Infof("视频下载完成: %s (大小: %.2f MB)", absPath, float64(fileSize)/(1024*1024))

	return result, nil
}

// downloadMerged 下载合并的音视频文件
func (s *MediaDownloadService) downloadMerged(ctx context.Context, result *MediaDownloadResult, streamData *VideoStreamData, cleanTitle string) (*MediaDownloadResult, error) {
	// 对于DASH格式，需要分别下载音频和视频然后合并
	if streamData.DASH != nil {
		return s.downloadAndMerge(ctx, result, streamData, cleanTitle)
	}

	// 对于MP4格式，直接下载
	if len(streamData.DURL) > 0 {
		return s.downloadMP4(ctx, result, streamData, cleanTitle)
	}

	return nil, errors.New("没有可用的视频流")
}

// downloadAndMerge 下载DASH格式并提示合并
func (s *MediaDownloadService) downloadAndMerge(ctx context.Context, result *MediaDownloadResult, streamData *VideoStreamData, cleanTitle string) (*MediaDownloadResult, error) {
	if len(streamData.DASH.Audio) == 0 || len(streamData.DASH.Video) == 0 {
		return nil, errors.New("该视频缺少音频或视频流")
	}

	logger.Infof("🎯 选择最佳音视频流...")
	// 选择最佳音频流
	bestAudio := streamData.DASH.Audio[0]
	for _, audio := range streamData.DASH.Audio {
		if audio.Bandwidth > bestAudio.Bandwidth {
			bestAudio = audio
		}
	}

	// 选择匹配清晰度的视频流
	var bestVideo *api.DASHStream
	for i, video := range streamData.DASH.Video {
		if video.ID == result.Quality {
			bestVideo = &streamData.DASH.Video[i]
			break
		}
	}
	if bestVideo == nil {
		bestVideo = &streamData.DASH.Video[0]
	}

	logger.Infof("✅ 已选择流: 音频带宽 %d kbps, 视频 %s (%dx%d)",
		bestAudio.Bandwidth/1000, result.QualityDesc, bestVideo.Width, bestVideo.Height)

	// 生成文件路径
	audioFilename := fmt.Sprintf("%s_%s_audio.m4a", cleanTitle, result.VideoID)
	videoFilename := fmt.Sprintf("%s_%s_video_%s.m4v", cleanTitle, result.VideoID, result.QualityDesc)
	mergedFilename := fmt.Sprintf("%s_%s_%s.mp4", cleanTitle, result.VideoID, result.QualityDesc)

	audioPath := filepath.Join(s.outputDir, audioFilename)
	videoPath := filepath.Join(s.outputDir, videoFilename)
	mergedPath := filepath.Join(s.outputDir, mergedFilename)

	// 转换为绝对路径
	absAudioPath, _ := filepath.Abs(audioPath)
	absVideoPath, _ := filepath.Abs(videoPath)
	absMergedPath, _ := filepath.Abs(mergedPath)

	result.AudioPath = absAudioPath
	result.VideoPath = absVideoPath
	result.MergedPath = absMergedPath
	result.AudioURL = bestAudio.BaseURL
	result.VideoURL = bestVideo.BaseURL
	result.MergeRequired = true

	// 检查合并文件是否已存在
	if fileInfo, err := os.Stat(absMergedPath); err == nil {
		logger.Infof("合并文件已存在: %s", absMergedPath)
		result.MergedSize = fileInfo.Size()
		result.Notes = "合并文件已存在，跳过下载"
		return result, nil
	}

	// 下载音频
	logger.Infof("🎵 开始处理音频文件...")
	audioExists := false
	if fileInfo, err := os.Stat(absAudioPath); err == nil {
		result.AudioSize = fileInfo.Size()
		audioExists = true
		logger.Infof("✅ 音频文件已存在: %s (%.2f MB)", filepath.Base(absAudioPath), float64(fileInfo.Size())/(1024*1024))
	} else {
		audioSize, err := s.downloadStream(ctx, bestAudio.BaseURL, absAudioPath, result.VideoID)
		if err != nil {
			return nil, errors.Wrap(err, "下载音频失败")
		}
		result.AudioSize = audioSize
	}

	// 下载视频
	logger.Infof("🎬 开始处理视频文件...")
	videoExists := false
	if fileInfo, err := os.Stat(absVideoPath); err == nil {
		result.VideoSize = fileInfo.Size()
		videoExists = true
		logger.Infof("✅ 视频文件已存在: %s (%.2f MB)", filepath.Base(absVideoPath), float64(fileInfo.Size())/(1024*1024))
	} else {
		videoSize, err := s.downloadStream(ctx, bestVideo.BaseURL, absVideoPath, result.VideoID)
		if err != nil {
			return nil, errors.Wrap(err, "下载视频失败")
		}
		result.VideoSize = videoSize
	}

	// 生成合并命令
	result.MergeCommand = fmt.Sprintf("ffmpeg -i \"%s\" -i \"%s\" -c copy \"%s\"",
		absVideoPath, absAudioPath, absMergedPath)

	if audioExists && videoExists {
		result.Notes = "音频和视频文件已存在，请使用ffmpeg合并"
	} else if audioExists {
		result.Notes = "音频文件已存在，视频下载完成，请使用ffmpeg合并"
	} else if videoExists {
		result.Notes = "视频文件已存在，音频下载完成，请使用ffmpeg合并"
	} else {
		result.Notes = "音频和视频下载完成，请使用ffmpeg合并"
	}

	return result, nil
}

// downloadMP4 下载MP4格式文件
func (s *MediaDownloadService) downloadMP4(ctx context.Context, result *MediaDownloadResult, streamData *VideoStreamData, cleanTitle string) (*MediaDownloadResult, error) {
	if len(streamData.DURL) == 0 {
		return nil, errors.New("没有可用的MP4流")
	}

	// 生成文件路径
	filename := fmt.Sprintf("%s_%s_%s.mp4", cleanTitle, result.VideoID, result.QualityDesc)
	mergedPath := filepath.Join(s.outputDir, filename)

	// 转换为绝对路径
	absPath, err := filepath.Abs(mergedPath)
	if err != nil {
		return nil, errors.Wrap(err, "获取绝对路径失败")
	}
	result.MergedPath = absPath

	// 检查文件是否已存在
	if fileInfo, err := os.Stat(absPath); err == nil {
		logger.Infof("MP4文件已存在: %s", absPath)
		result.MergedSize = fileInfo.Size()
		result.Notes = "文件已存在，跳过下载"
		return result, nil
	}

	// MP4格式通常只有一个分段，下载第一个
	videoURL := streamData.DURL[0].URL
	result.VideoURL = videoURL

	// 下载文件
	fileSize, err := s.downloadStream(ctx, videoURL, absPath, result.VideoID)
	if err != nil {
		return nil, errors.Wrap(err, "下载MP4文件失败")
	}

	result.MergedSize = fileSize
	result.Notes = "MP4文件下载完成（已包含音频和视频）"

	logger.Infof("MP4下载完成: %s (大小: %.2f MB)", absPath, float64(fileSize)/(1024*1024))

	return result, nil
}

// downloadStream 下载流文件
func (s *MediaDownloadService) downloadStream(ctx context.Context, streamURL, outputPath, videoID string) (int64, error) {
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", streamURL, nil)
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
		Timeout: 30 * time.Minute, // 30分钟超时，足够下载大文件
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

	// 获取文件大小和文件名
	contentLength := resp.ContentLength
	filename := filepath.Base(outputPath)

	// 创建进度跟踪器
	tracker := NewProgressTracker(filename, contentLength)

	if contentLength > 0 {
		logger.Infof("[开始下载] %s: 文件大小 %.2f MB", filename, float64(contentLength)/(1024*1024))
	} else {
		logger.Infof("[开始下载] %s: 文件大小未知", filename)
	}

	// 创建带进度跟踪的Reader
	progressReader := NewProgressReader(resp.Body, tracker)

	// 复制数据，同时跟踪进度
	written, err := io.Copy(tempFile, progressReader)
	if err != nil {
		os.Remove(tempPath)
		return 0, errors.Wrap(err, "下载数据失败")
	}

	// 输出完成日志
	tracker.Finish(written)

	tempFile.Close()

	// 重命名为最终文件
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return 0, errors.Wrap(err, "重命名文件失败")
	}

	return written, nil
}

// StreamResult 流获取结果
type StreamResult struct {
	StreamData         *VideoStreamData
	CurrentQuality     QualityInfo
	AvailableQualities []QualityInfo
}

// getOptimalStream 获取最优的视频流，优先尝试包含音频的完整视频
func (s *MediaDownloadService) getOptimalStream(videoID string, cid int64, preferredQuality int) (*StreamResult, error) {
	logger.Infof("🎯 分析可用清晰度和最优下载策略...")

	// 1. 先获取所有可用的清晰度信息
	availableQualities, err := s.getAvailableQualities(videoID, cid)
	if err != nil {
		logger.Warnf("获取可用清晰度失败，使用默认策略: %v", err)
		availableQualities = []QualityInfo{}
	}

	// 2. 尝试获取包含音频的完整视频（MP4格式）
	logger.Infof("🎯 尝试获取包含音频的完整视频...")

	// 根据用户需求选择尝试的清晰度顺序
	var qualities []int
	if preferredQuality > 0 {
		if preferredQuality >= 80 {
			qualities = []int{preferredQuality, 80, 64, 32, 16}
		} else {
			qualities = []int{preferredQuality, 64, 32, 16}
		}
	} else {
		qualities = []int{64, 32, 16} // 默认优先尝试标清完整视频
	}

	// 尝试MP4格式
	for _, quality := range qualities {
		streamResp, err := s.apiClient.GetVideoStream(videoID, cid, quality, 1, "html5")
		if err != nil {
			continue
		}
		if streamResp.Code != 0 {
			continue
		}
		if len(streamResp.Data.DURL) > 0 {
			logger.Infof("✅ 找到包含音频的完整视频: %s", getQualityDescription(quality))

			// 构建当前清晰度信息
			currentQuality := QualityInfo{
				Quality:     quality,
				Description: getQualityDescription(quality),
				HasAudio:    true,
				Available:   true,
			}

			return &StreamResult{
				StreamData:         streamResp.Data,
				CurrentQuality:     currentQuality,
				AvailableQualities: availableQualities,
			}, nil
		}
	}

	// 3. 如果没有找到MP4格式，使用DASH格式（音视频分离）
	logger.Infof("⚠️  未找到包含音频的完整视频，使用音视频分离格式")

	targetQuality := preferredQuality
	if targetQuality == 0 {
		targetQuality = 80 // 默认1080P
	}

	streamResp, err := s.apiClient.GetVideoStream(videoID, cid, targetQuality, 16, "html5")
	if err != nil {
		// 回退到GetPlayUrl
		return s.fallbackToPlayUrl(videoID, availableQualities)
	}

	if streamResp.Code != 0 {
		// 回退到GetPlayUrl
		return s.fallbackToPlayUrl(videoID, availableQualities)
	}

	// 从DASH数据中获取实际清晰度信息
	actualQuality := targetQuality
	width, height := 0, 0
	if streamResp.Data.DASH != nil && len(streamResp.Data.DASH.Video) > 0 {
		video := streamResp.Data.DASH.Video[0]
		actualQuality = video.ID
		width = video.Width
		height = video.Height
	}

	currentQuality := QualityInfo{
		Quality:     actualQuality,
		Description: getQualityDescription(actualQuality),
		Width:       width,
		Height:      height,
		HasAudio:    false, // DASH格式音视频分离
		Available:   true,
	}

	return &StreamResult{
		StreamData:         streamResp.Data,
		CurrentQuality:     currentQuality,
		AvailableQualities: availableQualities,
	}, nil
}

// fallbackToPlayUrl 回退到GetPlayUrl
func (s *MediaDownloadService) fallbackToPlayUrl(videoID string, availableQualities []QualityInfo) (*StreamResult, error) {
	logger.Warnf("回退到GetPlayUrl")
	playUrlResp, err := s.apiClient.GetPlayUrl(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取播放地址失败")
	}
	if playUrlResp.Code != 0 {
		return nil, errors.Errorf("获取播放地址失败: %s (code: %d)", playUrlResp.Message, playUrlResp.Code)
	}

	streamData := convertPlayUrlToStreamData(playUrlResp)
	currentQuality := QualityInfo{
		Quality:     streamData.Quality,
		Description: getQualityDescription(streamData.Quality),
		HasAudio:    false, // PlayUrl通常返回DASH格式
		Available:   true,
	}

	return &StreamResult{
		StreamData:         streamData,
		CurrentQuality:     currentQuality,
		AvailableQualities: availableQualities,
	}, nil
}

// getAvailableQualities 获取所有可用的清晰度信息（简化版，避免过多请求）
func (s *MediaDownloadService) getAvailableQualities(videoID string, cid int64) ([]QualityInfo, error) {
	var qualities []QualityInfo

	// 首先尝试获取DASH格式信息，这通常包含所有可用清晰度
	dashResp, err := s.apiClient.GetVideoStream(videoID, cid, 80, 16, "html5")
	if err == nil && dashResp.Code == 0 && dashResp.Data.DASH != nil {
		// 从DASH响应中提取可用的清晰度
		videoStreams := dashResp.Data.DASH.Video
		qualityMap := make(map[int]QualityInfo)

		for _, video := range videoStreams {
			qualityMap[video.ID] = QualityInfo{
				Quality:     video.ID,
				Description: getQualityDescription(video.ID),
				Width:       video.Width,
				Height:      video.Height,
				HasAudio:    false, // DASH格式音视频分离
				Available:   true,
			}
		}

		// 测试几个常见清晰度的MP4格式（包含音频）
		testMP4Qualities := []int{64, 32, 16} // 只测试标清，因为高清很少有MP4
		for _, quality := range testMP4Qualities {
			mp4Resp, err := s.apiClient.GetVideoStream(videoID, cid, quality, 1, "html5")
			if err == nil && mp4Resp.Code == 0 && len(mp4Resp.Data.DURL) > 0 {
				// 更新或添加MP4格式信息
				if existing, exists := qualityMap[quality]; exists {
					existing.HasAudio = true
					qualityMap[quality] = existing
				} else {
					qualityMap[quality] = QualityInfo{
						Quality:     quality,
						Description: getQualityDescription(quality),
						HasAudio:    true,
						Available:   true,
					}
				}
			}
		}

		// 转换为切片并排序
		for _, quality := range []int{127, 125, 120, 116, 112, 80, 74, 64, 32, 16} {
			if info, exists := qualityMap[quality]; exists {
				qualities = append(qualities, info)
			}
		}
	} else {
		// 如果无法获取DASH信息，返回基本的清晰度列表
		logger.Warnf("无法获取详细清晰度信息，使用基本列表")
		basicQualities := []int{80, 64, 32, 16}
		for _, quality := range basicQualities {
			qualities = append(qualities, QualityInfo{
				Quality:     quality,
				Description: getQualityDescription(quality),
				HasAudio:    quality <= 64, // 假设标清有完整视频
				Available:   true,
			})
		}
	}

	return qualities, nil
}

// getQualityDescription 获取清晰度描述
func getQualityDescription(quality int) string {
	qualityMap := map[int]string{
		16:  "360P",
		32:  "480P",
		64:  "720P",
		74:  "720P60",
		80:  "1080P",
		112: "1080P+",
		116: "1080P60",
		120: "4K",
		125: "HDR",
		126: "杜比视界",
		127: "8K",
	}

	if desc, exists := qualityMap[quality]; exists {
		return desc
	}
	return fmt.Sprintf("Q%d", quality)
}

// sanitizeFilename 清理文件名，移除不安全字符
func sanitizeFilename(filename string) string {
	// 移除或替换不安全的字符
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	filename = reg.ReplaceAllString(filename, "_")

	// 移除控制字符
	reg = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	filename = reg.ReplaceAllString(filename, "")

	// 移除首尾空格和点
	filename = strings.TrimSpace(filename)
	filename = strings.Trim(filename, ".")

	// 限制长度
	if len(filename) > 100 {
		filename = filename[:100]
	}

	// 如果文件名为空，使用默认名称
	if filename == "" {
		filename = "bilibili_media"
	}

	return filename
}

// convertPlayUrlToStreamData 将PlayUrlResponse转换为VideoStreamData
func convertPlayUrlToStreamData(playUrlResp *api.PlayUrlResponse) *VideoStreamData {
	// 创建DASH信息
	dash := &api.DASHInfo{
		Duration: playUrlResp.Data.Dash.Duration,
		Video:    make([]api.DASHStream, len(playUrlResp.Data.Dash.Video)),
		Audio:    make([]api.DASHStream, len(playUrlResp.Data.Dash.Audio)),
	}

	// 转换视频流
	for i, video := range playUrlResp.Data.Dash.Video {
		dash.Video[i] = api.DASHStream{
			ID:        video.ID,
			BaseURL:   video.BaseURL,
			Bandwidth: int64(video.Bandwidth),
			MimeType:  video.MimeType,
			Codecs:    video.Codecs,
			Width:     video.Width,
			Height:    video.Height,
		}
	}

	// 转换音频流
	for i, audio := range playUrlResp.Data.Dash.Audio {
		dash.Audio[i] = api.DASHStream{
			ID:        audio.ID,
			BaseURL:   audio.BaseURL,
			Bandwidth: int64(audio.Bandwidth),
			MimeType:  audio.MimeType,
			Codecs:    audio.Codecs,
		}
	}

	return &VideoStreamData{
		Quality:    getQualityFromVideo(playUrlResp.Data.Dash.Video),
		TimeLength: int64(playUrlResp.Data.Dash.Duration * 1000), // 转换为毫秒
		DASH:       dash,
	}
}

// getQualityFromVideo 从视频流中推断清晰度
func getQualityFromVideo(videos []struct {
	ID        int    `json:"id"`
	BaseURL   string `json:"baseUrl"`
	Bandwidth int    `json:"bandwidth"`
	MimeType  string `json:"mimeType"`
	Codecs    string `json:"codecs"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}) int {
	if len(videos) == 0 {
		return 64 // 默认720P
	}

	// 根据分辨率推断清晰度
	maxHeight := 0
	for _, video := range videos {
		if video.Height > maxHeight {
			maxHeight = video.Height
		}
	}

	// 根据高度映射到清晰度代码
	switch {
	case maxHeight >= 2160:
		return 120 // 4K
	case maxHeight >= 1080:
		return 80 // 1080P
	case maxHeight >= 720:
		return 64 // 720P
	case maxHeight >= 480:
		return 32 // 480P
	default:
		return 16 // 360P
	}
}
