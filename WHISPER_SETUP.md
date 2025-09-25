# Whisper.cpp 音频转录功能设置指南

## 概述

本项目现已集成 Whisper.cpp 音频转录功能，可以将音频文件转录为文字。该功能使用最小的 `tiny` 模型以节省资源，同时保持良好的转录效果。

## 快速开始

### 1. 下载预制模型（推荐）

为了确保离线可用和更快的初始化，建议先下载预制模型：

```bash
# 创建模型目录
mkdir -p models

# 基础模型（必需）
curl -L -o models/ggml-tiny.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin?download=true"
curl -L -o models/ggml-base.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin?download=true"

# Core ML 加速模型（macOS 推荐，性能提升 2-3 倍）
curl -L -o models/ggml-tiny.en-encoder.mlmodelc.zip "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en-encoder.mlmodelc.zip?download=true"
curl -L -o models/ggml-base.en-encoder.mlmodelc.zip "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en-encoder.mlmodelc.zip?download=true"

# 解压 Core ML 模型
cd models
unzip -q ggml-tiny.en-encoder.mlmodelc.zip
unzip -q ggml-base.en-encoder.mlmodelc.zip
cd ..
```

### 2. 初始化 Whisper.cpp

运行初始化工具来自动安装和配置 Whisper.cpp：

```bash
./whisper-init
```

初始化工具会自动：
- 🖥️ **智能系统检测**：自动识别操作系统和GPU类型
- 📦 **预制模型优先**：优先使用已下载的预制模型
- ⚡ **GPU加速配置**：
  - macOS Apple Silicon：启用 Metal 加速
  - Linux/Windows：检测并启用 CUDA 加速（如有NVIDIA GPU）
  - 其他情况：使用优化的CPU模式
- 🔧 **自动安装编译**：如果未安装，引导安装到 `~/whisper.cpp`
- ⚙️ **配置文件更新**：自动更新配置启用 Whisper 功能

### 2. 使用音频转录功能

初始化完成后，您可以使用 `whisper_audio_2_text` MCP 工具：

```json
{
  "name": "whisper_audio_2_text",
  "arguments": {
    "audio_path": "/path/to/your/audio.mp3",
    "language": "zh",
    "model": "tiny"
  }
}
```

## 支持的功能

### 音频格式支持
- **输入格式**：MP3, WAV, M4A, FLAC 等常见格式
- **自动转换**：自动转换为 Whisper 需要的 16kHz 单声道 WAV 格式

### 语言支持
- `zh` - 中文（默认）
- `en` - 英文
- `ja` - 日语
- `auto` - 自动检测

### 模型选择
| 模型名 | 大小 | 速度 | 准确性 | 推荐场景 |
|--------|------|------|--------|----------|
| `tiny` | 39MB | 最快 | 基础 | **默认选择**，快速转录 |
| `base` | 142MB| 快 | 良好 | 平衡速度和质量 |
| `small` | 466MB| 较快 | 很好 | 高质量需求 |
| `medium` | 1.5GB| 慢 | 优秀 | 专业转录 |
| `large` | 2.9GB| 很慢 | 最佳 | 最高质量 |

## 配置选项

配置文件 `config.yaml` 中的 Whisper 相关设置：

```yaml
features:
  whisper:
    enabled: true  # 启用 Whisper 功能
    whisper_cpp_path: "/Users/yourname/whisper.cpp"  # Whisper.cpp 安装路径
    model_path: "/Users/yourname/whisper.cpp/models/ggml-tiny.bin"  # 模型文件路径
    default_model: "tiny"  # 默认使用的模型
    language: "zh"  # 默认识别语言
    cpu_threads: 4  # CPU 线程数
    timeout_seconds: 600  # 转换超时时间（秒）
    enable_gpu: true  # 启用GPU加速
    enable_core_ml: true  # 启用Core ML加速（macOS）
```

## 性能优化

### macOS
- **Apple Silicon (M1/M2/M3/M4)**：
  - 自动启用 **Metal GPU 加速**
  - 支持 **Core ML** 优化模型
  - 建议使用 `tiny` 或 `base` 模型以获得最佳性能
- **Intel Mac**：
  - 使用优化的 CPU 多线程处理
  - 可调整 `cpu_threads` 参数优化性能

### Linux
- **NVIDIA GPU**：自动检测并启用 CUDA 加速
- **CPU模式**：使用多线程优化
- 建议根据 CPU 核心数设置 `cpu_threads`

### Windows  
- **NVIDIA GPU**：自动检测并启用 CUDA 加速
- **CPU模式**：使用多线程优化
- 需要安装 Visual Studio Build Tools 进行编译

### 内存使用
- `tiny` 模型：约 100MB 内存
- `base` 模型：约 200MB 内存
- `small` 模型：约 500MB 内存

### 超时设置
- **默认超时**：20分钟（1200秒）
- **大文件建议**：可在配置文件中调整 `timeout_seconds`
- **建议值**：
  - 小文件（<10分钟）：600秒
  - 中等文件（10-30分钟）：1200秒
  - 大文件（>30分钟）：1800秒或更长

## 故障排除

### 1. 初始化失败

**问题**：运行 `./bilibili-whisper-init` 时出错

**解决方案**：
- 确保已安装 `git`、`cmake`、`make`
- macOS 用户需要安装 Xcode Command Line Tools：
  ```bash
  xcode-select --install
  ```
- 确保有足够的磁盘空间（至少 1GB）

### 2. 编译失败

**问题**：Whisper.cpp 编译失败

**解决方案**：
- 检查是否安装了完整的开发工具链
- 尝试手动编译：
  ```bash
  cd ~/whisper.cpp
  mkdir build && cd build
  cmake -DCMAKE_BUILD_TYPE=Release ..
  make -j$(nproc)
  ```

### 3. 模型下载失败

**问题**：模型文件下载失败

**解决方案**：
- 检查网络连接
- 手动下载模型：
  ```bash
  cd ~/whisper.cpp/models
  bash download-ggml-model.sh tiny
  ```

### 4. 转录失败

**问题**：音频转录时出错

**解决方案**：
- 检查音频文件是否存在且可读
- 确保安装了 `ffmpeg`：
  ```bash
  # macOS
  brew install ffmpeg
  
  # Ubuntu/Debian
  sudo apt install ffmpeg
  ```
- 检查配置文件中的路径是否正确

### 5. Core ML 错误

**问题**：macOS 上出现 Core ML 相关错误

**解决方案**：
- 在配置文件中禁用 Core ML：
  ```yaml
  features:
    whisper:
      enable_core_ml: false
  ```
- 或者重新下载 Core ML 模型

## 使用示例

### 基础使用

```json
{
  "name": "whisper_audio_2_text",
  "arguments": {
    "audio_path": "./downloads/audio.mp3"
  }
}
```

### 指定语言和模型

```json
{
  "name": "whisper_audio_2_text",
  "arguments": {
    "audio_path": "./downloads/english_audio.wav",
    "language": "en",
    "model": "base"
  }
}
```

### 转录结果

转录完成后会返回：
- **转录文本**：提取的纯文本内容
- **SRT文件**：包含时间轴的字幕文件（完整绝对路径）
- **处理信息**：使用的模型、语言、加速类型、处理时间等
- **加速信息**：详细显示使用的加速类型（Core ML、Metal、CUDA或CPU）
- **可用模型列表**：显示当前系统中所有可用的模型及其详细信息

#### 可用模型显示格式

```
📚 当前可用模型
 ✅ tiny - 最快速度，基础准确性 (~39MB) + Core ML加速 🚀 [74.1MB]
    base - 平衡速度和质量 (~142MB) + Core ML加速 🚀 [141.1MB]
```

- **✅** 标记当前使用的模型
- **🚀** 表示支持Core ML加速
- **[文件大小]** 显示实际文件大小

### 智能加速检测

系统会自动检测并选择最优的加速方式：

1. **macOS + 预制Core ML模型** → 使用 Core ML 加速（最快）
2. **macOS Apple Silicon** → 使用 Metal GPU 加速
3. **Linux/Windows + NVIDIA GPU** → 使用 CUDA 加速
4. **其他情况** → 使用优化的CPU多线程模式

### 智能模型选择

系统现在支持智能模型选择，**建议不传递model参数**，让系统自动选择最佳可用模型：

#### 推荐使用方式（无需指定模型）

```json
{
  "tool": "whisper_audio_2_text", 
  "arguments": {
    "audio_path": "/path/to/audio.mp3"
    // 不传model参数，系统自动选择最佳模型
  }
}
```

#### 配置说明

```yaml
features:
  whisper:
    default_model: "auto"  # 推荐设置，智能选择最佳模型
```

#### 智能选择策略

1. **优选顺序**：`base` → `small` → `medium` → `large` → `tiny`
2. **自动模式**：不传model参数或使用 `"model": "auto"`
3. **手动指定**：明确指定模型时优先使用，不存在时自动降级
4. **日志提示**：显示实际使用的模型和选择原因

**智能选择示例**：
```
🎯 智能选择最佳可用模型...
🔍 按优先级搜索最佳模型: [base small medium large tiny]
✅ 自动选择最佳可用模型: base
```

**手动指定示例**（可选）：
```json
{
  "tool": "whisper_audio_2_text",
  "arguments": {
    "audio_path": "/path/to/audio.mp3",
    "model": "base"  // 明确指定使用base模型
  }
}
```

### 转录日志

转录过程中会显示详细日志：
```
🎯 检测到加速类型: Core ML
🚀 启用 Core ML 加速 (Apple Neural Engine)
🔧 执行whisper命令: /path/to/whisper-cli -f audio.wav -m model.bin -osrt -l zh -of output
⏱️ 设置超时时间: 1200秒 (20.0分钟)
🔍 检测到实际使用: Core ML 加速
📊 处理进度: [========>  ] 80%
⏱️ 性能信息: processing time: 15.2s, real time factor: 0.25
✅ Whisper 转录执行成功
📄 转录完成，提取文本长度: 1024 字符
```

## 注意事项

1. **首次使用**：第一次转录会比较慢，因为需要加载模型
2. **文件大小**：建议音频文件不超过 100MB，过大的文件可能导致超时
3. **网络需求**：仅在下载模型时需要网络连接，转录过程完全离线
4. **隐私保护**：所有转录都在本地完成，不会上传到任何服务器

## 更新和维护

### 更新 Whisper.cpp

```bash
cd ~/whisper.cpp
git pull
cd build
make clean
make -j$(nproc)
```

### 下载新模型

```bash
cd ~/whisper.cpp/models
bash download-ggml-model.sh [model_name]
```

### 清理空间

```bash
# 删除不需要的模型文件
rm ~/whisper.cpp/models/ggml-*.bin

# 保留 tiny 模型
cd ~/whisper.cpp/models
bash download-ggml-model.sh tiny
```

---

如果遇到问题，请查看日志文件 `logs/bilibili-mcp.log` 获取详细错误信息。
