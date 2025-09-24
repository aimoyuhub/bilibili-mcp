package comment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/playwright-community/playwright-go"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// CommentService 评论服务
type CommentService struct {
	page playwright.Page
}

// NewCommentService 创建评论服务
func NewCommentService(page playwright.Page) *CommentService {
	return &CommentService{page: page}
}

// PostComment 发表文字评论
func (s *CommentService) PostComment(ctx context.Context, videoID, content string) error {
	logger.Infof("开始发表评论 - 视频: %s, 内容: %s", videoID, content)

	// 导航到视频页面
	videoURL := fmt.Sprintf("https://www.bilibili.com/video/%s", videoID)
	if _, err := s.page.Goto(videoURL); err != nil {
		return errors.Wrap(err, "导航到视频页面失败")
	}

	// 等待页面加载 - 增加超时时间
	logger.Info("等待视频页面加载...")
	if _, err := s.page.WaitForSelector("h1[title]", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(60000), // 增加到30秒
	}); err != nil {
		logger.Errorf("视频页面加载超时: %v", err)
		return errors.Wrap(err, "视频页面加载超时")
	}
	logger.Info("视频页面加载完成")

	// 滚动到评论区
	logger.Info("滚动到评论区...")
	if err := s.scrollToCommentSection(); err != nil {
		logger.Errorf("滚动到评论区失败: %v", err)
		return errors.Wrap(err, "滚动到评论区失败")
	}

	// 等待评论框出现 - 增加超时时间和更多选择器
	logger.Info("等待评论框出现...")
	commentBoxSelectors := []string{
		"textarea.reply-box-textarea",
		"textarea[placeholder*='评论']",
		".reply-box-send textarea",
		".reply-box-wrap textarea",
		"textarea[data-reply-id='0']",
		"textarea.reply-textarea",
	}

	var commentBoxSelector string
	var found bool

	for _, selector := range commentBoxSelectors {
		if _, err := s.page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(15000), // 每个选择器等待15秒
		}); err == nil {
			commentBoxSelector = selector
			found = true
			logger.Infof("找到评论框: %s", selector)
			break
		}
	}

	if !found {
		logger.Error("未找到评论框，可能需要登录或页面结构已变化")
		return errors.New("评论框未找到，可能需要登录或页面结构已变化")
	}

	// 点击评论框激活
	logger.Info("激活评论框...")
	if err := s.page.Locator(commentBoxSelector).Click(); err != nil {
		logger.Errorf("激活评论框失败: %v", err)
		return errors.Wrap(err, "激活评论框失败")
	}

	// 输入评论内容
	logger.Infof("输入评论内容: %s", content)
	if err := s.page.Locator(commentBoxSelector).Fill(content); err != nil {
		logger.Errorf("输入评论内容失败: %v", err)
		return errors.Wrap(err, "输入评论内容失败")
	}

	// 等待一下确保内容输入完成
	time.Sleep(2 * time.Second)

	// 查找发送按钮 - 增加更多选择器和超时时间
	logger.Info("查找发送按钮...")
	sendButtonSelectors := []string{
		".reply-box-send .reply-send-btn",
		".reply-box-send button[class*='send']",
		".reply-box-warp .reply-send-btn",
		"button:has-text('发布')",
		"button:has-text('发送')",
		".reply-send-btn",
		"button[class*='reply-send']",
		".reply-box-wrap button[type='button']",
	}

	var sendButton playwright.Locator
	var foundSelector string

	for _, selector := range sendButtonSelectors {
		logger.Debugf("尝试选择器: %s", selector)
		if locator := s.page.Locator(selector); locator != nil {
			count, _ := locator.Count()
			if count > 0 {
				sendButton = locator.First()
				foundSelector = selector
				logger.Infof("找到发送按钮: %s", selector)
				break
			}
		}
	}

	if sendButton == nil {
		logger.Error("未找到发送按钮，可能页面结构已变化")
		return errors.New("未找到发送按钮，可能页面结构已变化")
	}

	// 等待按钮可用
	logger.Info("等待发送按钮可用...")
	for i := 0; i < 10; i++ {
		if disabled, _ := sendButton.IsDisabled(); !disabled {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 再次检查按钮是否可用
	if disabled, _ := sendButton.IsDisabled(); disabled {
		logger.Error("发送按钮不可用，可能内容为空或需要登录")
		return errors.New("发送按钮不可用，可能内容为空或需要登录")
	}

	// 点击发送按钮
	logger.Infof("点击发送按钮: %s", foundSelector)
	if err := sendButton.Click(); err != nil {
		logger.Errorf("点击发送按钮失败: %v", err)
		return errors.Wrap(err, "点击发送按钮失败")
	}

	// 等待评论发送完成 - 增加等待时间
	logger.Info("等待评论发送完成...")
	time.Sleep(5 * time.Second)

	// 检查是否有错误提示
	if err := s.checkForErrors(); err != nil {
		return err
	}

	logger.Infof("评论发表成功")
	return nil
}

// PostImageComment 发表图片评论
func (s *CommentService) PostImageComment(ctx context.Context, videoID, content, imagePath string) error {
	logger.Infof("开始发表图片评论 - 视频: %s, 内容: %s, 图片: %s", videoID, content, imagePath)

	// 检查上下文是否已经超时
	select {
	case <-ctx.Done():
		return errors.New("操作被取消或超时")
	default:
	}

	// 检查图片文件是否存在
	logger.Info("步骤1: 检查图片文件...")
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return errors.New("图片文件不存在: " + imagePath)
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(imagePath))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
		return errors.New("不支持的图片格式，仅支持 JPG, PNG, GIF")
	}
	logger.Infof("图片文件检查通过: %s", imagePath)

	// 导航到视频页面
	logger.Info("步骤2: 导航到视频页面...")
	videoURL := fmt.Sprintf("https://www.bilibili.com/video/%s", videoID)
	if _, err := s.page.Goto(videoURL); err != nil {
		return errors.Wrap(err, "导航到视频页面失败")
	}
	logger.Infof("成功导航到视频页面: %s", videoURL)

	// 等待页面加载
	if _, err := s.page.WaitForSelector("h1[title]", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return errors.Wrap(err, "视频页面加载超时")
	}

	// 滚动到评论区
	if err := s.scrollToCommentSection(); err != nil {
		return errors.Wrap(err, "滚动到评论区失败")
	}

	// 等待评论框出现（增加超时时间）
	logger.Info("步骤5: 等待评论框出现...")
	commentBoxSelector := "textarea.reply-box-textarea, textarea[placeholder*='评论']"
	if _, err := s.page.WaitForSelector(commentBoxSelector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(15000), // 15秒
	}); err != nil {
		return errors.Wrap(err, "评论框未找到，可能需要登录")
	}
	logger.Info("找到评论框")

	// 查找图片上传按钮
	logger.Info("步骤6: 查找图片上传按钮...")
	imageUploadSelectors := []string{
		"input[type='file'][accept*='image']",
		".reply-box-send .image-upload input",
		".reply-emoji .image-upload input",
	}

	var uploadInput playwright.Locator
	for i, selector := range imageUploadSelectors {
		logger.Infof("尝试选择器 %d: %s", i+1, selector)
		if locator := s.page.Locator(selector); locator != nil {
			count, _ := locator.Count()
			logger.Infof("选择器 %s 找到 %d 个元素", selector, count)
			if count > 0 {
				uploadInput = locator.First()
				logger.Infof("使用选择器: %s", selector)
				break
			}
		}
	}

	if uploadInput == nil {
		return errors.New("未找到图片上传功能")
	}

	// 上传图片
	logger.Info("步骤7: 开始上传图片...")
	if err := uploadInput.SetInputFiles(imagePath); err != nil {
		return errors.Wrap(err, "上传图片失败")
	}
	logger.Info("图片上传完成")

	// 等待图片上传完成
	time.Sleep(3 * time.Second)

	// 输入文字内容（如果有）
	if content != "" {
		if err := s.page.Locator(commentBoxSelector).Fill(content); err != nil {
			return errors.Wrap(err, "输入评论内容失败")
		}
	}

	// 等待一下确保内容输入完成
	time.Sleep(1 * time.Second)

	// 查找发送按钮并发送
	sendButtonSelectors := []string{
		".reply-box-send .reply-send-btn",
		".reply-box-send button[class*='send']",
		"button:has-text('发布')",
		"button:has-text('发送')",
	}

	var sendButton playwright.Locator
	for _, selector := range sendButtonSelectors {
		if locator := s.page.Locator(selector); locator != nil {
			count, _ := locator.Count()
			if count > 0 {
				sendButton = locator.First()
				break
			}
		}
	}

	if sendButton == nil {
		return errors.New("未找到发送按钮")
	}

	// 点击发送按钮
	if err := sendButton.Click(); err != nil {
		return errors.Wrap(err, "点击发送按钮失败")
	}

	// 等待评论发送完成
	time.Sleep(3 * time.Second)

	// 检查是否有错误提示
	if err := s.checkForErrors(); err != nil {
		return err
	}

	logger.Infof("图片评论发表成功")
	return nil
}

// ReplyComment 回复评论
func (s *CommentService) ReplyComment(ctx context.Context, videoID, parentCommentID, content string) error {
	logger.Infof("开始回复评论 - 视频: %s, 父评论: %s, 内容: %s", videoID, parentCommentID, content)

	// 导航到视频页面
	videoURL := fmt.Sprintf("https://www.bilibili.com/video/%s", videoID)
	if _, err := s.page.Goto(videoURL); err != nil {
		return errors.Wrap(err, "导航到视频页面失败")
	}

	// 等待页面加载
	if _, err := s.page.WaitForSelector("h1[title]", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	}); err != nil {
		return errors.Wrap(err, "视频页面加载超时")
	}

	// 滚动到评论区
	if err := s.scrollToCommentSection(); err != nil {
		return errors.Wrap(err, "滚动到评论区失败")
	}

	// 查找目标评论的回复按钮
	commentSelector := fmt.Sprintf("[data-reply-id='%s'], [data-comment-id='%s']", parentCommentID, parentCommentID)
	replyButtonSelector := fmt.Sprintf("%s .reply-btn, %s button:has-text('回复')", commentSelector, commentSelector)

	// 等待评论出现
	if _, err := s.page.WaitForSelector(commentSelector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(5000),
	}); err != nil {
		return errors.Wrap(err, "未找到目标评论")
	}

	// 点击回复按钮
	if err := s.page.Locator(replyButtonSelector).First().Click(); err != nil {
		return errors.Wrap(err, "点击回复按钮失败")
	}

	// 等待回复框出现
	replyBoxSelector := fmt.Sprintf("%s .reply-box textarea", commentSelector)
	if _, err := s.page.WaitForSelector(replyBoxSelector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(3000),
	}); err != nil {
		return errors.Wrap(err, "回复框未出现")
	}

	// 输入回复内容
	if err := s.page.Locator(replyBoxSelector).Fill(content); err != nil {
		return errors.Wrap(err, "输入回复内容失败")
	}

	// 等待内容输入完成
	time.Sleep(1 * time.Second)

	// 查找回复发送按钮
	replySendSelector := fmt.Sprintf("%s .reply-send-btn", commentSelector)
	if err := s.page.Locator(replySendSelector).Click(); err != nil {
		return errors.Wrap(err, "点击回复发送按钮失败")
	}

	// 等待回复发送完成
	time.Sleep(2 * time.Second)

	logger.Infof("回复评论成功")
	return nil
}

// scrollToCommentSection 滚动到评论区
func (s *CommentService) scrollToCommentSection() error {
	// 尝试多种评论区选择器
	commentSectionSelectors := []string{
		"#comment, .comment-container",
		".reply-box-warp",
		".bb-comment",
		"[id*='comment']",
	}

	for _, selector := range commentSectionSelectors {
		if err := s.page.Locator(selector).ScrollIntoViewIfNeeded(); err == nil {
			time.Sleep(1 * time.Second)
			return nil
		}
	}

	// 如果找不到评论区，尝试滚动到页面底部
	s.page.Evaluate("window.scrollTo(0, document.body.scrollHeight)")
	time.Sleep(2 * time.Second)

	return nil
}

// checkForErrors 检查是否有错误提示
func (s *CommentService) checkForErrors() error {
	errorSelectors := []string{
		".error-tips",
		".toast-error",
		"[class*='error']",
		".message:has-text('失败')",
		".message:has-text('错误')",
	}

	for _, selector := range errorSelectors {
		if locator := s.page.Locator(selector); locator != nil {
			if count, _ := locator.Count(); count > 0 {
				if errorText, _ := locator.First().TextContent(); errorText != "" {
					return errors.New("评论发送失败: " + errorText)
				}
			}
		}
	}

	return nil
}
