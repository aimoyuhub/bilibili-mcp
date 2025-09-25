# bilibili-mcp

MCP for bilibili.com - B站自动化操作的标准化接口

## 功能特性

- 🔐 **多账号管理**: 支持多个B站账号切换和管理
- 💬 **智能评论**: 文字和图片评论支持
- 📹 **视频操作**: 点赞、投币、收藏、获取信息
- 👥 **用户互动**: 关注、获取用户信息和视频列表
- 🎵 **音频转录**: 集成 Whisper.cpp，本地音频转文字（需要初始化）
- 🌐 **标准化接口**: 遵循MCP协议，支持各种AI客户端

## 快速开始

### 1. 下载和安装

```bash
# 下载预编译二进制文件（推荐）
# 或者从源码编译
git clone https://github.com/shirenchuang/bilibili-mcp.git
cd bilibili-mcp
go build -o bilibili-mcp ./cmd/server
go build -o bilibili-login ./cmd/login
```

### 2. 登录B站账号

```bash
# 登录默认账号
./bilibili-login

# 登录指定账号
./bilibili-login -account work
./bilibili-login -account personal
```

### 3. （可选）初始化Whisper音频转录

如果需要使用音频转录功能：

1. **下载预制模型**（推荐）：
   ```bash
   mkdir -p models
   
   # 基础模型（必需）
   curl -L -o models/ggml-tiny.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin?download=true"
   curl -L -o models/ggml-base.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin?download=true"
   
   # Core ML 加速模型（macOS 推荐，性能提升 2-3 倍）
   curl -L -o models/ggml-tiny.en-encoder.mlmodelc.zip "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en-encoder.mlmodelc.zip?download=true"
   curl -L -o models/ggml-base.en-encoder.mlmodelc.zip "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en-encoder.mlmodelc.zip?download=true"
   
   # 解压 Core ML 模型
   cd models && unzip -q ggml-tiny.en-encoder.mlmodelc.zip && unzip -q ggml-base.en-encoder.mlmodelc.zip && cd ..
   ```

2. **运行初始化工具**：
   ```bash
   ./whisper-init
   ```

详见 [Whisper设置指南](WHISPER_SETUP.md)

### 4. 启动MCP服务

```bash
./bilibili-mcp
```

服务将运行在 `http://localhost:18666/mcp`

### 5. 在AI客户端中配置

#### Cursor
在项目根目录创建 `.cursor/mcp.json`：
```json
{
  "mcpServers": {
    "bilibili-mcp": {
      "url": "http://localhost:18666/mcp",
      "description": "B站内容操作服务 - MCP Streamable HTTP"
    }
  }
}
```

#### Claude Code CLI
```bash
claude mcp add --transport http bilibili-mcp http://localhost:18666/mcp
```

## MCP工具列表

- `check_login_status` - 检查登录状态
- `list_accounts` - 列出所有账号
- `switch_account` - 切换账号
- `post_comment` - 发表文字评论
- ~~`post_image_comment` - 发表图片评论~~（暂时不可用）
- `reply_comment` - 回复评论
- `get_video_info` - 获取视频信息
- `like_video` - 点赞视频
- `coin_video` - 投币视频
- `favorite_video` - 收藏视频
- `follow_user` - 关注用户
- `get_user_videos` - 获取用户视频列表
- `whisper_audio_2_text` - 音频转录为文字（智能选择最佳模型，需要初始化）

## 配置说明

编辑 `config.yaml` 文件来自定义配置：

```yaml
server:
  port: 18666  # MCP服务端口

browser:
  headless: true  # 是否无头模式
  timeout: 30s    # 操作超时时间

features:
  whisper:
    enabled: false  # 是否启用Whisper转录
```

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！
