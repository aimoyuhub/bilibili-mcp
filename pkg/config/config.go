package config

import (
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
	Enabled   bool   `mapstructure:"enabled"`
	ModelPath string `mapstructure:"model_path"`
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

var globalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

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
	viper.SetDefault("features.whisper.model_path", "./models/ggml-base.bin")

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "./logs/bilibili-mcp.log")

	viper.SetDefault("accounts.cookie_dir", "./cookies")
	viper.SetDefault("accounts.default_account", "")
}
