# bilibili-mcp 使用示例

这个目录包含了各种AI客户端集成bilibili-mcp的配置示例和使用方法。

## 🚀 快速开始

### 1. 启动服务

```bash
# 首次使用，先登录B站账号
./bilibili-login

# 启动MCP服务
./bilibili-mcp
```

### 2. 选择客户端集成

- **[Cursor](./cursor/)** - 在Cursor中使用bilibili-mcp
- **[VSCode](./vscode/)** - 在VSCode中使用bilibili-mcp  
- **[Claude Code CLI](./claude/)** - 在Claude CLI中使用bilibili-mcp

## 📋 可用功能

### 基础功能
- ✅ `check_login_status` - 检查登录状态
- ✅ `list_accounts` - 列出所有账号
- ✅ `switch_account` - 切换账号

### 视频操作
- ✅ `get_video_info` - 获取视频信息
- ✅ `post_comment` - 发表评论
- ✅ `post_image_comment` - 发表图片评论
- 🔄 `like_video` - 点赞视频（开发中）
- 🔄 `coin_video` - 投币视频（开发中）
- 🔄 `favorite_video` - 收藏视频（开发中）

### 用户操作
- 🔄 `follow_user` - 关注用户（开发中）
- 🔄 `get_user_videos` - 获取用户视频列表（开发中）

### 可选功能
- ⏳ `transcribe_video` - 视频转录（需要Whisper）

## 💡 使用示例

### 在Cursor中使用

1. 配置MCP服务：
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

2. 在聊天中使用：
```
请帮我给这个视频BV1234567890发表评论："很棒的视频！"
```

### 在Claude Code CLI中使用

1. 添加MCP服务器：
```bash
claude mcp add --transport http bilibili-mcp http://localhost:18666/mcp
```

2. 使用功能：
```bash
claude chat
# 然后在对话中使用B站相关功能
```

## 🔧 故障排除

### 常见问题

1. **MCP服务连接失败**
   - 确保bilibili-mcp服务正在运行
   - 检查端口18666是否被占用
   - 确认防火墙设置

2. **登录状态丢失**
   - 重新运行登录工具：`./bilibili-login`
   - 检查cookies目录权限

3. **评论发送失败**
   - 确认已登录正确的账号
   - 检查评论内容是否符合B站规范
   - 确认视频ID格式正确（BV号或AV号）

### 日志查看

```bash
# 查看服务日志
tail -f logs/bilibili-mcp.log

# 或者以非无头模式运行查看浏览器操作
./bilibili-mcp -config config.yaml
# 然后编辑config.yaml设置 headless: false
```

## 📚 更多资源

- [项目主页](https://github.com/shirenchuang/bilibili-mcp)
- [问题反馈](https://github.com/shirenchuang/bilibili-mcp/issues)
- [B站API文档](https://github.com/SocialSisterYi/bilibili-API-collect)
