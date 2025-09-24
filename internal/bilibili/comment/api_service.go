package comment

import (
	"context"

	"github.com/pkg/errors"
	"github.com/playwright-community/playwright-go"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/api"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// APICommentService 基于API的评论服务
type APICommentService struct {
	apiClient *api.Client
}

// NewAPICommentService 创建API评论服务
func NewAPICommentService(page playwright.Page) (*APICommentService, error) {
	// 从playwright页面获取cookies
	cookies, err := page.Context().Cookies()
	if err != nil {
		return nil, errors.Wrap(err, "获取cookies失败")
	}

	// 转换为map格式
	cookieMap := make(map[string]string)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	// 创建API客户端
	apiClient := api.NewClient(cookieMap)

	// 验证登录状态
	navInfo, err := apiClient.GetNavInfo()
	if err != nil {
		return nil, errors.Wrap(err, "验证登录状态失败")
	}

	if navInfo.Code != 0 || !navInfo.Data.IsLogin {
		return nil, errors.New("用户未登录，无法使用评论功能")
	}

	logger.Infof("API评论服务初始化成功 - 用户: %s (UID: %d)", navInfo.Data.Uname, navInfo.Data.Mid)

	return &APICommentService{
		apiClient: apiClient,
	}, nil
}

// PostComment 发表评论 - 使用API
func (s *APICommentService) PostComment(ctx context.Context, videoID, content string) (int64, error) {
	logger.Infof("使用API发表评论 - 视频: %s, 内容: %s", videoID, content)

	// 调用API发表评论
	resp, err := s.apiClient.PostComment(videoID, content)
	if err != nil {
		return 0, errors.Wrap(err, "API调用失败")
	}

	if resp.Code != 0 {
		return 0, errors.Errorf("评论发表失败: %s (code: %d)", resp.Message, resp.Code)
	}

	logger.Infof("评论发表成功 - 视频: %s, 评论ID: %d", videoID, resp.Data.Rpid)
	return resp.Data.Rpid, nil
}

// PostImageComment 发表图片评论 - 暂时不支持，需要复杂的图片上传API
func (s *APICommentService) PostImageComment(ctx context.Context, videoID, content, imagePath string) error {
	return errors.New("图片评论暂不支持，需要实现图片上传API")
}

// ReplyComment 回复评论 - 需要实现
func (s *APICommentService) ReplyComment(ctx context.Context, videoID, parentCommentID, content string) error {
	return errors.New("回复评论功能待实现")
}
