# bilibili-mcp

[![CI](https://github.com/shirenchuang/bilibili-mcp/workflows/CI/badge.svg)](https://github.com/shirenchuang/bilibili-mcp/actions)
[![Release](https://github.com/shirenchuang/bilibili-mcp/workflows/Release/badge.svg)](https://github.com/shirenchuang/bilibili-mcp/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

🎬 **B站自动化操作的标准化MCP接口** - 让AI助手能够直接操作哔哩哔哩，支持评论、点赞、收藏、关注等功能，还集成了Whisper音频转录！

## ✨ 功能特性

- 🔐 **多账号管理**: 支持多个B站账号切换和管理
- 💬 **智能评论**: 文字评论支持，AI可直接发表评论
- 📹 **视频操作**: 点赞、投币、收藏、获取详细信息
- 👥 **用户互动**: 关注用户、获取用户信息和视频列表  
- 🎵 **音频转录**: 集成 Whisper.cpp，本地音频转文字，支持Core ML加速
- 🌐 **标准化接口**: 遵循MCP协议，支持Cursor、Claude等AI客户端
- ⚡ **高性能**: 浏览器池管理，支持并发操作
- 🛡️ **反检测**: 模拟真实用户行为，稳定可靠

## 🚀 快速开始

### 1. 下载安装

#### 方式一：下载预编译版本（推荐）

前往 [Releases 页面](https://github.com/shirenchuang/bilibili-mcp/releases) 下载对应平台的版本：

- **macOS 用户**: 下载 `bilibili-mcp-vX.X.X-darwin-arm64.tar.gz` (Apple Silicon) 或 `darwin-amd64.tar.gz` (Intel)
- **Windows 用户**: 下载 `bilibili-mcp-vX.X.X-windows-amd64.zip`  
- **Linux 用户**: 下载 `bilibili-mcp-vX.X.X-linux-amd64.tar.gz`

解压后即可使用，macOS版本包含Core ML加速模型，转录速度提升2-3倍。

#### 方式二：从源码编译

```bash
git clone https://github.com/shirenchuang/bilibili-mcp.git
cd bilibili-mcp
make build
```

### 2. 登录B站账号

```bash
# 登录默认账号
./bilibili-login

# 登录指定账号
./bilibili-login -account work
./bilibili-login -account personal
```

### 3. 启动MCP服务

```bash
./bilibili-mcp
```

服务将运行在 `http://localhost:18666/mcp`

### 4. 在AI客户端中配置

#### Cursor
在项目根目录创建 `.cursor/mcp.json`：
```json
{
  "mcpServers": {
    "bilibili-mcp": {
      "url": "http://localhost:18666/mcp",
      "description": "B站内容操作服务"
    }
  }
}
```

#### Claude Code CLI
```bash
claude mcp add --transport http bilibili-mcp http://localhost:18666/mcp
```

#### VSCode
参考 `examples/vscode/mcp.json` 配置文件

## 🎵 音频转录功能（可选）

如果需要使用Whisper音频转录功能：

### 自动设置（推荐）

```bash
# 下载模型文件（可选，加速初始化）
./scripts/download-whisper-models.sh

# 运行初始化工具
./whisper-init
```

初始化工具会自动：
- 🖥️ 智能检测系统和GPU类型
- ⚡ 配置最优加速方式（Core ML/Metal/CUDA/CPU）
- 📦 安装和编译Whisper.cpp
- ⚙️ 更新配置文件

### 性能优化

- **macOS Apple Silicon**: 自动启用Core ML + Metal加速，性能提升2-3倍
- **macOS Intel**: 使用优化的CPU多线程
- **Linux/Windows + NVIDIA**: 自动启用CUDA加速  
- **其他平台**: 使用优化的CPU模式

### 支持的功能

- **音频格式**: MP3, WAV, M4A, FLAC 等
- **语言支持**: 中文、英文、日语、自动检测
- **智能模型选择**: 系统自动选择最佳可用模型（默认使用 base 模型）
- **离线转录**: 完全本地处理，保护隐私

## 🛠️ MCP工具列表

| 工具名称 | 功能描述 | 状态 |
|---------|---------|------|
| `check_login_status` | 检查B站登录状态 | ✅ |
| `list_accounts` | 列出所有已登录账号 | ✅ |
| `switch_account` | 切换当前使用的账号 | ✅ |
| `post_comment` | 发表文字评论到视频 | ✅ |
| `reply_comment` | 回复评论 | ✅ |
| `get_video_info` | 获取视频详细信息 | ✅ |
| `like_video` | 点赞视频 | ✅ |
| `coin_video` | 投币视频 | ✅ |
| `favorite_video` | 收藏视频 | ✅ |
| `follow_user` | 关注用户 | ✅ |
| `get_user_videos` | 获取用户发布的视频列表 | ✅ |
| `download_media` | 智能下载B站视频/音频 | ✅ |
| `get_video_stream` | 获取视频播放地址 | ✅ |
| `whisper_audio_2_text` | 音频转录为文字（需初始化） | ✅ |

## 💡 使用示例

### 基础操作
```
"帮我给视频BV1234567890发表评论：很棒的内容！"
"获取视频BV1234567890的详细信息"
"点赞视频BV1234567890"
"关注UP主UID12345"
```

### 音频转录
```
"帮我转录这个音频文件：/path/to/audio.mp3"
"将下载的视频音频转录成文字"
```

### 账号管理
```
"列出我当前登录的所有B站账号"
"切换到工作账号"
"检查当前登录状态"
```

## ⚙️ 配置说明

编辑 `config.yaml` 文件来自定义配置：

```yaml
server:
  port: 18666  # MCP服务端口

browser:
  headless: true  # 是否无头模式
  timeout: 30s    # 操作超时时间
  pool_size: 3    # 浏览器池大小

features:
  whisper:
    enabled: false  # 是否启用Whisper转录
    default_model: "auto"  # 默认模型（auto=智能选择）
    language: "zh"  # 默认识别语言
    timeout_seconds: 1200  # 转录超时时间（秒）
```

## 🔧 开发者指南

### 构建命令

```bash
make build          # 构建所有二进制文件
make build-all      # 跨平台构建
make release        # 创建发布包
make test           # 运行测试
make clean          # 清理构建文件
```

### 发布新版本

```bash
# 1. 提交代码
git add . && git commit -m "feat: 新功能"
git push origin main

# 2. 创建版本标签
git tag v1.0.0
git push origin v1.0.0

# 3. GitHub Actions 自动构建和发布
```

## 📦 发布包说明

### 文件大小对比
- **macOS版本**: ~178MB (包含Core ML加速模型)
- **Windows/Linux版本**: ~143MB (仅基础模型)

### 包含内容
- `bilibili-mcp` - MCP服务器主程序
- `bilibili-login` - B站账号登录工具  
- `whisper-init` - Whisper初始化工具
- `models/ggml-base.bin` - Whisper基础模型
- `models/ggml-base.en-encoder.mlmodelc/` - Core ML加速模型（仅macOS）

## 🏗️ 项目架构

```
bilibili-mcp/
├── cmd/                    # 命令行工具
│   ├── server/            # MCP服务器
│   ├── login/             # 登录工具
│   └── whisper-init/      # Whisper初始化
├── internal/
│   ├── bilibili/          # B站业务逻辑
│   │   ├── auth/          # 认证管理
│   │   ├── comment/       # 评论功能
│   │   ├── download/      # 下载功能
│   │   ├── video/         # 视频操作
│   │   └── whisper/       # 音频转录
│   ├── browser/           # 浏览器池管理
│   └── mcp/              # MCP协议实现
├── pkg/                   # 公共包
└── examples/             # 使用示例
```

## 🤝 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 📄 许可证

本项目基于 MIT 许可证开源 - 查看 [LICENSE](LICENSE) 文件了解详情

## 🙏 致谢

- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect) - B站API文档
- [Whisper.cpp](https://github.com/ggerganov/whisper.cpp) - 高性能音频转录
- [Playwright](https://playwright.dev/) - 浏览器自动化

## 📞 支持

- 🐛 **问题反馈**: [GitHub Issues](https://github.com/shirenchuang/bilibili-mcp/issues)
- 💬 **功能建议**: [GitHub Discussions](https://github.com/shirenchuang/bilibili-mcp/discussions)
- 📖 **文档**: 查看项目Wiki

---

⭐ 如果这个项目对你有帮助，请给它一个星标！