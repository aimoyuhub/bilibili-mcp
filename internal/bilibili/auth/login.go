package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/playwright-community/playwright-go"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// LoginService 登录服务
type LoginService struct {
	accountManager *AccountManager
	config         *config.Config
}

// NewLoginService 创建登录服务
func NewLoginService() *LoginService {
	return &LoginService{
		accountManager: NewAccountManager(),
		config:         config.Get(),
	}
}

// Login 登录指定账号（支持多种登录方式）
func (s *LoginService) Login(ctx context.Context, accountName string) error {
	logger.Infof("开始为账号 '%s' 进行登录", accountName)

	// 启动Playwright
	pw, err := playwright.Run()
	if err != nil {
		return errors.Wrap(err, "启动playwright失败")
	}
	defer pw.Stop()

	// 启动浏览器
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false), // 显示浏览器以便扫码
	})
	if err != nil {
		return errors.Wrap(err, "启动浏览器失败")
	}
	defer browser.Close()

	// 创建新页面
	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent: playwright.String(s.config.Browser.UserAgent),
		Viewport: &playwright.Size{
			Width:  s.config.Browser.Viewport.Width,
			Height: s.config.Browser.Viewport.Height,
		},
	})
	if err != nil {
		return errors.Wrap(err, "创建页面失败")
	}

	// 导航到登录页面
	loginURL := s.config.Bilibili.PassportURL + "/login"
	logger.Infof("导航到登录页面: %s", loginURL)

	if _, err := page.Goto(loginURL); err != nil {
		return errors.Wrap(err, "导航到登录页面失败")
	}

	fmt.Printf("🌐 已打开B站登录页面，请选择任意方式登录：\n")
	fmt.Printf("   • 账号密码登录\n")
	fmt.Printf("   • 手机验证码登录\n")
	fmt.Printf("   • 二维码登录\n")
	fmt.Printf("   • 其他登录方式\n")
	fmt.Printf("\n⏰ 登录超时时间: 5分钟\n")
	fmt.Printf("💡 登录成功后会自动保存登录状态\n\n")

	// 等待用户完成登录
	if err := s.waitForLoginCompletion(page, accountName); err != nil {
		return err
	}

	logger.Info("登录成功，正在获取用户信息...")

	// 等待页面完全加载
	time.Sleep(2 * time.Second)

	// 获取用户信息
	userInfo, err := s.getUserInfo(page)
	if err != nil {
		logger.Warnf("获取用户信息失败: %v", err)
		// 即使获取用户信息失败，也继续保存cookies
		userInfo = &UserInfo{
			Username: "未知用户",
			Nickname: "未知昵称",
			UID:      "0",
		}
	}

	// 获取cookies
	cookies, err := page.Context().Cookies()
	if err != nil {
		return errors.Wrap(err, "获取cookies失败")
	}

	if len(cookies) == 0 {
		return errors.New("未获取到有效的cookies")
	}

	// 保存cookies
	if err := s.saveCookies(accountName, cookies); err != nil {
		return errors.Wrap(err, "保存cookies失败")
	}

	// 保存账号信息
	account := &Account{
		Name:      accountName,
		Username:  userInfo.Username,
		Nickname:  userInfo.Nickname,
		UID:       userInfo.UID,
		Avatar:    userInfo.Avatar,
		IsActive:  true,
		LoginTime: time.Now(),
		LastUsed:  time.Now(),
	}

	if err := s.accountManager.SaveAccount(account); err != nil {
		return errors.Wrap(err, "保存账号信息失败")
	}

	logger.Infof("账号 '%s' 登录成功！用户: %s (UID: %s)", accountName, userInfo.Nickname, userInfo.UID)
	return nil
}

// LoadCookies 加载指定账号的cookies
func (s *LoginService) LoadCookies(accountName string) ([]playwright.Cookie, error) {
	cookieFile := s.accountManager.GetCookieFile(accountName)

	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return nil, errors.Wrapf(err, "读取账号 '%s' 的cookies失败", accountName)
	}

	var cookies []playwright.Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, errors.Wrap(err, "解析cookies失败")
	}

	return cookies, nil
}

// CheckLoginStatus 检查指定账号的登录状态
func (s *LoginService) CheckLoginStatus(ctx context.Context, accountName string) (bool, *Account, error) {
	// 如果没有指定账号，使用默认账号
	var account *Account
	var err error

	if accountName == "" {
		account, err = s.accountManager.GetDefaultAccount()
		if err != nil {
			return false, nil, err
		}
		accountName = account.Name
	} else {
		account, err = s.accountManager.GetAccount(accountName)
		if err != nil {
			return false, nil, err
		}
	}

	// 检查cookies文件是否存在
	_, err = s.LoadCookies(accountName)
	if err != nil {
		return false, account, nil
	}

	// 更新最后使用时间
	s.accountManager.UpdateLastUsed(accountName)

	return true, account, nil
}

// ListAccounts 列出所有账号
func (s *LoginService) ListAccounts() ([]Account, error) {
	return s.accountManager.LoadAccounts()
}

// SwitchAccount 切换默认账号
func (s *LoginService) SwitchAccount(accountName string) error {
	return s.accountManager.SetDefaultAccount(accountName)
}

// saveCookies 保存cookies到文件
func (s *LoginService) saveCookies(accountName string, cookies []playwright.Cookie) error {
	cookieFile := s.accountManager.GetCookieFile(accountName)

	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return errors.Wrap(err, "序列化cookies失败")
	}

	return os.WriteFile(cookieFile, data, 0644)
}

// UserInfo 用户信息
type UserInfo struct {
	Username string
	Nickname string
	UID      string
	Avatar   string
}

// isLoggedIn 检查是否已经登录
func (s *LoginService) isLoggedIn(page playwright.Page) bool {
	// 检查当前URL是否已经跳转离开登录页面
	currentURL := page.URL()
	if !strings.Contains(currentURL, "passport.bilibili.com") {
		return true
	}

	// 检查是否有登录成功的元素
	selectors := []string{
		".header-avatar-wrap",
		".bili-avatar",
		".nav-user-info",
		".header-entry-mini",
	}

	for _, selector := range selectors {
		if count, _ := page.Locator(selector).Count(); count > 0 {
			return true
		}
	}

	// 检查页面内容是否包含用户信息
	if content, err := page.Content(); err == nil {
		if strings.Contains(content, "用户中心") || strings.Contains(content, "个人空间") {
			return true
		}
	}

	return false
}

// waitForLoginCompletion 等待用户完成登录（支持任意登录方式）
func (s *LoginService) waitForLoginCompletion(page playwright.Page, accountName string) error {
	logger.Info("等待用户完成登录...")

	// 先检查是否已经登录
	if s.isLoggedIn(page) {
		logger.Info("检测到已经登录")
		return nil
	}

	// 使用轮询方式检测登录状态
	timeout := time.After(5 * time.Minute)    // 5分钟超时
	ticker := time.NewTicker(3 * time.Second) // 每3秒检查一次，避免过于频繁
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("登录超时，请重试")
		case <-ticker.C:
			if s.isLoggedIn(page) {
				logger.Info("检测到登录成功")
				return nil
			}
			// 输出当前状态，让用户知道系统在等待
			logger.Debug("继续等待用户完成登录...")
		}
	}
}

// getUserInfo 使用API获取用户信息
func (s *LoginService) getUserInfo(page playwright.Page) (*UserInfo, error) {
	logger.Info("使用API获取用户信息...")

	// 获取cookies
	cookies, err := page.Context().Cookies()
	if err != nil {
		return nil, errors.Wrap(err, "获取cookies失败")
	}

	// 创建HTTP客户端并设置cookies
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 构建cookie字符串
	var cookieStr strings.Builder
	for i, cookie := range cookies {
		if i > 0 {
			cookieStr.WriteString("; ")
		}
		cookieStr.WriteString(fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}

	// 调用B站导航API获取用户信息
	req, err := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		return nil, errors.Wrap(err, "创建请求失败")
	}

	// 设置请求头
	req.Header.Set("Cookie", cookieStr.String())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "API请求失败")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "读取响应失败")
	}

	// 解析API响应
	var navResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			IsLogin   bool   `json:"isLogin"`
			Uname     string `json:"uname"`
			Mid       int64  `json:"mid"`
			Face      string `json:"face"`
			LevelInfo struct {
				CurrentLevel int `json:"current_level"`
			} `json:"level_info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &navResp); err != nil {
		return nil, errors.Wrap(err, "解析API响应失败")
	}

	if navResp.Code != 0 {
		return nil, errors.Errorf("API返回错误: %s (code: %d)", navResp.Message, navResp.Code)
	}

	if !navResp.Data.IsLogin {
		return nil, errors.New("用户未登录")
	}

	// 构建用户信息
	userInfo := &UserInfo{
		Username: navResp.Data.Uname,
		Nickname: navResp.Data.Uname,
		UID:      fmt.Sprintf("%d", navResp.Data.Mid),
		Avatar:   navResp.Data.Face,
	}

	return userInfo, nil
}

// ValidateCookies 验证cookies是否有效
func (s *LoginService) ValidateCookies(ctx context.Context, accountName string) (bool, error) {
	cookies, err := s.LoadCookies(accountName)
	if err != nil {
		return false, err
	}

	// 启动Playwright进行验证
	pw, err := playwright.Run()
	if err != nil {
		return false, err
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return false, err
	}
	defer browser.Close()

	context, err := browser.NewContext()
	if err != nil {
		return false, err
	}

	// 设置cookies
	optionalCookies := make([]playwright.OptionalCookie, len(cookies))
	for i, cookie := range cookies {
		optionalCookies[i] = playwright.OptionalCookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   playwright.String(cookie.Domain),
			Path:     playwright.String(cookie.Path),
			Expires:  playwright.Float(cookie.Expires),
			HttpOnly: playwright.Bool(cookie.HttpOnly),
			Secure:   playwright.Bool(cookie.Secure),
			SameSite: cookie.SameSite,
		}
	}
	if err := context.AddCookies(optionalCookies); err != nil {
		return false, err
	}

	page, err := context.NewPage()
	if err != nil {
		return false, err
	}

	// 访问需要登录的页面
	if _, err := page.Goto(s.config.Bilibili.BaseURL + "/account/home"); err != nil {
		return false, err
	}

	// 检查是否有用户头像（表示已登录）
	_, err = page.WaitForSelector(".header-avatar-wrap", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(5000),
	})

	return err == nil, nil
}
