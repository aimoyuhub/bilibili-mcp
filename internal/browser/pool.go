package browser

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/playwright-community/playwright-go"
	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/auth"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// BrowserPool 浏览器池
type BrowserPool struct {
	browsers   []*BrowserInstance
	available  chan *BrowserInstance
	mu         sync.Mutex
	config     *config.Config
	playwright *playwright.Playwright
	closed     bool
}

// BrowserInstance 浏览器实例
type BrowserInstance struct {
	Browser playwright.Browser
	InUse   bool
	Created time.Time
	LastUse time.Time
}

// NewBrowserPool 创建浏览器池
func NewBrowserPool(cfg *config.Config) (*BrowserPool, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, errors.Wrap(err, "启动playwright失败")
	}

	pool := &BrowserPool{
		browsers:   make([]*BrowserInstance, 0, cfg.Browser.PoolSize),
		available:  make(chan *BrowserInstance, cfg.Browser.PoolSize),
		config:     cfg,
		playwright: pw,
	}

	// 初始化浏览器实例
	for i := 0; i < cfg.Browser.PoolSize; i++ {
		instance, err := pool.createBrowserInstance()
		if err != nil {
			pool.Close()
			return nil, errors.Wrapf(err, "创建浏览器实例 %d 失败", i)
		}
		pool.browsers = append(pool.browsers, instance)
		pool.available <- instance
	}

	logger.Infof("浏览器池初始化完成，池大小: %d", cfg.Browser.PoolSize)
	return pool, nil
}

// Get 获取一个可用的浏览器实例
func (p *BrowserPool) Get() (*BrowserInstance, error) {
	if p.closed {
		return nil, errors.New("浏览器池已关闭")
	}

	select {
	case instance := <-p.available:
		p.mu.Lock()
		instance.InUse = true
		instance.LastUse = time.Now()
		p.mu.Unlock()
		return instance, nil
	case <-time.After(30 * time.Second):
		return nil, errors.New("获取浏览器实例超时")
	}
}

// Put 归还浏览器实例到池中
func (p *BrowserPool) Put(instance *BrowserInstance) {
	if p.closed {
		return
	}

	p.mu.Lock()
	instance.InUse = false
	p.mu.Unlock()

	select {
	case p.available <- instance:
		// 成功归还
	default:
		// 池已满，这种情况不应该发生
		logger.Warn("浏览器池已满，无法归还实例")
	}
}

// GetWithAuth 获取带认证的浏览器页面
func (p *BrowserPool) GetWithAuth(accountName string) (playwright.Page, func(), error) {
	instance, err := p.Get()
	if err != nil {
		return nil, nil, err
	}

	// 创建新的浏览器上下文
	context, err := instance.Browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(p.config.Browser.UserAgent),
		Viewport: &playwright.Size{
			Width:  p.config.Browser.Viewport.Width,
			Height: p.config.Browser.Viewport.Height,
		},
	})
	if err != nil {
		p.Put(instance)
		return nil, nil, errors.Wrap(err, "创建浏览器上下文失败")
	}

	// 加载账号cookies
	logger.Infof("GetWithAuth - 请求的账号名: '%s' (空表示默认账号)", accountName)

	loginService := auth.NewLoginService()

	// 如果没有指定账号名，使用默认账号
	if accountName == "" {
		logger.Info("使用默认账号加载cookies")
		// 获取默认账号信息
		accountManager := auth.NewAccountManager()
		defaultAccount, err := accountManager.GetDefaultAccount()
		if err != nil {
			logger.Errorf("获取默认账号失败: %v", err)
			context.Close()
			p.Put(instance)
			return nil, nil, errors.Wrap(err, "获取默认账号失败")
		}
		accountName = defaultAccount.Name
		logger.Infof("找到默认账号: %s", accountName)
	}

	cookies, err := loginService.LoadCookies(accountName)
	if err != nil {
		logger.Errorf("加载账号 '%s' 的cookies失败: %v", accountName, err)
		context.Close()
		p.Put(instance)
		return nil, nil, errors.Wrapf(err, "加载账号 '%s' 的cookies失败", accountName)
	}

	// 检查是否包含bili_jct
	hasBiliJct := false
	for _, cookie := range cookies {
		if cookie.Name == "bili_jct" {
			hasBiliJct = true
			break
		}
	}
	if !hasBiliJct {
		logger.Warn("cookie文件中没有找到bili_jct")
	}

	// 转换cookies类型
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
		context.Close()
		p.Put(instance)
		return nil, nil, errors.Wrap(err, "设置cookies失败")
	}

	// 创建页面
	page, err := context.NewPage()
	if err != nil {
		context.Close()
		p.Put(instance)
		return nil, nil, errors.Wrap(err, "创建页面失败")
	}

	// 返回清理函数
	cleanup := func() {
		page.Close()
		context.Close()
		p.Put(instance)
	}

	return page, cleanup, nil
}

// Close 关闭浏览器池
func (p *BrowserPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.available)

	// 关闭所有浏览器实例
	for _, instance := range p.browsers {
		if instance.Browser != nil {
			instance.Browser.Close()
		}
	}

	// 停止playwright
	if p.playwright != nil {
		p.playwright.Stop()
	}

	logger.Info("浏览器池已关闭")
	return nil
}

// createBrowserInstance 创建浏览器实例
func (p *BrowserPool) createBrowserInstance() (*BrowserInstance, error) {
	browser, err := p.playwright.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(p.config.Browser.Headless),
		Args: []string{
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-dev-shm-usage",
			"--disable-accelerated-2d-canvas",
			"--no-first-run",
			"--no-zygote",
			"--disable-gpu",
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "启动浏览器失败")
	}

	return &BrowserInstance{
		Browser: browser,
		InUse:   false,
		Created: time.Now(),
		LastUse: time.Now(),
	}, nil
}

// Stats 获取浏览器池统计信息
func (p *BrowserPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	inUseCount := 0
	for _, instance := range p.browsers {
		if instance.InUse {
			inUseCount++
		}
	}

	return map[string]interface{}{
		"total":     len(p.browsers),
		"in_use":    inUseCount,
		"available": len(p.browsers) - inUseCount,
		"closed":    p.closed,
	}
}
