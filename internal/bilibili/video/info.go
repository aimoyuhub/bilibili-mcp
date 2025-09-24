package video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/playwright-community/playwright-go"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// VideoInfo 视频信息结构
type VideoInfo struct {
	BVID        string `json:"bvid"`
	AID         int64  `json:"aid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Duration    int    `json:"duration"`
	View        int64  `json:"view"`
	Like        int64  `json:"like"`
	Coin        int64  `json:"coin"`
	Favorite    int64  `json:"favorite"`
	Share       int64  `json:"share"`
	Reply       int64  `json:"reply"`
	Author      Author `json:"author"`
	PubDate     int64  `json:"pub_date"`
	CoverURL    string `json:"cover_url"`
	Tags        []Tag  `json:"tags"`
}

// Author 作者信息
type Author struct {
	UID      int64  `json:"uid"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Verified bool   `json:"verified"`
}

// Tag 标签信息
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// VideoService 视频服务
type VideoService struct {
	page playwright.Page
}

// NewVideoService 创建视频服务
func NewVideoService(page playwright.Page) *VideoService {
	return &VideoService{page: page}
}

// GetVideoInfo 获取视频信息
func (s *VideoService) GetVideoInfo(ctx context.Context, videoID string) (*VideoInfo, error) {
	// 规范化视频ID
	normalizedID, err := s.normalizeVideoID(videoID)
	if err != nil {
		return nil, err
	}

	// 构建视频URL
	videoURL := fmt.Sprintf("https://www.bilibili.com/video/%s", normalizedID)
	logger.Infof("获取视频信息: %s", videoURL)

	// 导航到视频页面
	if _, err := s.page.Goto(videoURL); err != nil {
		return nil, errors.Wrap(err, "导航到视频页面失败")
	}

	// 等待页面加载
	if _, err := s.page.WaitForSelector("h1[title]", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return nil, errors.Wrap(err, "视频页面加载超时")
	}

	// 提取视频信息
	videoInfo, err := s.extractVideoInfo()
	if err != nil {
		return nil, errors.Wrap(err, "提取视频信息失败")
	}

	logger.Infof("成功获取视频信息: %s", videoInfo.Title)
	return videoInfo, nil
}

// extractVideoInfo 从页面提取视频信息
func (s *VideoService) extractVideoInfo() (*VideoInfo, error) {
	videoInfo := &VideoInfo{}

	// 获取标题
	if title, err := s.page.Locator("h1[title]").GetAttribute("title"); err == nil {
		videoInfo.Title = title
	}

	// 获取描述
	if desc, err := s.page.Locator(".desc-info-text").TextContent(); err == nil {
		videoInfo.Description = strings.TrimSpace(desc)
	}

	// 获取作者信息
	if authorName, err := s.page.Locator(".up-name").TextContent(); err == nil {
		videoInfo.Author.Name = strings.TrimSpace(authorName)
	}

	if authorAvatar, err := s.page.Locator(".up-avatar img").GetAttribute("src"); err == nil {
		videoInfo.Author.Avatar = authorAvatar
	}

	// 获取统计数据
	s.extractStatistics(videoInfo)

	// 从URL获取BVID
	currentURL := s.page.URL()
	if bvid := s.extractBVIDFromURL(currentURL); bvid != "" {
		videoInfo.BVID = bvid
	}

	// 尝试从页面数据获取更多信息
	if err := s.extractFromPageData(videoInfo); err != nil {
		logger.Warnf("从页面数据提取信息失败: %v", err)
	}

	return videoInfo, nil
}

// extractStatistics 提取统计数据
func (s *VideoService) extractStatistics(videoInfo *VideoInfo) {
	// 播放量
	if viewText, err := s.page.Locator(".view-text").TextContent(); err == nil {
		if view := s.parseNumber(viewText); view > 0 {
			videoInfo.View = view
		}
	}

	// 点赞数
	if likeText, err := s.page.Locator(".like-info .info-text").TextContent(); err == nil {
		if like := s.parseNumber(likeText); like > 0 {
			videoInfo.Like = like
		}
	}

	// 投币数
	if coinText, err := s.page.Locator(".coin-info .info-text").TextContent(); err == nil {
		if coin := s.parseNumber(coinText); coin > 0 {
			videoInfo.Coin = coin
		}
	}

	// 收藏数
	if favoriteText, err := s.page.Locator(".collect-info .info-text").TextContent(); err == nil {
		if favorite := s.parseNumber(favoriteText); favorite > 0 {
			videoInfo.Favorite = favorite
		}
	}

	// 分享数
	if shareText, err := s.page.Locator(".share-info .info-text").TextContent(); err == nil {
		if share := s.parseNumber(shareText); share > 0 {
			videoInfo.Share = share
		}
	}
}

// extractFromPageData 从页面初始数据中提取信息
func (s *VideoService) extractFromPageData(videoInfo *VideoInfo) error {
	// 执行JavaScript获取初始数据
	result, err := s.page.Evaluate(`() => {
		if (window.__INITIAL_STATE__) {
			const data = window.__INITIAL_STATE__;
			if (data.videoData) {
				return {
					aid: data.videoData.aid,
					bvid: data.videoData.bvid,
					duration: data.videoData.duration,
					pubdate: data.videoData.pubdate,
					pic: data.videoData.pic,
					owner: data.videoData.owner,
					stat: data.videoData.stat
				};
			}
		}
		return null;
	}`)
	if err != nil {
		return err
	}

	if result == nil {
		return errors.New("未找到页面初始数据")
	}

	// 解析结果
	dataBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(dataBytes, &pageData); err != nil {
		return err
	}

	// 提取AID
	if aid, ok := pageData["aid"].(float64); ok {
		videoInfo.AID = int64(aid)
	}

	// 提取BVID
	if bvid, ok := pageData["bvid"].(string); ok {
		videoInfo.BVID = bvid
	}

	// 提取时长
	if duration, ok := pageData["duration"].(float64); ok {
		videoInfo.Duration = int(duration)
	}

	// 提取发布时间
	if pubdate, ok := pageData["pubdate"].(float64); ok {
		videoInfo.PubDate = int64(pubdate)
	}

	// 提取封面
	if pic, ok := pageData["pic"].(string); ok {
		videoInfo.CoverURL = pic
	}

	// 提取作者信息
	if owner, ok := pageData["owner"].(map[string]interface{}); ok {
		if uid, ok := owner["mid"].(float64); ok {
			videoInfo.Author.UID = int64(uid)
		}
		if name, ok := owner["name"].(string); ok {
			videoInfo.Author.Name = name
		}
		if face, ok := owner["face"].(string); ok {
			videoInfo.Author.Avatar = face
		}
	}

	// 提取统计数据
	if stat, ok := pageData["stat"].(map[string]interface{}); ok {
		if view, ok := stat["view"].(float64); ok {
			videoInfo.View = int64(view)
		}
		if like, ok := stat["like"].(float64); ok {
			videoInfo.Like = int64(like)
		}
		if coin, ok := stat["coin"].(float64); ok {
			videoInfo.Coin = int64(coin)
		}
		if favorite, ok := stat["favorite"].(float64); ok {
			videoInfo.Favorite = int64(favorite)
		}
		if share, ok := stat["share"].(float64); ok {
			videoInfo.Share = int64(share)
		}
		if reply, ok := stat["reply"].(float64); ok {
			videoInfo.Reply = int64(reply)
		}
	}

	return nil
}

// normalizeVideoID 规范化视频ID
func (s *VideoService) normalizeVideoID(videoID string) (string, error) {
	videoID = strings.TrimSpace(videoID)
	
	// 如果是BV号，直接返回
	if strings.HasPrefix(videoID, "BV") {
		return videoID, nil
	}
	
	// 如果是AV号，需要转换为BV号（这里简化处理，实际可能需要调用API）
	if strings.HasPrefix(videoID, "av") {
		// 提取数字部分
		aidStr := strings.TrimPrefix(videoID, "av")
		if _, err := strconv.ParseInt(aidStr, 10, 64); err != nil {
			return "", errors.New("无效的AV号格式")
		}
		// 这里返回原始AV号，实际使用中可能需要转换
		return videoID, nil
	}
	
	return "", errors.New("无效的视频ID格式，应为BV号或AV号")
}

// extractBVIDFromURL 从URL中提取BVID
func (s *VideoService) extractBVIDFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	
	// 匹配BV号模式
	re := regexp.MustCompile(`/video/(BV[0-9A-Za-z]+)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) > 1 {
		return matches[1]
	}
	
	return ""
}

// parseNumber 解析数字字符串（支持万、亿等单位）
func (s *VideoService) parseNumber(text string) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	
	// 移除非数字和单位字符以外的字符
	re := regexp.MustCompile(`[\d.万亿]+`)
	matches := re.FindString(text)
	if matches == "" {
		return 0
	}
	
	// 处理万、亿单位
	if strings.Contains(matches, "万") {
		numStr := strings.Replace(matches, "万", "", 1)
		if num, err := strconv.ParseFloat(numStr, 64); err == nil {
			return int64(num * 10000)
		}
	} else if strings.Contains(matches, "亿") {
		numStr := strings.Replace(matches, "亿", "", 1)
		if num, err := strconv.ParseFloat(numStr, 64); err == nil {
			return int64(num * 100000000)
		}
	} else {
		if num, err := strconv.ParseInt(matches, 10, 64); err == nil {
			return num
		}
	}
	
	return 0
}
