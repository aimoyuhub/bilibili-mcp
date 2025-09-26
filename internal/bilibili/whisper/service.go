package whisper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// Service Whisper转录服务
type Service struct {
	config         *config.WhisperConfig
	fullConfig     *config.Config // 完整配置，用于获取解析后的路径
	whisperCLIPath string
}

// TranscribeResult 转录结果
type TranscribeResult struct {
	AudioPath        string      `json:"audio_path"`
	OutputPath       string      `json:"output_path"`
	Text             string      `json:"text"`
	Duration         float64     `json:"duration"`
	Model            string      `json:"model"`
	Language         string      `json:"language"`
	AccelerationType string      `json:"acceleration_type"`
	ProcessTime      float64     `json:"process_time"`
	CreatedAt        time.Time   `json:"created_at"`
	AvailableModels  []ModelInfo `json:"available_models"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	IsCoreMl    bool   `json:"is_core_ml"`
	Description string `json:"description"`
}

// NewService 创建Whisper服务
func NewService(fullCfg *config.Config) (*Service, error) {
	cfg := &fullCfg.Features.Whisper
	if !cfg.Enabled {
		return nil, errors.New("Whisper功能未启用")
	}

	service := &Service{
		config:     cfg,
		fullConfig: fullCfg,
	}

	// 查找whisper-cli可执行文件
	if err := service.findWhisperCLI(); err != nil {
		return nil, errors.Wrap(err, "找不到whisper-cli")
	}

	return service, nil
}

// findWhisperCLI 查找whisper-cli可执行文件
func (s *Service) findWhisperCLI() error {
	// 1. 如果配置中指定了whisper.cpp路径，优先使用解析后的路径
	whisperCppPath := s.fullConfig.GetResolvedWhisperCppPath()
	if whisperCppPath != "" {
		cliPath := filepath.Join(whisperCppPath, "build", "bin", "whisper-cli")
		if _, err := os.Stat(cliPath); err == nil {
			s.whisperCLIPath = cliPath
			logger.Infof("使用配置指定的whisper-cli: %s", cliPath)
			return nil
		}
		logger.Debugf("配置路径中未找到whisper-cli: %s", cliPath)
	}

	// 2. 在PATH中查找
	if path, err := exec.LookPath("whisper-cli"); err == nil {
		s.whisperCLIPath = path
		logger.Infof("在PATH中找到whisper-cli: %s", path)
		return nil
	}

	// 3. 检查常见安装位置
	possiblePaths := []string{
		"/usr/local/bin/whisper-cli",
		"/opt/homebrew/bin/whisper-cli",
		filepath.Join(os.Getenv("HOME"), "whisper.cpp/build/bin/whisper-cli"),
		"./whisper.cpp/build/bin/whisper-cli",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			s.whisperCLIPath = path
			logger.Infof("找到whisper-cli: %s", path)
			return nil
		}
	}

	return errors.New("未找到whisper-cli，请先运行 ./bilibili-whisper-init 进行初始化")
}

// TranscribeAudio 转录音频文件
func (s *Service) TranscribeAudio(ctx context.Context, audioPath string) (*TranscribeResult, error) {
	startTime := time.Now()

	// 验证音频文件存在
	if _, err := os.Stat(audioPath); err != nil {
		return nil, errors.Wrap(err, "音频文件不存在")
	}

	// 转换为WAV格式（如果需要）
	wavPath, err := s.ensureWAVFormat(audioPath)
	if err != nil {
		return nil, errors.Wrap(err, "音频格式转换失败")
	}

	// 准备输出路径
	outputDir := filepath.Dir(audioPath)
	outputBase := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	outputPath := filepath.Join(outputDir, outputBase)

	// 确定使用的模型
	modelPath, modelName, err := s.getModelPath()
	if err != nil {
		return nil, errors.Wrap(err, "获取模型路径失败")
	}

	// 检测加速类型
	accelerationType := s.detectAccelerationType(modelPath)
	logger.Infof("开始转录音频: %s, 模型: %s, 加速: %s", audioPath, modelName, accelerationType)

	// 执行转录
	text, err := s.executeWhisper(ctx, wavPath, modelPath, outputPath)
	if err != nil {
		return nil, errors.Wrap(err, "转录执行失败")
	}

	// 计算处理时间
	processTime := time.Since(startTime).Seconds()

	// 扫描所有可用模型
	availableModels := s.scanAvailableModels()
	logger.Infof("📋 扫描到 %d 个可用模型", len(availableModels))

	result := &TranscribeResult{
		AudioPath:        audioPath,
		OutputPath:       outputPath + ".srt",
		Text:             text,
		Model:            modelName,
		Language:         s.config.Language,
		AccelerationType: accelerationType,
		ProcessTime:      processTime,
		CreatedAt:        time.Now(),
		AvailableModels:  availableModels,
	}

	logger.Infof("转录完成: %s, 耗时: %.2fs", audioPath, processTime)
	return result, nil
}

// ensureWAVFormat 确保音频为WAV格式
func (s *Service) ensureWAVFormat(audioPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(audioPath))
	if ext == ".wav" {
		return audioPath, nil
	}

	// 需要转换为WAV
	wavPath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".wav"

	logger.Infof("转换音频格式: %s -> %s", audioPath, wavPath)

	cmd := exec.Command("ffmpeg",
		"-y", // 覆盖输出文件
		"-i", audioPath,
		"-ar", "16000", // 采样率16kHz
		"-ac", "1", // 单声道
		"-c:a", "pcm_s16le", // 16位PCM编码
		"-hide_banner", // 隐藏版本信息
		wavPath,
	)

	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "ffmpeg转换失败")
	}

	// 验证转换结果
	if _, err := os.Stat(wavPath); err != nil {
		return "", errors.New("转换后的WAV文件不存在")
	}

	return wavPath, nil
}

// getModelPath 获取模型路径和名称
func (s *Service) getModelPath() (string, string, error) {
	requestedModel := s.config.DefaultModel

	// 如果没有指定模型、指定为auto或默认的tiny，则智能选择最佳可用模型
	if requestedModel == "" || requestedModel == "auto" || requestedModel == "tiny" {
		logger.Info("🎯 智能选择最佳可用模型...")
		return s.selectBestAvailableModel()
	}

	// 首先尝试用户明确指定的模型
	modelPath, modelName, err := s.tryGetModel(requestedModel)
	if err == nil {
		logger.Infof("✅ 使用用户指定的模型: %s", requestedModel)
		return modelPath, modelName, nil
	}

	logger.Warnf("⚠️  请求的模型 '%s' 不可用: %v", requestedModel, err)

	// 如果指定的模型不可用，智能选择最佳替代模型
	logger.Infof("🔍 自动选择最佳替代模型...")
	return s.selectBestAvailableModel()
}

// selectBestAvailableModel 选择最佳可用模型
func (s *Service) selectBestAvailableModel() (string, string, error) {
	// 按质量优先级排序（推荐的选择顺序）
	preferredModels := []string{"base", "small", "medium", "large", "tiny"}

	logger.Infof("🔍 按优先级搜索最佳模型: %v", preferredModels)

	for _, model := range preferredModels {
		modelPath, modelName, err := s.tryGetModel(model)
		if err == nil {
			logger.Infof("✅ 自动选择最佳可用模型: %s", model)
			return modelPath, modelName, nil
		}
		logger.Debugf("❌ 模型 %s 不可用: %v", model, err)
	}

	return "", "", errors.Errorf("未找到任何可用的模型，请运行 ./whisper-init 下载模型")
}

// tryGetModel 尝试获取指定模型的路径
func (s *Service) tryGetModel(modelName string) (string, string, error) {
	var possiblePaths []string

	// 1. 检查预制模型目录 (./models/)
	prebuiltPath := fmt.Sprintf("./models/ggml-%s.bin", modelName)
	possiblePaths = append(possiblePaths, prebuiltPath)

	// 2. 检查配置中的模型路径（使用解析后的路径）
	resolvedModelPath := s.fullConfig.GetResolvedModelPath()
	if resolvedModelPath != "" && strings.Contains(resolvedModelPath, modelName) {
		possiblePaths = append(possiblePaths, resolvedModelPath)
	}

	// 3. 检查whisper.cpp安装目录中的模型（使用解析后的路径）
	whisperCppPath := s.fullConfig.GetResolvedWhisperCppPath()
	if whisperCppPath != "" {
		whisperModelPath := filepath.Join(whisperCppPath, "models", fmt.Sprintf("ggml-%s.bin", modelName))
		possiblePaths = append(possiblePaths, whisperModelPath)
	}

	// 尝试每个可能的路径
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			// 转换为绝对路径
			absPath, err := filepath.Abs(path)
			if err != nil {
				absPath = path
			}
			return absPath, modelName, nil
		}
	}

	return "", "", errors.Errorf("模型 %s 在以下位置都不存在: %v，请运行 ./whisper-init 下载模型", modelName, possiblePaths)
}

// executeWhisper 执行Whisper转录
func (s *Service) executeWhisper(ctx context.Context, audioPath, modelPath, outputPath string) (string, error) {
	// 检测系统和加速类型
	accelerationType := s.detectAccelerationType(modelPath)
	logger.Infof("🎯 检测到加速类型: %s", accelerationType)

	// 构建命令参数
	args := []string{
		"-f", audioPath,
		"-m", modelPath,
		"-osrt", // 输出SRT格式
		"-l", s.config.Language,
		"-of", outputPath,
	}

	// 根据加速类型配置参数
	switch accelerationType {
	case "Core ML":
		logger.Info("🚀 启用 Core ML 加速 (Apple Neural Engine)")
		// Core ML 会自动使用，不需要额外参数
	case "Metal":
		logger.Info("⚡ 启用 Metal GPU 加速 (Apple Silicon)")
		// Metal 默认启用，不需要额外参数
	case "CUDA":
		logger.Info("🔥 启用 CUDA GPU 加速 (NVIDIA)")
		// CUDA 会自动使用，不需要额外参数
	case "CPU":
		logger.Info("💻 使用 CPU 多线程模式")
		args = append(args, "-ng") // 禁用GPU
		if s.config.CPUThreads > 0 {
			args = append(args, "-t", strconv.Itoa(s.config.CPUThreads))
			logger.Infof("🧵 CPU 线程数: %d", s.config.CPUThreads)
		}
	}

	logger.Infof("🔧 执行whisper命令: %s %s", s.whisperCLIPath, strings.Join(args, " "))

	// 设置更长的超时时间 - 默认20分钟，大文件可能需要更长时间
	timeoutSeconds := s.config.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 1200 // 默认20分钟
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 创建命令
	cmd := exec.CommandContext(timeoutCtx, s.whisperCLIPath, args...)

	logger.Infof("⏱️ 设置超时时间: %d秒 (%.1f分钟)", timeoutSeconds, float64(timeoutSeconds)/60)

	// 执行命令并实时输出日志
	output, err := cmd.CombinedOutput()

	// 解析输出日志，提取有用信息
	s.parseWhisperOutput(string(output), accelerationType)

	if err != nil {
		logger.Errorf("❌ Whisper执行失败: %s", err)
		logger.Errorf("📝 详细输出: %s", string(output))

		// 检查是否是Core ML相关错误，尝试降级
		if strings.Contains(string(output), "Core ML") || strings.Contains(string(output), "failed to initialize") {
			logger.Warn("⚠️  Core ML 初始化失败，尝试降级到 Metal/CPU 模式")
			return s.executeWhisperFallback(ctx, audioPath, modelPath, outputPath, accelerationType)
		}

		// 检查是否是GPU相关错误
		if strings.Contains(string(output), "CUDA") || strings.Contains(string(output), "Metal") {
			logger.Warn("⚠️  GPU 加速失败，尝试降级到 CPU 模式")
			return s.executeWhisperFallback(ctx, audioPath, modelPath, outputPath, accelerationType)
		}

		return "", errors.Wrap(err, "Whisper执行失败")
	}

	logger.Info("✅ Whisper 转录执行成功")

	// 读取生成的SRT文件内容
	srtPath := outputPath + ".srt"
	if _, err := os.Stat(srtPath); err != nil {
		return "", errors.New("SRT文件未生成")
	}

	content, err := os.ReadFile(srtPath)
	if err != nil {
		return "", errors.Wrap(err, "读取SRT文件失败")
	}

	// 提取纯文本
	text := s.extractTextFromSRT(string(content))

	logger.Infof("📄 转录完成，提取文本长度: %d 字符", len(text))

	return text, nil
}

// extractTextFromSRT 从SRT内容中提取纯文本
func (s *Service) extractTextFromSRT(srtContent string) string {
	lines := strings.Split(srtContent, "\n")
	var textLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过序号行和时间戳行
		if line == "" || isNumber(line) || strings.Contains(line, "-->") {
			continue
		}
		textLines = append(textLines, line)
	}

	return strings.Join(textLines, " ")
}

// isNumber 检查字符串是否为数字
func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// IsEnabled 检查Whisper功能是否启用
func (s *Service) IsEnabled() bool {
	return s.config.Enabled
}

// GetConfig 获取配置信息
func (s *Service) GetConfig() *config.WhisperConfig {
	return s.config
}

// detectAccelerationType 检测加速类型
func (s *Service) detectAccelerationType(modelPath string) string {
	// 检查是否有对应的Core ML模型
	if strings.Contains(modelPath, "models/") {
		// 预制模型目录，检查是否有Core ML版本
		modelName := filepath.Base(modelPath)
		modelName = strings.TrimSuffix(modelName, ".bin")
		coreMLPath := filepath.Join(filepath.Dir(modelPath), modelName+".en-encoder.mlmodelc")

		if _, err := os.Stat(coreMLPath); err == nil && runtime.GOOS == "darwin" {
			return "Core ML"
		}
	}

	// 检查系统和GPU支持
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "Metal" // Apple Silicon 支持 Metal
		}
		return "CPU" // Intel Mac 使用 CPU
	case "linux", "windows":
		// 检查是否有 NVIDIA GPU
		if s.hasNVIDIAGPU() {
			return "CUDA"
		}
		return "CPU"
	default:
		return "CPU"
	}
}

// hasNVIDIAGPU 检查是否有NVIDIA GPU
func (s *Service) hasNVIDIAGPU() bool {
	cmd := exec.Command("nvidia-smi")
	return cmd.Run() == nil
}

// parseWhisperOutput 解析Whisper输出日志
func (s *Service) parseWhisperOutput(output, expectedAcceleration string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查实际使用的加速类型
		if strings.Contains(line, "Metal") || strings.Contains(line, "GPU") {
			if strings.Contains(line, "enabled") || strings.Contains(line, "using") {
				logger.Infof("🔍 检测到实际使用: Metal GPU 加速")
			}
		}

		if strings.Contains(line, "Core ML") {
			if strings.Contains(line, "enabled") || strings.Contains(line, "using") {
				logger.Infof("🔍 检测到实际使用: Core ML 加速")
			} else if strings.Contains(line, "failed") || strings.Contains(line, "error") {
				logger.Warnf("⚠️  Core ML 问题: %s", line)
			}
		}

		if strings.Contains(line, "CUDA") {
			if strings.Contains(line, "enabled") || strings.Contains(line, "using") {
				logger.Infof("🔍 检测到实际使用: CUDA GPU 加速")
			} else if strings.Contains(line, "failed") || strings.Contains(line, "error") {
				logger.Warnf("⚠️  CUDA 问题: %s", line)
			}
		}

		// 输出处理进度信息
		if strings.Contains(line, "progress") || strings.Contains(line, "%") {
			logger.Infof("📊 处理进度: %s", line)
		}

		// 输出时间信息
		if strings.Contains(line, "processing time") || strings.Contains(line, "real time factor") {
			logger.Infof("⏱️  性能信息: %s", line)
		}

		// 输出错误信息
		if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "failed") {
			logger.Warnf("⚠️  警告: %s", line)
		}
	}
}

// executeWhisperFallback 降级执行Whisper
func (s *Service) executeWhisperFallback(ctx context.Context, audioPath, modelPath, outputPath, failedType string) (string, error) {
	logger.Warnf("🔄 %s 模式失败，尝试降级处理", failedType)

	var fallbackType string
	args := []string{
		"-f", audioPath,
		"-m", modelPath,
		"-osrt",
		"-l", s.config.Language,
		"-of", outputPath,
	}

	// 根据失败的类型选择降级策略
	switch failedType {
	case "Core ML":
		// Core ML 失败，尝试 Metal 或 CPU
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			fallbackType = "Metal"
			logger.Info("🔄 降级到 Metal GPU 加速")
		} else {
			fallbackType = "CPU"
			logger.Info("🔄 降级到 CPU 模式")
			args = append(args, "-ng")
			if s.config.CPUThreads > 0 {
				args = append(args, "-t", strconv.Itoa(s.config.CPUThreads))
			}
		}
	case "Metal", "CUDA":
		// GPU 失败，降级到 CPU
		fallbackType = "CPU"
		logger.Info("🔄 降级到 CPU 模式")
		args = append(args, "-ng")
		if s.config.CPUThreads > 0 {
			args = append(args, "-t", strconv.Itoa(s.config.CPUThreads))
		}
	default:
		return "", errors.New("所有加速模式都失败了")
	}

	logger.Infof("🔧 降级命令: %s %s", s.whisperCLIPath, strings.Join(args, " "))

	// 执行降级命令
	cmd := exec.CommandContext(ctx, s.whisperCLIPath, args...)
	output, err := cmd.CombinedOutput()

	s.parseWhisperOutput(string(output), fallbackType)

	if err != nil {
		logger.Errorf("❌ 降级模式也失败: %s", err)
		logger.Errorf("📝 详细输出: %s", string(output))
		return "", errors.Wrap(err, "降级模式转录失败")
	}

	logger.Infof("✅ 降级模式 (%s) 转录成功", fallbackType)

	// 读取SRT文件
	srtPath := outputPath + ".srt"
	content, err := os.ReadFile(srtPath)
	if err != nil {
		return "", errors.Wrap(err, "读取SRT文件失败")
	}

	return s.extractTextFromSRT(string(content)), nil
}

// scanAvailableModels 扫描所有可用的模型
func (s *Service) scanAvailableModels() []ModelInfo {
	var models []ModelInfo
	modelNames := []string{"tiny", "base", "small", "medium", "large"}

	// 模型描述映射
	modelDescriptions := map[string]string{
		"tiny":   "最快速度，基础准确性 (~39MB)",
		"base":   "平衡速度和质量 (~142MB)",
		"small":  "推荐选择，高质量 (~466MB)",
		"medium": "专业级质量 (~1.5GB)",
		"large":  "最佳质量 (~2.9GB)",
	}

	for _, modelName := range modelNames {
		// 检查各个可能的位置
		paths := s.getModelPaths(modelName)

		for _, path := range paths {
			if stat, err := os.Stat(path); err == nil {
				// 转换为绝对路径
				absPath, err := filepath.Abs(path)
				if err != nil {
					absPath = path
				}

				model := ModelInfo{
					Name:        modelName,
					Path:        absPath,
					Size:        stat.Size(),
					IsCoreMl:    false,
					Description: modelDescriptions[modelName],
				}

				// 检查是否有对应的Core ML模型
				if s.hasCoreMlModel(modelName) {
					model.IsCoreMl = true
					model.Description += " + Core ML加速"
				}

				models = append(models, model)
				break // 找到一个就够了，避免重复
			}
		}
	}

	return models
}

// getModelPaths 获取模型的所有可能路径
func (s *Service) getModelPaths(modelName string) []string {
	var paths []string

	// 1. 预制模型目录
	prebuiltPath := fmt.Sprintf("./models/ggml-%s.bin", modelName)
	paths = append(paths, prebuiltPath)

	// 2. 配置文件指定的路径（使用解析后的路径）
	resolvedModelPath := s.fullConfig.GetResolvedModelPath()
	if resolvedModelPath != "" && strings.Contains(resolvedModelPath, modelName) {
		paths = append(paths, resolvedModelPath)
	}

	// 3. whisper.cpp安装目录（使用解析后的路径）
	whisperCppPath := s.fullConfig.GetResolvedWhisperCppPath()
	if whisperCppPath != "" {
		whisperPath := filepath.Join(whisperCppPath, "models", fmt.Sprintf("ggml-%s.bin", modelName))
		paths = append(paths, whisperPath)
	}

	return paths
}

// hasCoreMlModel 检查是否有对应的Core ML模型
func (s *Service) hasCoreMlModel(modelName string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	// 检查预制模型目录中的Core ML模型
	coreMLPath := fmt.Sprintf("./models/ggml-%s-encoder.mlmodelc", modelName)
	if _, err := os.Stat(coreMLPath); err == nil {
		return true
	}

	// 检查whisper.cpp目录中的Core ML模型（使用解析后的路径）
	whisperCppPath := s.fullConfig.GetResolvedWhisperCppPath()
	if whisperCppPath != "" {
		coreMLPath = filepath.Join(whisperCppPath, "models", fmt.Sprintf("ggml-%s-encoder.mlmodelc", modelName))
		if _, err := os.Stat(coreMLPath); err == nil {
			return true
		}
	}

	return false
}
