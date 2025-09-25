# GitHub Actions 自动化

本项目使用 GitHub Actions 实现自动化构建、测试和发布。

## 🔄 工作流程

### 1. CI (持续集成) - `ci.yml`

**触发条件:**
- 推送到 `main` 或 `develop` 分支
- 创建针对 `main` 或 `develop` 分支的 Pull Request

**执行内容:**
- ✅ 代码格式化检查
- ✅ 运行单元测试
- ✅ 生成测试覆盖率报告
- ✅ 跨平台构建检查
- ✅ 代码质量检查 (golangci-lint)

### 2. Release (自动发布) - `release.yml`

**触发条件:**
- 推送版本标签 (如 `v1.0.0`, `v2.1.3`)

**执行内容:**
- 🔨 跨平台构建 (macOS, Windows, Linux)
- 📦 智能分平台打包:
  - **macOS**: 包含 Core ML 加速模型 (~180MB)
  - **Windows/Linux**: 仅包含基础模型 (~142MB)
- 🔐 生成 SHA256 校验和文件
- 🚀 自动创建 GitHub Release
- 📄 生成详细的发布说明

## 🏷️ 如何发布新版本

### 1. 本地准备
```bash
# 确保代码是最新的
git pull origin main

# 运行测试确保一切正常
make test

# 本地测试构建
make release
```

### 2. 创建版本标签
```bash
# 创建版本标签 (遵循语义化版本)
git tag v1.0.0

# 推送标签到远程仓库
git push origin v1.0.0
```

### 3. 自动发布
推送标签后，GitHub Actions 会自动：
1. 运行完整的 CI 检查
2. 下载 Whisper 模型文件
3. 构建所有平台的二进制文件
4. 创建分平台发布包
5. 生成校验和文件
6. 创建 GitHub Release 并上传文件

## 📦 发布包说明

### 文件命名规则
```
bilibili-mcp-v{版本号}-{平台}-{架构}.{扩展名}
```

### 平台支持
- `darwin-arm64` - macOS Apple Silicon (M1/M2/M3/M4)
- `darwin-amd64` - macOS Intel
- `windows-amd64` - Windows 64位
- `linux-amd64` - Linux 64位

### 包含内容
每个发布包都包含：
- `bilibili-mcp` - MCP 服务器
- `bilibili-login` - 登录工具
- `whisper-init` - Whisper 初始化工具
- `models/ggml-base.bin` - Whisper 基础模型
- `models/ggml-base.en-encoder.mlmodelc/` - Core ML 模型 (仅 macOS)

## 🔐 安全性

- 所有 secrets 通过 GitHub 环境变量管理
- 使用官方 GitHub Actions
- 自动生成 SHA256 校验和
- 版本标签保护

## 🛠️ 本地开发

如果需要在本地模拟 GitHub Actions：
```bash
# 安装 act (GitHub Actions 本地运行工具)
brew install act

# 运行 CI 工作流程
act push

# 运行发布工作流程 (需要设置环境变量)
act push --eventpath .github/workflows/test-event.json
```

## 📊 监控

- CI 状态徽章: 在主 README 中显示
- 测试覆盖率: 通过 Codecov 跟踪
- 发布状态: 在 GitHub Releases 页面查看

## ⚡ 性能优化

- 使用 Go 模块缓存加速构建
- 并行执行测试和检查
- 智能模型下载 (仅下载需要的文件)
- 压缩发布包减少下载时间
