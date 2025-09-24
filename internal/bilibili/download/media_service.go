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
	logger.Infof("开始下载媒体 - 视频ID: %s, 类型: %s, 清晰度: %d, CID: %d",
		videoID, opts.MediaType, opts.Quality, opts.CID)

	// 获取视频信息
	videoInfo, err := s.apiClient.GetVideoInfo(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取视频信息失败")
	}

	if videoInfo.Code != 0 {
		return nil, errors.Errorf("获取视频信息失败: %s (code: %d)", videoInfo.Message, videoInfo.Code)
	}

	// 如果没有指定CID，使用第一个分P的CID
	cid := opts.CID
	if cid == 0 && len(videoInfo.Data.Pages) > 0 {
		cid = videoInfo.Data.Pages[0].Cid
	}

	if cid == 0 {
		return nil, errors.New("无法获取视频CID")
	}

	// 首先尝试使用GetPlayUrl（兼容性更好）
	playUrlResp, err := s.apiClient.GetPlayUrl(videoID)
	if err != nil {
		return nil, errors.Wrap(err, "获取播放地址失败")
	}

	if playUrlResp.Code != 0 {
		return nil, errors.Errorf("获取播放地址失败: %s (code: %d)", playUrlResp.Message, playUrlResp.Code)
	}

	// 检查是否需要特定清晰度，如果需要，使用GetVideoStream
	var streamData *VideoStreamData
	currentQuality := getQualityFromVideo(playUrlResp.Data.Dash.Video)
	if opts.Quality > 0 && opts.Quality != currentQuality {
		// 使用GetVideoStream获取指定清晰度
		quality := opts.Quality
		fnval := 16 // DASH格式
		if opts.MediaType == MediaTypeMerged {
			fnval = 1 // MP4格式，已合并
		}

		streamResp, err := s.apiClient.GetVideoStream(videoID, cid, quality, fnval, "html5")
		if err != nil {
			// 如果GetVideoStream失败，回退到GetPlayUrl的结果
			logger.Warnf("GetVideoStream失败，回退到GetPlayUrl: %v", err)
			streamData = convertPlayUrlToStreamData(playUrlResp)
		} else if streamResp.Code != 0 {
			logger.Warnf("GetVideoStream返回错误，回退到GetPlayUrl: %s (code: %d)", streamResp.Message, streamResp.Code)
			streamData = convertPlayUrlToStreamData(playUrlResp)
		} else {
			streamData = streamResp.Data
		}
	} else {
		// 使用GetPlayUrl的结果
		streamData = convertPlayUrlToStreamData(playUrlResp)
	}

	// 创建结果对象
	result := &MediaDownloadResult{
		VideoID:     videoID,
		Title:       videoInfo.Data.Title,
		MediaType:   opts.MediaType,
		Quality:     streamData.Quality,
		QualityDesc: getQualityDescription(streamData.Quality),
		Duration:    int(streamData.TimeLength / 1000), // 转换为秒
	}

	// 确保输出目录存在
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return nil, errors.Wrap(err, "创建输出目录失败")
	}

	// 清理文件名
	cleanTitle := sanitizeFilename(videoInfo.Data.Title)

	// 根据媒体类型下载
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

	// 选择最佳音频流
	bestAudio := streamData.DASH.Audio[0]
	for _, audio := range streamData.DASH.Audio {
		if audio.Bandwidth > bestAudio.Bandwidth {
			bestAudio = audio
		}
	}

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
	audioExists := false
	if fileInfo, err := os.Stat(absAudioPath); err == nil {
		result.AudioSize = fileInfo.Size()
		audioExists = true
		logger.Infof("音频文件已存在: %s", absAudioPath)
	} else {
		audioSize, err := s.downloadStream(ctx, bestAudio.BaseURL, absAudioPath, result.VideoID)
		if err != nil {
			return nil, errors.Wrap(err, "下载音频失败")
		}
		result.AudioSize = audioSize
		logger.Infof("音频下载完成: %s (大小: %.2f MB)", absAudioPath, float64(audioSize)/(1024*1024))
	}

	// 下载视频
	videoExists := false
	if fileInfo, err := os.Stat(absVideoPath); err == nil {
		result.VideoSize = fileInfo.Size()
		videoExists = true
		logger.Infof("视频文件已存在: %s", absVideoPath)
	} else {
		videoSize, err := s.downloadStream(ctx, bestVideo.BaseURL, absVideoPath, result.VideoID)
		if err != nil {
			return nil, errors.Wrap(err, "下载视频失败")
		}
		result.VideoSize = videoSize
		logger.Infof("视频下载完成: %s (大小: %.2f MB)", absVideoPath, float64(videoSize)/(1024*1024))
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

	// 获取文件大小用于进度显示
	contentLength := resp.ContentLength
	if contentLength > 0 {
		logger.Infof("开始下载文件，大小: %.2f MB", float64(contentLength)/(1024*1024))
	} else {
		logger.Infof("开始下载文件，大小未知")
	}

	// 复制数据
	written, err := io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempPath)
		return 0, errors.Wrap(err, "下载数据失败")
	}

	logger.Infof("文件下载完成，实际大小: %.2f MB", float64(written)/(1024*1024))

	tempFile.Close()

	// 重命名为最终文件
	if err := os.Rename(tempPath, outputPath); err != nil {
		os.Remove(tempPath)
		return 0, errors.Wrap(err, "重命名文件失败")
	}

	return written, nil
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
