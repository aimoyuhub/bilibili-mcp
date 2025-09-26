package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirenchuang/bilibili-mcp/internal/bilibili/auth"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// findConfigFile 智能查找配置文件
func findConfigFile(defaultPath string) string {
	// 1. 如果指定了绝对路径，直接使用
	if filepath.IsAbs(defaultPath) {
		return defaultPath
	}

	// 2. 先在当前工作目录查找
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	// 3. 在可执行文件所在目录查找
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		configInExecDir := filepath.Join(execDir, defaultPath)
		if _, err := os.Stat(configInExecDir); err == nil {
			return configInExecDir
		}
	}

	// 4. 都找不到，返回原路径（让程序使用默认配置）
	return defaultPath
}

func main() {
	// 解析命令行参数
	var (
		accountName string
		configPath  string
	)
	flag.StringVar(&accountName, "account", "", "账号名称（用于区分多账号）")
	flag.StringVar(&configPath, "config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 智能查找配置文件
	configPath = findConfigFile(configPath)

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

	// 如果没有指定账号名，提示用户输入
	if accountName == "" {
		fmt.Print("请输入账号名称（用于区分多账号，直接回车使用'default'）: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		accountName = strings.TrimSpace(input)

		if accountName == "" {
			accountName = "default"
		}
	}

	fmt.Println()
	fmt.Println("🔐 B站账号登录工具")
	fmt.Println("==================")
	fmt.Printf("账号名称: %s\n", accountName)
	fmt.Println()

	// 创建登录服务
	loginService := auth.NewLoginService()

	// 检查账号是否已经存在
	if isLoggedIn, account, err := loginService.CheckLoginStatus(context.Background(), accountName); err == nil && isLoggedIn && account != nil {
		fmt.Printf("⚠️  账号 '%s' 已存在\n", accountName)
		fmt.Printf("   昵称: %s\n", account.Nickname)
		fmt.Printf("   UID: %s\n", account.UID)
		fmt.Printf("   最后使用: %s\n", account.LastUsed.Format("2006-01-02 15:04:05"))
		fmt.Println()

		fmt.Print("是否要重新登录？(y/N): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "y") {
			fmt.Println("取消登录")
			return
		}
		fmt.Println()
	}

	// 开始登录流程
	fmt.Println("🔄 开始登录流程...")
	fmt.Println("🌐 即将打开B站登录页面，支持多种登录方式")
	fmt.Println("⏰ 登录超时时间: 5分钟")
	fmt.Println()

	// 执行登录
	if err := loginService.Login(context.Background(), accountName); err != nil {
		logger.Errorf("登录失败: %v", err)
		fmt.Printf("❌ 登录失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✅ 账号 '%s' 登录成功！\n", accountName)
	fmt.Println()

	// 显示当前所有账号
	accounts, err := loginService.ListAccounts()
	if err == nil && len(accounts) > 0 {
		fmt.Println("📋 当前已登录的账号:")
		for i, acc := range accounts {
			marker := ""
			if acc.IsDefault {
				marker += " (默认)"
			}
			if !acc.IsActive {
				marker += " (未激活)"
			}
			fmt.Printf("  %d. %s - %s (UID: %s)%s\n",
				i+1, acc.Name, acc.Nickname, acc.UID, marker)
		}
		fmt.Println()
	}

	fmt.Println("🚀 现在可以启动MCP服务了:")
	fmt.Println("   ./bilibili-mcp")
	fmt.Println()
	fmt.Println("📖 或者查看更多账号管理命令:")
	fmt.Println("   ./bilibili-login -account work     # 登录工作账号")
	fmt.Println("   ./bilibili-login -account personal # 登录个人账号")
}
