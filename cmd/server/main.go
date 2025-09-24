package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shirenchuang/bilibili-mcp/internal/browser"
	"github.com/shirenchuang/bilibili-mcp/internal/mcp"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

func main() {
	// 解析命令行参数
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	if err := logger.Init(cfg); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info("bilibili-mcp 服务启动中...")
	logger.Infof("配置文件: %s", configPath)

	// 初始化浏览器池
	logger.Info("初始化浏览器池...")
	browserPool, err := browser.NewBrowserPool(cfg)
	if err != nil {
		logger.Errorf("初始化浏览器池失败: %v", err)
		os.Exit(1)
	}
	defer browserPool.Close()

	// 创建MCP服务器
	mcpServer := mcp.NewServer(cfg, browserPool)

	// 创建HTTP服务器
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: mcpServer,

		// 设置超时（增加WriteTimeout以支持长时间操作如图片评论）
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Minute, // 增加到10分钟，支持图片评论等耗时操作
		IdleTimeout:  60 * time.Second,
	}

	// 启动HTTP服务器
	go func() {
		logger.Infof("MCP服务器启动在 http://%s:%s/mcp", cfg.Server.Host, cfg.Server.Port)
		logger.Info("服务器准备就绪，等待MCP客户端连接...")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP服务器启动失败: %v", err)
			os.Exit(1)
		}
	}()

	// 打印使用说明
	printUsageInfo(cfg)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务器...")

	// 优雅关闭HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Errorf("服务器关闭失败: %v", err)
	}

	logger.Info("服务器已关闭")
}

// printUsageInfo 打印使用说明
func printUsageInfo(cfg *config.Config) {
	fmt.Println()
	fmt.Println("🚀 bilibili-mcp 服务已启动！")
	fmt.Println()
	fmt.Printf("📡 MCP服务地址: http://%s:%s/mcp\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Println()
	fmt.Println("📋 使用步骤:")
	fmt.Println("1. 首次使用请先登录B站账号:")
	fmt.Println("   ./bilibili-login")
	fmt.Println("   ./bilibili-login -account work  # 多账号登录")
	fmt.Println()
	fmt.Println("2. 在AI客户端中配置MCP:")
	fmt.Println("   - Cursor: 在项目根目录创建 .cursor/mcp.json")
	fmt.Println("   - Claude Code: claude mcp add --transport http bilibili-mcp http://localhost:18666/mcp")
	fmt.Println("   - VSCode: 使用MCP插件添加HTTP服务器")
	fmt.Println()
	fmt.Println("3. 可用的MCP工具:")
	fmt.Println("   - check_login_status: 检查登录状态")
	fmt.Println("   - list_accounts: 列出所有账号")
	fmt.Println("   - post_comment: 发表评论")
	fmt.Println("   - get_video_info: 获取视频信息")
	fmt.Println("   - like_video: 点赞视频")
	fmt.Println("   - 更多工具请查看文档...")
	fmt.Println()
	fmt.Println("📖 文档: https://github.com/shirenchuang/bilibili-mcp")
	fmt.Println("❓ 问题反馈: https://github.com/shirenchuang/bilibili-mcp/issues")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止服务")
	fmt.Println()
}
