package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

const (
	// 默认使用基础模型
	defaultModel = "base"
	// Whisper.cpp GitHub仓库
	whisperRepo = "https://github.com/ggerganov/whisper.cpp.git"
)

// WhisperSetup Whisper设置结构
type WhisperSetup struct {
	WhisperCppPath string
	ModelPath      string
	IsInstalled    bool
	PrebuiltModels []string
}

// SystemInfo 系统信息
type SystemInfo struct {
	OS            string
	Arch          string
	HasGPU        bool
	GPUType       string
	SupportsMetal bool
	SupportsCUDA  bool
}

func main() {
	fmt.Println("🎤 Whisper.cpp 初始化工具")
	fmt.Println("============================")

	setup := &WhisperSetup{}

	// 0. 检测系统信息
	sysInfo := detectSystemInfo()
	displaySystemInfo(sysInfo)

	// 1. 检查预制模型
	if err := setup.checkPrebuiltModels(); err != nil {
		logger.Errorf("检查预制模型失败: %v", err)
		os.Exit(1)
	}

	// 2. 检查用户是否已安装whisper.cpp
	if err := setup.checkExistingInstallation(); err != nil {
		logger.Errorf("检查现有安装失败: %v", err)
		os.Exit(1)
	}

	// 3. 如果没有安装，引导用户安装
	if !setup.IsInstalled {
		if err := setup.installWhisperCpp(sysInfo); err != nil {
			logger.Errorf("安装 Whisper.cpp 失败: %v", err)
			os.Exit(1)
		}
	}

	// 4. 设置模型（使用预制模型或现有模型）
	if err := setup.setupModels(); err != nil {
		logger.Errorf("设置模型失败: %v", err)
		os.Exit(1)
	}

	// 5. 更新配置文件
	if err := setup.updateConfig(); err != nil {
		logger.Errorf("更新配置失败: %v", err)
		os.Exit(1)
	}

	fmt.Println("\n🎉 Whisper.cpp 初始化完成！")
	fmt.Printf("   Whisper.cpp 路径: %s\n", setup.WhisperCppPath)
	fmt.Printf("   模型路径: %s\n", setup.ModelPath)
	fmt.Printf("   GPU 加速: %s\n", getGPUStatus(sysInfo))
	fmt.Println("   现在您可以使用 whisper_audio_2_text 功能了！")
}

// detectSystemInfo 检测系统信息
func detectSystemInfo() *SystemInfo {
	info := &SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// 检测GPU支持
	switch info.OS {
	case "darwin":
		info.SupportsMetal = info.Arch == "arm64" // Apple Silicon支持Metal
		info.HasGPU = info.SupportsMetal
		if info.SupportsMetal {
			info.GPUType = "Metal (Apple Silicon)"
		}
	case "linux", "windows":
		// 检查NVIDIA GPU
		if checkNVIDIAGPU() {
			info.SupportsCUDA = true
			info.HasGPU = true
			info.GPUType = "NVIDIA CUDA"
		}
	}

	return info
}

// checkNVIDIAGPU 检查是否有NVIDIA GPU
func checkNVIDIAGPU() bool {
	cmd := exec.Command("nvidia-smi")
	return cmd.Run() == nil
}

// displaySystemInfo 显示系统信息
func displaySystemInfo(info *SystemInfo) {
	fmt.Println("\n🖥️  系统信息:")
	fmt.Printf("   • 操作系统: %s\n", getOSName(info.OS))
	fmt.Printf("   • 架构: %s\n", info.Arch)

	if info.HasGPU {
		fmt.Printf("   • GPU加速: ✅ %s\n", info.GPUType)
	} else {
		fmt.Printf("   • GPU加速: ❌ 将使用CPU模式\n")
	}
}

// getOSName 获取友好的操作系统名称
func getOSName(os string) string {
	switch os {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return strings.Title(os)
	}
}

// getGPUStatus 获取GPU状态描述
func getGPUStatus(info *SystemInfo) string {
	if info.HasGPU {
		return fmt.Sprintf("启用 (%s)", info.GPUType)
	}
	return "CPU模式"
}

// findModelsDir 智能查找 models 目录
func (w *WhisperSetup) findModelsDir() string {
	// 1. 先检查当前目录
	if _, err := os.Stat("./models"); err == nil {
		return "./models"
	}

	// 2. 检查可执行文件所在目录
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		modelsInExecDir := filepath.Join(execDir, "models")
		if _, err := os.Stat(modelsInExecDir); err == nil {
			return modelsInExecDir
		}
	}

	// 3. 默认返回当前目录下的 models
	return "./models"
}

// checkPrebuiltModels 检查预制模型
func (w *WhisperSetup) checkPrebuiltModels() error {
	fmt.Println("\n1️⃣  检查预制模型...")

	// 智能查找 models 目录
	modelsDir := w.findModelsDir()
	prebuiltModels := []string{"ggml-base.bin"}            // 只检查 base 模型
	coreMLModels := []string{"ggml-base-encoder.mlmodelc"} // 修正 Core ML 模型名称

	w.PrebuiltModels = []string{}

	// 检查基础模型
	for _, model := range prebuiltModels {
		modelPath := filepath.Join(modelsDir, model)
		if _, err := os.Stat(modelPath); err == nil {
			w.PrebuiltModels = append(w.PrebuiltModels, modelPath)
			fmt.Printf("✅ 找到预制模型: %s\n", model)
		}
	}

	// 检查 Core ML 模型（仅在 macOS 上显示）
	if runtime.GOOS == "darwin" {
		coreMLFound := 0
		for _, model := range coreMLModels {
			modelPath := filepath.Join(modelsDir, model)
			if _, err := os.Stat(modelPath); err == nil {
				coreMLFound++
				fmt.Printf("🚀 找到 Core ML 加速模型: %s\n", model)
			}
		}

		if coreMLFound > 0 {
			fmt.Printf("⚡ Core ML 加速可用，将获得更好的性能\n")
		} else if len(w.PrebuiltModels) > 0 {
			fmt.Printf("💡 提示：下载 Core ML 模型可获得更好的 macOS 性能\n")
		}
	}

	if len(w.PrebuiltModels) == 0 {
		fmt.Printf("❌ 未找到预制模型 (检查目录: %s)\n", modelsDir)
		fmt.Println()
		fmt.Println("📥 手动下载模型文件指南：")
		fmt.Println("====================")
		fmt.Printf("请将以下模型文件下载到 %s 目录：\n", modelsDir)
		fmt.Println()

		// 基础模型
		fmt.Println("🔹 基础模型 (必需):")
		fmt.Println("   文件名: ggml-base.bin")
		fmt.Println("   大小: ~142MB")
		fmt.Println("   下载地址: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin")
		fmt.Println("   直接下载: curl -L -o ggml-base.bin 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin?download=true'")

		// macOS Core ML 模型
		if runtime.GOOS == "darwin" {
			fmt.Println()
			fmt.Println("🚀 Core ML 加速模型 (macOS 推荐):")
			fmt.Println("   文件名: ggml-base-encoder.mlmodelc (解压后的文件夹)")
			fmt.Println("   大小: ~6MB (压缩包)")
			fmt.Println("   下载地址: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-encoder.mlmodelc.zip")
			fmt.Println("   直接下载: curl -L -o ggml-base-encoder.mlmodelc.zip 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-encoder.mlmodelc.zip?download=true'")
			fmt.Println("   解压命令: unzip ggml-base-encoder.mlmodelc.zip && rm ggml-base-encoder.mlmodelc.zip")
		}

		fmt.Println()
		fmt.Println("💡 下载完成后请重新运行此工具进行检测")
		fmt.Println("   如果网络较慢，建议使用下载工具或分段下载")
	} else {
		fmt.Printf("✅ 找到 %d 个预制模型 (目录: %s)\n", len(w.PrebuiltModels), modelsDir)
	}

	return nil
}

// checkExistingInstallation 检查现有安装
func (w *WhisperSetup) checkExistingInstallation() error {
	fmt.Println("\n2️⃣  检查现有 Whisper.cpp 安装...")

	// 常见的安装位置
	possiblePaths := []string{
		"/usr/local/bin/whisper-cli",
		"/opt/homebrew/bin/whisper-cli",
		filepath.Join(os.Getenv("HOME"), "whisper.cpp/build/bin/whisper-cli"),
		filepath.Join(os.Getenv("HOME"), "Documents/whisper.cpp/build/bin/whisper-cli"),
		"./whisper.cpp/build/bin/whisper-cli",
	}

	// Windows下的可执行文件扩展名
	if runtime.GOOS == "windows" {
		for i, path := range possiblePaths {
			if !strings.HasSuffix(path, ".exe") {
				possiblePaths[i] = path + ".exe"
			}
		}
	}

	// 检查PATH中的whisper-cli
	execName := "whisper-cli"
	if runtime.GOOS == "windows" {
		execName = "whisper-cli.exe"
	}

	if path, err := exec.LookPath(execName); err == nil {
		fmt.Printf("✅ 在PATH中找到 whisper-cli: %s\n", path)
		w.WhisperCppPath = filepath.Dir(filepath.Dir(path)) // 获取whisper.cpp根目录
		w.IsInstalled = true
		return nil
	}

	// 检查常见路径
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("✅ 找到现有安装: %s\n", path)
			w.WhisperCppPath = filepath.Dir(filepath.Dir(filepath.Dir(path))) // 获取whisper.cpp根目录
			w.IsInstalled = true
			return nil
		}
	}

	fmt.Println("❌ 未找到现有的 Whisper.cpp 安装")
	return nil
}

// installWhisperCpp 安装Whisper.cpp
func (w *WhisperSetup) installWhisperCpp(sysInfo *SystemInfo) error {
	fmt.Println("\n3️⃣  安装 Whisper.cpp...")

	// 询问用户安装位置
	fmt.Print("请选择安装位置 (默认: ~/whisper.cpp): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	installPath := filepath.Join(os.Getenv("HOME"), "whisper.cpp")
	if input != "" {
		installPath = input
	}

	// 展开用户路径
	if strings.HasPrefix(installPath, "~") {
		installPath = filepath.Join(os.Getenv("HOME"), installPath[1:])
	}

	fmt.Printf("将安装到: %s\n", installPath)

	// 检查目录是否存在
	if _, err := os.Stat(installPath); err == nil {
		fmt.Print("目录已存在，是否删除并重新安装? (y/N): ")
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "y" {
			if err := os.RemoveAll(installPath); err != nil {
				return errors.Wrap(err, "删除现有目录失败")
			}
		} else {
			fmt.Println("取消安装")
			os.Exit(0)
		}
	}

	// 克隆仓库
	fmt.Println("正在克隆 Whisper.cpp 仓库...")
	cmd := exec.Command("git", "clone", whisperRepo, installPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "克隆仓库失败")
	}

	// 编译
	fmt.Println("正在编译 Whisper.cpp...")
	if err := w.buildWhisperCpp(installPath, sysInfo); err != nil {
		return errors.Wrap(err, "编译失败")
	}

	w.WhisperCppPath = installPath
	w.IsInstalled = true

	fmt.Println("✅ Whisper.cpp 安装完成")
	return nil
}

// buildWhisperCpp 编译Whisper.cpp
func (w *WhisperSetup) buildWhisperCpp(installPath string, sysInfo *SystemInfo) error {
	// 创建build目录
	buildPath := filepath.Join(installPath, "build")
	if err := os.MkdirAll(buildPath, 0755); err != nil {
		return errors.Wrap(err, "创建build目录失败")
	}

	// 构建cmake参数
	var cmakeArgs []string = []string{"-DCMAKE_BUILD_TYPE=Release"}

	// 根据系统和GPU支持添加参数
	switch sysInfo.OS {
	case "darwin":
		if sysInfo.SupportsMetal {
			cmakeArgs = append(cmakeArgs, "-DGGML_METAL=ON")
			fmt.Println("🚀 启用 Metal GPU 加速 (Apple Silicon)")
		}
	case "linux", "windows":
		if sysInfo.SupportsCUDA {
			cmakeArgs = append(cmakeArgs, "-DGGML_CUDA=ON")
			fmt.Println("🚀 启用 CUDA GPU 加速")
		}
	}

	// 运行cmake
	fmt.Println("运行 cmake...")
	cmd := exec.Command("cmake", append(cmakeArgs, "..")...)
	cmd.Dir = buildPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "cmake配置失败")
	}

	// 编译
	fmt.Println("编译中...")
	buildCmd := "make"
	buildArgs := []string{"-j", fmt.Sprintf("%d", runtime.NumCPU())}

	// Windows使用不同的构建命令
	if sysInfo.OS == "windows" {
		buildCmd = "cmake"
		buildArgs = []string{"--build", ".", "--config", "Release"}
	}

	cmd = exec.Command(buildCmd, buildArgs...)
	cmd.Dir = buildPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "编译失败")
	}

	return nil
}

// setupModels 设置模型
func (w *WhisperSetup) setupModels() error {
	fmt.Println("\n4️⃣  设置模型...")

	// 如果有预制模型，优先使用
	if len(w.PrebuiltModels) > 0 {
		// 优先使用base模型
		for _, modelPath := range w.PrebuiltModels {
			if strings.Contains(modelPath, "base") {
				w.ModelPath = modelPath
				fmt.Printf("✅ 使用预制模型: %s\n", filepath.Base(modelPath))
				return nil
			}
		}
		// 如果没有base，使用第一个可用的模型
		w.ModelPath = w.PrebuiltModels[0]
		fmt.Printf("✅ 使用预制模型: %s\n", filepath.Base(w.ModelPath))
		return nil
	}

	// 如果没有预制模型，检查whisper.cpp安装目录中的模型
	if w.WhisperCppPath != "" {
		modelsPath := filepath.Join(w.WhisperCppPath, "models")
		modelFile := fmt.Sprintf("ggml-%s.bin", defaultModel)
		modelPath := filepath.Join(modelsPath, modelFile)

		// 检查模型是否存在
		if _, err := os.Stat(modelPath); err == nil {
			fmt.Printf("✅ 找到现有模型: %s\n", modelPath)
			w.ModelPath = modelPath
			return nil
		}

		// 尝试下载模型
		fmt.Printf("正在下载 %s 模型...\n", defaultModel)
		if err := w.downloadModel(modelsPath, defaultModel); err != nil {
			fmt.Println("\n❌ 自动下载模型失败")
			fmt.Println("📥 请手动下载模型文件：")
			fmt.Println("====================")
			fmt.Printf("请将以下模型文件下载到 %s 目录：\n", modelsPath)
			fmt.Println()

			// 基础模型下载指南
			fmt.Println("🔹 基础模型 (必需):")
			fmt.Printf("   文件名: ggml-%s.bin\n", defaultModel)
			fmt.Println("   大小: ~142MB")
			fmt.Printf("   下载地址: https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-%s.bin\n", defaultModel)
			fmt.Printf("   直接下载: curl -L -o ggml-%s.bin 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-%s.bin?download=true'\n", defaultModel, defaultModel)

			fmt.Println()
			fmt.Println("💡 下载完成后请重新运行此工具")
			fmt.Println("   如果网络较慢，建议使用下载工具或分段下载")

			return fmt.Errorf("需要手动下载模型: %v", err)
		}

		w.ModelPath = modelPath
		fmt.Printf("✅ 模型下载完成: %s\n", modelPath)
		return nil
	}

	return errors.New("无法设置模型：既没有预制模型，也没有安装whisper.cpp")
}

// downloadModel 下载模型
func (w *WhisperSetup) downloadModel(modelsPath, modelName string) error {
	// 使用whisper.cpp提供的下载脚本
	downloadScript := filepath.Join(modelsPath, "download-ggml-model.sh")

	// 检查下载脚本是否存在
	if _, err := os.Stat(downloadScript); err != nil {
		return errors.New("下载脚本不存在，请手动下载模型或使用预制模型")
	}

	// 执行下载脚本
	cmd := exec.Command("bash", downloadScript, modelName)
	cmd.Dir = modelsPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "下载脚本执行失败")
	}

	return nil
}

// updateConfig 更新配置文件
func (w *WhisperSetup) updateConfig() error {
	fmt.Println("\n5️⃣  更新配置文件...")

	configPath := "config.yaml"

	// 读取现有配置
	content, err := os.ReadFile(configPath)
	if err != nil {
		return errors.Wrap(err, "读取配置文件失败")
	}

	configStr := string(content)

	// 更新Whisper配置
	// 启用Whisper
	configStr = strings.Replace(configStr, "enabled: false", "enabled: true", 1)

	// 更新whisper.cpp路径 - 使用相对路径或环境变量
	if w.WhisperCppPath != "" {
		// 尝试使用相对于用户目录的路径
		homeDir := os.Getenv("HOME")
		whisperPath := w.WhisperCppPath

		// 如果路径在用户目录下，使用 ~ 符号
		if homeDir != "" && strings.HasPrefix(w.WhisperCppPath, homeDir) {
			whisperPath = "~" + strings.TrimPrefix(w.WhisperCppPath, homeDir)
		}

		if !strings.Contains(configStr, "whisper_cpp_path:") {
			// 添加whisper_cpp_path配置
			whisperSection := `  whisper:
    enabled: true`
			newWhisperSection := fmt.Sprintf(`  whisper:
    enabled: true
    whisper_cpp_path: "%s"  # Whisper.cpp 安装路径，支持 ~/path 和 ${VAR} 环境变量`, whisperPath)
			configStr = strings.Replace(configStr, whisperSection, newWhisperSection, 1)
		} else {
			// 更新现有路径
			newPath := fmt.Sprintf(`whisper_cpp_path: "%s"  # Whisper.cpp 安装路径，支持 ~/path 和 ${VAR} 环境变量`, whisperPath)

			// 先尝试替换空路径
			if strings.Contains(configStr, `whisper_cpp_path: ""`) {
				configStr = strings.Replace(configStr, `whisper_cpp_path: ""`, newPath, 1)
			} else {
				// 使用更精确的正则表达式替换现有路径，只匹配whisper配置块中的路径
				re := regexp.MustCompile(`(?m)^(\s+)whisper_cpp_path:\s*"[^"]*".*$`)
				configStr = re.ReplaceAllString(configStr, fmt.Sprintf("${1}%s", newPath))
			}
		}
	}

	// 更新模型路径
	if w.ModelPath != "" {
		// 将绝对路径转换为相对路径（如果是预制模型）
		modelPath := w.ModelPath
		if strings.HasPrefix(modelPath, "./models/") {
			// 保持相对路径
		} else if absPath, err := filepath.Abs(modelPath); err == nil {
			// 使用绝对路径
			modelPath = absPath
		}

		oldModelPath := `model_path: "./models/ggml-tiny.bin"`
		newModelPath := fmt.Sprintf(`model_path: "%s"`, modelPath)
		configStr = strings.Replace(configStr, oldModelPath, newModelPath, 1)
	}

	// 写回配置文件
	if err := os.WriteFile(configPath, []byte(configStr), 0644); err != nil {
		return errors.Wrap(err, "写入配置文件失败")
	}

	fmt.Println("✅ 配置文件更新完成")
	return nil
}
