package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/api"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/comment"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/download"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// 频率限制器
var (
	rateLimiter = make(map[string]time.Time)
	rateMutex   sync.RWMutex
)

// checkRateLimit 检查频率限制
func checkRateLimit(operation string, minInterval time.Duration) error {
	rateMutex.Lock()
	defer rateMutex.Unlock()

	now := time.Now()
	if lastTime, exists := rateLimiter[operation]; exists {
		if elapsed := now.Sub(lastTime); elapsed < minInterval {
			return errors.Errorf("请求过于频繁，请等待 %.1f 秒后再试", (minInterval - elapsed).Seconds())
		}
	}

	rateLimiter[operation] = now
	return nil
}

// 认证相关处理器

// handleCheckLoginStatus 检查登录状态
func (s *Server) handleCheckLoginStatus(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountName := s.getAccountName(args)

	isLoggedIn, account, err := s.loginService.CheckLoginStatus(ctx, accountName)
	if err != nil {
		return s.createErrorResult(err)
	}

	if !isLoggedIn {
		if accountName == "" {
			return s.createToolResult("未登录，请先运行登录工具: ./bilibili-login", false)
		} else {
			return s.createToolResult(fmt.Sprintf("账号 '%s' 未登录，请运行: ./bilibili-login -account %s", accountName, accountName), false)
		}
	}

	result := fmt.Sprintf("已登录 - 账号: %s, 昵称: %s, UID: %s",
		account.Name, account.Nickname, account.UID)
	return s.createToolResult(result, false)
}

// handleListAccounts 列出所有账号
func (s *Server) handleListAccounts(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accounts, err := s.loginService.ListAccounts()
	if err != nil {
		return s.createErrorResult(err)
	}

	if len(accounts) == 0 {
		return s.createToolResult("没有已登录的账号，请先运行登录工具: ./bilibili-login", false)
	}

	// 格式化账号列表
	var result strings.Builder
	result.WriteString("已登录的账号列表:\n")

	for i, account := range accounts {
		status := ""
		if account.IsDefault {
			status += " (默认)"
		}
		if !account.IsActive {
			status += " (未激活)"
		}

		result.WriteString(fmt.Sprintf("%d. %s - %s (UID: %s)%s\n",
			i+1, account.Name, account.Nickname, account.UID, status))
	}

	return s.createToolResult(result.String(), false)
}

// handleSwitchAccount 切换账号
func (s *Server) handleSwitchAccount(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountName, ok := args["account_name"].(string)
	if !ok || accountName == "" {
		return s.createToolResult("缺少account_name参数", true)
	}

	if err := s.loginService.SwitchAccount(accountName); err != nil {
		return s.createErrorResult(err)
	}

	return s.createToolResult(fmt.Sprintf("已切换到账号: %s", accountName), false)
}

// 评论相关处理器

// handlePostComment 发表评论 - 使用API优先
func (s *Server) handlePostComment(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return s.createToolResult("缺少content参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	accountName := s.getAccountName(args)

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 创建API评论服务
	apiCommentService, err := comment.NewAPICommentService(page)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "创建API评论服务失败"))
	}

	// 使用API发表评论
	commentID, err := apiCommentService.PostComment(ctx, videoID, content)
	if err != nil {
		return s.createErrorResult(err)
	}

	// 生成评论链接
	commentURL := fmt.Sprintf("https://www.bilibili.com/video/%s#reply%d", videoID, commentID)

	result := fmt.Sprintf("评论发表成功！\n视频: %s\n评论ID: %d\n评论链接: %s", videoID, commentID, commentURL)
	return s.createToolResult(result, false)
}

// handlePostImageComment 发表图片评论
func (s *Server) handlePostImageComment(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return s.createToolResult("缺少content参数", true)
	}

	imagePath, ok := args["image_path"].(string)
	if !ok || imagePath == "" {
		return s.createToolResult("缺少image_path参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	// 提醒用户图片评论较慢
	logger.Warn("图片评论使用浏览器自动化，可能需要30-60秒，请耐心等待...")

	accountName := s.getAccountName(args)

	// 获取带认证的浏览器页面，设置更长的超时时间
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 创建评论服务
	commentService := comment.NewCommentService(page)

	// 发表图片评论（这个操作可能需要较长时间）
	if err := commentService.PostImageComment(ctx, videoID, content, imagePath); err != nil {
		return s.createErrorResult(err)
	}

	result := fmt.Sprintf("图片评论发表成功！\n视频: %s\n注意: 由于使用浏览器自动化，图片评论无法获取评论ID和链接", videoID)
	return s.createToolResult(result, false)
}

// handleReplyComment 回复评论
func (s *Server) handleReplyComment(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	parentCommentID, ok := args["parent_comment_id"].(string)
	if !ok || parentCommentID == "" {
		return s.createToolResult("缺少parent_comment_id参数", true)
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return s.createToolResult("缺少content参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	accountName := s.getAccountName(args)

	// 检查频率限制
	rateLimitKey := fmt.Sprintf("reply_comment_%s_%s", accountName, videoID)
	if err := checkRateLimit(rateLimitKey, 10*time.Second); err != nil {
		return s.createErrorResult(err)
	}

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 获取cookies并创建API客户端
	cookies, err := page.Context().Cookies()
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取cookies失败"))
	}

	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	apiClient := api.NewClient(cookieMap)

	// 使用API回复评论
	replyResp, err := apiClient.ReplyComment(videoID, parentCommentID, content)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "回复评论失败"))
	}

	if replyResp.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", replyResp.Message, replyResp.Code))
	}

	return s.createToolResult(fmt.Sprintf("回复评论成功 - 视频: %s, 回复ID: %s", videoID, replyResp.Data.RPID), false)
}

// 视频相关处理器

// handleGetVideoInfo 获取视频信息 - 使用API优先
func (s *Server) handleGetVideoInfo(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	// 创建API客户端（不需要登录cookies获取基本视频信息）
	apiClient := api.NewClient(map[string]string{})

	// 使用API获取视频信息
	videoInfo, err := apiClient.GetVideoInfo(videoID)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取视频信息失败"))
	}

	if videoInfo.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", videoInfo.Message, videoInfo.Code))
	}

	// 格式化输出
	jsonData, err := json.MarshalIndent(videoInfo.Data, "", "  ")
	if err != nil {
		return s.createErrorResult(err)
	}

	return s.createToolResult(string(jsonData), false)
}

// handleDownloadMedia 下载媒体文件（音频、视频或合并文件）
func (s *Server) handleDownloadMedia(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createErrorResult(errors.New("缺少必需的参数: video_id"))
	}

	// 获取媒体类型，默认为合并文件
	mediaTypeStr := "merged"
	if mt, ok := args["media_type"].(string); ok && mt != "" {
		mediaTypeStr = mt
	}

	var mediaType download.MediaType
	switch mediaTypeStr {
	case "audio":
		mediaType = download.MediaTypeAudio
	case "video":
		mediaType = download.MediaTypeVideo
	case "merged":
		mediaType = download.MediaTypeMerged
	default:
		return s.createErrorResult(errors.Errorf("不支持的媒体类型: %s，支持的类型: audio, video, merged", mediaTypeStr))
	}

	// 获取清晰度，默认为0（自动选择）
	quality := 0
	if q, ok := args["quality"]; ok {
		if qInt, ok := q.(float64); ok {
			quality = int(qInt)
		}
	}

	// 获取CID
	var cid int64
	if cidValue, ok := args["cid"]; ok {
		switch v := cidValue.(type) {
		case float64:
			cid = int64(v)
		case int:
			cid = int64(v)
		case int64:
			cid = v
		case string:
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return s.createToolResult("cid参数格式错误", true)
			}
			cid = parsed
		}
	}

	// 获取输出目录
	outputDir := "./downloads"
	if dir, ok := args["output_dir"].(string); ok && dir != "" {
		outputDir = dir
	}

	accountName := s.getAccountName(args)

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 获取cookies并创建API客户端
	cookies, err := page.Context().Cookies()
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取cookies失败"))
	}

	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	apiClient := api.NewClient(cookieMap)

	// 创建媒体下载服务
	mediaDownloadService := download.NewMediaDownloadService(apiClient, outputDir)

	// 设置下载选项
	opts := download.DownloadOptions{
		MediaType: mediaType,
		Quality:   quality,
		CID:       cid,
	}

	// 下载媒体
	result, err := mediaDownloadService.DownloadMedia(ctx, videoID, opts)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "下载媒体失败"))
	}

	// 格式化输出
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return s.createErrorResult(err)
	}

	// 构建提示信息
	var message strings.Builder
	message.WriteString(fmt.Sprintf("媒体下载完成！类型: %s, 清晰度: %s\n\n", result.MediaType, result.QualityDesc))

	// 添加文件路径信息
	if result.AudioPath != "" {
		message.WriteString(fmt.Sprintf("音频文件: %s\n", result.AudioPath))
	}
	if result.VideoPath != "" {
		message.WriteString(fmt.Sprintf("视频文件: %s\n", result.VideoPath))
	}
	if result.MergedPath != "" {
		message.WriteString(fmt.Sprintf("合并文件: %s\n", result.MergedPath))
	}

	// 添加合并提示
	if result.MergeRequired && result.MergeCommand != "" {
		message.WriteString(fmt.Sprintf("\n⚠️  需要合并音视频文件，请运行以下命令：\n%s\n", result.MergeCommand))
	}

	if result.Notes != "" {
		message.WriteString(fmt.Sprintf("\n📝 %s\n", result.Notes))
	}

	message.WriteString(fmt.Sprintf("\n详细信息：\n%s", string(jsonData)))

	return s.createToolResult(message.String(), false)
}

// handleGetUserVideos 获取用户视频列表
func (s *Server) handleGetUserVideos(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	userID, ok := args["user_id"].(string)
	if !ok || userID == "" {
		return s.createErrorResult(errors.New("缺少必需的参数: user_id"))
	}

	// 检查频率限制 - 每个用户每20秒最多请求一次
	rateLimitKey := fmt.Sprintf("get_user_videos_%s", userID)
	if err := checkRateLimit(rateLimitKey, 20*time.Second); err != nil {
		return s.createErrorResult(err)
	}

	// 获取页码参数
	page := 1
	if p, ok := args["page"].(float64); ok {
		page = int(p)
	}
	if page < 1 {
		page = 1
	}

	// 获取每页数量参数
	pageSize := 20
	if ps, ok := args["page_size"].(float64); ok {
		pageSize = int(ps)
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	logger.Infof("获取用户视频列表 - 用户ID: %s, 页码: %d, 每页数量: %d", userID, page, pageSize)

	// 创建API客户端（获取用户视频列表不需要登录）
	apiClient := api.NewClient(map[string]string{})

	// 获取用户视频列表
	userVideos, err := apiClient.GetUserVideos(userID, page, pageSize)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取用户视频列表失败"))
	}

	if userVideos.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", userVideos.Message, userVideos.Code))
	}

	// 格式化输出
	result := map[string]interface{}{
		"user_id":     userID,
		"page":        userVideos.Data.Page.Pn,
		"page_size":   userVideos.Data.Page.Ps,
		"total_count": userVideos.Data.Page.Count,
		"videos":      userVideos.Data.List.Vlist,
		"categories":  userVideos.Data.List.Tlist,
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return s.createErrorResult(err)
	}

	return s.createToolResult(string(jsonData), false)
}

// handleLikeVideo 点赞视频 - 使用API优先
func (s *Server) handleLikeVideo(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createErrorResult(errors.New("缺少必需的参数: video_id"))
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	// 获取点赞状态，默认为true（点赞）
	like := true
	if likeArg, ok := args["like"].(bool); ok {
		like = likeArg
	}

	accountName := s.getAccountName(args)
	logger.Infof("点赞视频 - 使用账号: '%s' (空表示默认账号)", accountName)

	// 检查频率限制
	rateLimitKey := fmt.Sprintf("like_video_%s_%s", accountName, videoID)
	if err := checkRateLimit(rateLimitKey, 5*time.Second); err != nil {
		return s.createErrorResult(err)
	}

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		logger.Errorf("获取浏览器页面失败: %v", err)
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 获取cookies并创建API客户端 - 从多个域名获取完整cookie
	allCookies := make(map[string]string)

	// 获取所有相关域名的cookies
	domains := []string{
		"https://www.bilibili.com",
		"https://api.bilibili.com",
		"https://passport.bilibili.com",
		"https://space.bilibili.com",
	}

	for _, domain := range domains {
		cookies, err := page.Context().Cookies(domain)
		if err != nil {
			logger.Warnf("获取%s域名cookies失败: %v", domain, err)
			continue
		}

		for _, cookie := range cookies {
			allCookies[cookie.Name] = cookie.Value
		}
	}

	// 如果还是没有bili_jct，尝试获取所有cookies
	if _, exists := allCookies["bili_jct"]; !exists {
		logger.Warn("从指定域名未获取到bili_jct，尝试获取所有cookies")
		allPageCookies, err := page.Context().Cookies()
		if err == nil {
			for _, cookie := range allPageCookies {
				allCookies[cookie.Name] = cookie.Value
			}
		}
	}

	// 调试：检查bili_jct是否存在
	logger.Infof("调试cookie信息: 总数=%d", len(allCookies))
	if biliJct, exists := allCookies["bili_jct"]; exists {
		logger.Infof("bili_jct存在: %s", biliJct[:8]+"...")
	} else {
		logger.Warnf("bili_jct不存在，可用的cookies: %v", func() []string {
			var names []string
			for name := range allCookies {
				names = append(names, name)
			}
			return names
		}())

		// 如果没有bili_jct，返回错误并提示重新登录
		return s.createErrorResult(errors.New("缺少CSRF token (bili_jct)，请重新登录账号"))
	}

	apiClient := api.NewClient(allCookies)

	// 使用API点赞视频
	action := 1
	if !like {
		action = 2 // 取消点赞
	}

	likeResp, err := apiClient.LikeVideo(videoID, action)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "点赞视频失败"))
	}

	if likeResp.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", likeResp.Message, likeResp.Code))
	}

	actionText := "点赞"
	if !like {
		actionText = "取消点赞"
	}

	return s.createToolResult(fmt.Sprintf("%s成功 - 视频: %s", actionText, videoID), false)
}

// handleCoinVideo 投币视频
func (s *Server) handleCoinVideo(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	coinCount := 1
	if count, ok := args["coin_count"].(float64); ok {
		coinCount = int(count)
		if coinCount < 1 || coinCount > 2 {
			coinCount = 1
		}
	}

	// 是否同时点赞
	alsoLike := false
	if like, ok := args["also_like"].(bool); ok {
		alsoLike = like
	}

	accountName := s.getAccountName(args)

	// 检查频率限制
	rateLimitKey := fmt.Sprintf("coin_video_%s_%s", accountName, videoID)
	if err := checkRateLimit(rateLimitKey, 10*time.Second); err != nil {
		return s.createErrorResult(err)
	}

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 获取cookies并创建API客户端
	cookies, err := page.Context().Cookies()
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取cookies失败"))
	}

	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	apiClient := api.NewClient(cookieMap)

	// 使用API投币视频
	coinResp, err := apiClient.CoinVideo(videoID, coinCount, alsoLike)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "投币视频失败"))
	}

	if coinResp.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", coinResp.Message, coinResp.Code))
	}

	resultMsg := fmt.Sprintf("投币成功 - 视频: %s, 数量: %d", videoID, coinCount)
	if alsoLike && coinResp.Data.Like {
		resultMsg += " (同时点赞)"
	}

	return s.createToolResult(resultMsg, false)
}

// handleFavoriteVideo 收藏视频
func (s *Server) handleFavoriteVideo(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	folderID := ""
	if id, ok := args["folder_id"].(string); ok {
		folderID = id
	}

	accountName := s.getAccountName(args)

	// 检查频率限制
	rateLimitKey := fmt.Sprintf("favorite_video_%s_%s", accountName, videoID)
	if err := checkRateLimit(rateLimitKey, 10*time.Second); err != nil {
		return s.createErrorResult(err)
	}

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 获取cookies并创建API客户端
	cookies, err := page.Context().Cookies()
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取cookies失败"))
	}

	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	apiClient := api.NewClient(cookieMap)

	// 使用API收藏视频
	folderIDs := []string{}
	if folderID != "" {
		folderIDs = []string{folderID}
	}

	favResp, err := apiClient.FavoriteVideo(videoID, folderIDs, true)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "收藏视频失败"))
	}

	if favResp.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", favResp.Message, favResp.Code))
	}

	return s.createToolResult(fmt.Sprintf("收藏成功 - 视频: %s", videoID), false)
}

// 用户相关处理器

// handleFollowUser 关注用户
func (s *Server) handleFollowUser(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	userID, ok := args["user_id"].(string)
	if !ok || userID == "" {
		return s.createToolResult("缺少user_id参数", true)
	}

	accountName := s.getAccountName(args)

	// 检查频率限制
	rateLimitKey := fmt.Sprintf("follow_user_%s_%s", accountName, userID)
	if err := checkRateLimit(rateLimitKey, 10*time.Second); err != nil {
		return s.createErrorResult(err)
	}

	// 获取带认证的浏览器页面（仅用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 获取cookies并创建API客户端
	cookies, err := page.Context().Cookies()
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取cookies失败"))
	}

	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	apiClient := api.NewClient(cookieMap)

	// 使用API关注用户 (1:关注 2:取消关注)
	followResp, err := apiClient.FollowUser(userID, 1)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "关注用户失败"))
	}

	if followResp.Code != 0 {
		return s.createErrorResult(errors.Errorf("API返回错误: %s (code: %d)", followResp.Message, followResp.Code))
	}

	return s.createToolResult(fmt.Sprintf("关注成功 - 用户: %s", userID), false)
}

// 可选功能处理器

// handleTranscribeVideo 转录视频
func (s *Server) handleTranscribeVideo(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	if err := s.validateVideoID(videoID); err != nil {
		return s.createErrorResult(err)
	}

	language := "zh"
	if lang, ok := args["language"].(string); ok {
		language = lang
	}

	// 检查Whisper是否启用
	if !s.config.Features.Whisper.Enabled {
		return s.createToolResult("Whisper功能未启用，请在配置文件中启用并安装Whisper", true)
	}

	// TODO: 实现视频转录功能
	logger.Infof("转录视频 - 视频: %s, 语言: %s", videoID, language)

	return s.createToolResult("视频转录功能正在开发中", false)
}

// handleGetVideoStream 获取视频流地址
func (s *Server) handleGetVideoStream(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	videoID, ok := args["video_id"].(string)
	if !ok || videoID == "" {
		return s.createToolResult("缺少video_id参数", true)
	}

	cidValue, ok := args["cid"]
	if !ok {
		return s.createToolResult("缺少cid参数", true)
	}

	var cid int64
	switch v := cidValue.(type) {
	case float64:
		cid = int64(v)
	case int:
		cid = int64(v)
	case int64:
		cid = v
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return s.createToolResult("cid参数格式错误", true)
		}
		cid = parsed
	default:
		return s.createToolResult("cid参数类型错误", true)
	}

	// 验证CID不能为0
	if cid <= 0 {
		return s.createToolResult("CID参数不能为0。请先使用 get_video_info 工具获取视频信息，从返回结果中的 pages 数组获取正确的 CID", true)
	}

	// 可选参数
	quality := 64 // 默认720P
	if q, ok := args["quality"]; ok {
		if qInt, ok := q.(float64); ok {
			quality = int(qInt)
		}
	}

	fnval := 16 // 默认DASH格式
	if f, ok := args["fnval"]; ok {
		if fInt, ok := f.(float64); ok {
			fnval = int(fInt)
		}
	}

	platform := ""
	if p, ok := args["platform"].(string); ok {
		platform = p
	}

	accountName := s.getAccountName(args)

	// 获取带认证的浏览器页面（用于获取cookies）
	page, cleanup, err := s.browserPool.GetWithAuth(accountName)
	if err != nil {
		return s.createErrorResult(err)
	}
	defer cleanup()

	// 从playwright页面获取cookies
	cookies, err := page.Context().Cookies()
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "获取cookies失败"))
	}

	// 转换为map格式
	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	// 创建API客户端
	client := api.NewClient(cookieMap)

	logger.Infof("获取视频流 - 视频ID: %s, CID: %d, 清晰度: %d, 格式: %d, 平台: %s, 账号: %s",
		videoID, cid, quality, fnval, platform, accountName)

	// 调用API获取视频流
	streamResp, err := client.GetVideoStream(videoID, cid, quality, fnval, platform)
	if err != nil {
		return s.createToolResult(fmt.Sprintf("获取视频流失败: %v", err), true)
	}

	// 构建返回结果
	result := map[string]interface{}{
		"video_id":           videoID,
		"cid":                cid,
		"quality":            streamResp.Data.Quality,
		"format":             streamResp.Data.Format,
		"time_length":        streamResp.Data.TimeLength,
		"accept_quality":     streamResp.Data.AcceptQuality,
		"accept_description": streamResp.Data.AcceptDescription,
		"support_formats":    streamResp.Data.SupportFormats,
		"usage_note":         "注意：视频流URL需要正确的Referer和User-Agent才能访问。浏览器直接访问会失败，请使用下载工具如curl/wget，并设置Referer为视频页面URL",
	}

	// 根据格式类型添加相应的流信息
	if streamResp.Data.DASH != nil {
		result["dash"] = map[string]interface{}{
			"duration": streamResp.Data.DASH.Duration,
			"video":    streamResp.Data.DASH.Video,
			"audio":    streamResp.Data.DASH.Audio,
		}
		if streamResp.Data.DASH.Dolby != nil {
			result["dolby"] = streamResp.Data.DASH.Dolby
		}
		if streamResp.Data.DASH.FLAC != nil {
			result["flac"] = streamResp.Data.DASH.FLAC
		}
	}

	if len(streamResp.Data.DURL) > 0 {
		result["durl"] = streamResp.Data.DURL
	}

	// 将结果转换为JSON字符串
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.createErrorResult(errors.Wrap(err, "序列化结果失败"))
	}

	return s.createToolResult(string(resultJSON), false)
}
