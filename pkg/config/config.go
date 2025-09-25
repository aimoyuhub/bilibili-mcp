package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Bilibili BilibiliConfig `mapstructure:"bilibili"`
	Browser  BrowserConfig  `mapstructure:"browser"`
	Features FeaturesConfig `mapstructure:"features"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Accounts AccountsConfig `mapstructure:"accounts"`

	// 运行时解析的路径（不保存到文件）
	resolved *ResolvedPaths
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// BilibiliConfig B站相关配置
type BilibiliConfig struct {
	BaseURL     string `mapstructure:"base_url"`
	APIURL      string `mapstructure:"api_url"`
	PassportURL string `mapstructure:"passport_url"`
}

// BrowserConfig 浏览器配置
type BrowserConfig struct {
	Headless  bool                  `mapstructure:"headless"`
	UserAgent string                `mapstructure:"user_agent"`
	Timeout   time.Duration         `mapstructure:"timeout"`
	PoolSize  int                   `mapstructure:"pool_size"`
	Viewport  BrowserViewportConfig `mapstructure:"viewport"`
}

// BrowserViewportConfig 浏览器视口配置
type BrowserViewportConfig struct {
	Width  int `mapstructure:"width"`
	Height int `mapstructure:"height"`
}

// FeaturesConfig 功能特性配置
type FeaturesConfig struct {
	Whisper WhisperConfig `mapstructure:"whisper"`
}

// WhisperConfig Whisper配置
type WhisperConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	WhisperCppPath string `mapstructure:"whisper_cpp_path"`
	ModelPath      string `mapstructure:"model_path"`
	DefaultModel   string `mapstructure:"default_model"`
	Language       string `mapstructure:"language"`
	CPUThreads     int    `mapstructure:"cpu_threads"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
	EnableGPU      bool   `mapstructure:"enable_gpu"`
	EnableCoreMl   bool   `mapstructure:"enable_core_ml"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// AccountsConfig 账号配置
type AccountsConfig struct {
	CookieDir      string `mapstructure:"cookie_dir"`
	DefaultAccount string `mapstructure:"default_account"`
}

// ResolvedPaths 运行时解析的路径
type ResolvedPaths struct {
	WhisperCppPath string
	ModelPath      string
	LogOutput      string
	CookieDir      string
}

var globalConfig *Config

// Load 加载配置文件，如果文件不存在则使用默认值
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	// 尝试读取配置文件，如果文件不存在则使用默认值
	if err := viper.ReadInConfig(); err != nil {
		// 如果是文件不存在的错误，使用默认配置
		if os.IsNotExist(err) {
			fmt.Printf("⚠️  配置文件 %s 不存在，使用默认配置\n", configPath)
		} else {
			// 其他错误（如格式错误）仍然返回错误
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 解析路径到单独的结构中，不修改原始配置
	resolved, err := createResolvedPaths(&config)
	if err != nil {
		return nil, fmt.Errorf("解析配置路径失败: %w", err)
	}
	config.resolved = resolved

	globalConfig = &config
	return &config, nil
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// setDefaults 设置默认配置值
func setDefaults() {
	viper.SetDefault("server.port", "18666")
	viper.SetDefault("server.host", "localhost")

	viper.SetDefault("bilibili.base_url", "https://www.bilibili.com")
	viper.SetDefault("bilibili.api_url", "https://api.bilibili.com")
	viper.SetDefault("bilibili.passport_url", "https://passport.bilibili.com")

	viper.SetDefault("browser.headless", true)
	viper.SetDefault("browser.user_agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	viper.SetDefault("browser.timeout", "30s")
	viper.SetDefault("browser.pool_size", 2)
	viper.SetDefault("browser.viewport.width", 1920)
	viper.SetDefault("browser.viewport.height", 1080)

	viper.SetDefault("features.whisper.enabled", false)
	viper.SetDefault("features.whisper.whisper_cpp_path", "")
	viper.SetDefault("features.whisper.model_path", "./models/ggml-base.bin")
	viper.SetDefault("features.whisper.default_model", "auto") // auto表示智能选择最佳可用模型
	viper.SetDefault("features.whisper.language", "zh")
	viper.SetDefault("features.whisper.cpu_threads", 4)
	viper.SetDefault("features.whisper.timeout_seconds", 1200)
	viper.SetDefault("features.whisper.enable_gpu", true)
	viper.SetDefault("features.whisper.enable_core_ml", true)

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "./logs/bilibili-mcp.log")

	viper.SetDefault("accounts.cookie_dir", "./cookies")
	viper.SetDefault("accounts.default_account", "")
}

// createResolvedPaths 创建解析后的路径结构，不修改原始配置
func createResolvedPaths(config *Config) (*ResolvedPaths, error) {
	resolved := &ResolvedPaths{}
	var err error

	// 解析 Whisper 相关路径
	if config.Features.Whisper.WhisperCppPath != "" {
		resolved.WhisperCppPath, err = resolvePath(config.Features.Whisper.WhisperCppPath)
		if err != nil {
			return nil, fmt.Errorf("解析whisper_cpp_path失败: %w", err)
		}
	}

	if config.Features.Whisper.ModelPath != "" {
		resolved.ModelPath, err = resolvePath(config.Features.Whisper.ModelPath)
		if err != nil {
			return nil, fmt.Errorf("解析model_path失败: %w", err)
		}
	}

	// 解析日志输出路径
	if config.Logging.Output != "" {
		resolved.LogOutput, err = resolvePath(config.Logging.Output)
		if err != nil {
			return nil, fmt.Errorf("解析log output失败: %w", err)
		}
	}

	// 解析 Cookie 目录
	if config.Accounts.CookieDir != "" {
		resolved.CookieDir, err = resolvePath(config.Accounts.CookieDir)
		if err != nil {
			return nil, fmt.Errorf("解析cookie_dir失败: %w", err)
		}
	}

	return resolved, nil
}

// resolvePath 解析单个路径，支持：
// 1. 环境变量替换 (${VAR} 或 $VAR)
// 2. 用户目录展开 (~)
// 3. 相对路径转绝对路径
// 4. 路径验证
func resolvePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	originalPath := path

	// 1. 环境变量替换
	path = os.ExpandEnv(path)

	// 2. 用户目录展开
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("无法获取用户目录: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	// 3. 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("无法转换为绝对路径 '%s': %w", originalPath, err)
		}
		path = absPath
	}

	// 4. 路径清理
	path = filepath.Clean(path)

	return path, nil
}

// GetResolvedWhisperCppPath 获取解析后的whisper.cpp路径
func (c *Config) GetResolvedWhisperCppPath() string {
	if c.resolved != nil && c.resolved.WhisperCppPath != "" {
		return c.resolved.WhisperCppPath
	}
	return c.Features.Whisper.WhisperCppPath
}

// GetResolvedModelPath 获取解析后的模型路径
func (c *Config) GetResolvedModelPath() string {
	if c.resolved != nil && c.resolved.ModelPath != "" {
		return c.resolved.ModelPath
	}
	return c.Features.Whisper.ModelPath
}

// GetResolvedLogOutput 获取解析后的日志输出路径
func (c *Config) GetResolvedLogOutput() string {
	if c.resolved != nil && c.resolved.LogOutput != "" {
		return c.resolved.LogOutput
	}
	return c.Logging.Output
}

// GetResolvedCookieDir 获取解析后的Cookie目录路径
func (c *Config) GetResolvedCookieDir() string {
	if c.resolved != nil && c.resolved.CookieDir != "" {
		return c.resolved.CookieDir
	}
	return c.Accounts.CookieDir
}
