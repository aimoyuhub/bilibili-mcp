package embedded

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/shirenchuang/bilibili-mcp/pkg/logger"
)

// 嵌入模型文件（构建时从 models/ 复制到此目录）
//
//go:embed models/ggml-base.bin
var baseModelData []byte

//go:embed models/ggml-base-encoder.mlmodelc.tar.gz
var coreMLModelData []byte

// ModelManager 嵌入模型管理器
type ModelManager struct {
	tempDir string
}

// NewModelManager 创建模型管理器
func NewModelManager() *ModelManager {
	return &ModelManager{}
}

// EnsureModelsExtracted 确保模型已提取到临时目录
func (m *ModelManager) EnsureModelsExtracted() (string, error) {
	if m.tempDir != "" {
		return m.tempDir, nil
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "bilibili-mcp-models-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	logger.Infof("📦 提取嵌入的模型文件到: %s", tempDir)

	// 提取基础模型
	baseModelPath := filepath.Join(tempDir, "ggml-base.bin")
	if err := m.extractFile(baseModelData, baseModelPath); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("提取基础模型失败: %w", err)
	}

	logger.Infof("✅ 基础模型已提取: %s (%.1f MB)", baseModelPath, float64(len(baseModelData))/1024/1024)

	// 在 macOS 上提取 Core ML 模型
	if runtime.GOOS == "darwin" && len(coreMLModelData) > 0 {
		if err := m.extractCoreMLModel(tempDir); err != nil {
			logger.Warnf("⚠️  Core ML 模型提取失败: %v", err)
		} else {
			logger.Infof("✅ Core ML 模型已提取并解压")
		}
	}

	m.tempDir = tempDir
	return tempDir, nil
}

// GetBaseModelPath 获取基础模型路径
func (m *ModelManager) GetBaseModelPath() (string, error) {
	tempDir, err := m.EnsureModelsExtracted()
	if err != nil {
		return "", err
	}
	return filepath.Join(tempDir, "ggml-base.bin"), nil
}

// GetCoreMLModelPath 获取 Core ML 模型路径 (仅 macOS)
func (m *ModelManager) GetCoreMLModelPath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("Core ML 模型仅在 macOS 上可用")
	}

	tempDir, err := m.EnsureModelsExtracted()
	if err != nil {
		return "", err
	}

	coreMLPath := filepath.Join(tempDir, "ggml-base-encoder.mlmodelc")
	if _, err := os.Stat(coreMLPath); err != nil {
		return "", fmt.Errorf("Core ML 模型不存在")
	}

	return coreMLPath, nil
}

// HasCoreMLModel 检查是否有 Core ML 模型
func (m *ModelManager) HasCoreMLModel() bool {
	return runtime.GOOS == "darwin" && len(coreMLModelData) > 0
}

// extractFile 提取文件到指定路径
func (m *ModelManager) extractFile(data []byte, targetPath string) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// extractCoreMLModel 提取并解压 Core ML 模型
func (m *ModelManager) extractCoreMLModel(tempDir string) error {
	// 先提取 tar.gz 文件
	tarPath := filepath.Join(tempDir, "ggml-base-encoder.mlmodelc.tar.gz")
	if err := m.extractFile(coreMLModelData, tarPath); err != nil {
		return fmt.Errorf("提取 tar.gz 文件失败: %w", err)
	}

	// 解压 tar.gz
	if err := m.extractTarGz(tarPath, tempDir); err != nil {
		return fmt.Errorf("解压 tar.gz 失败: %w", err)
	}

	// 删除临时 tar.gz 文件
	os.Remove(tarPath)

	return nil
}

// extractTarGz 解压 tar.gz 文件
func (m *ModelManager) extractTarGz(tarPath, destDir string) error {
	// 使用系统命令解压（简单可靠）
	cmd := fmt.Sprintf("cd %s && tar -xzf %s", destDir, filepath.Base(tarPath))

	// 执行解压命令
	if err := executeCommand(cmd); err != nil {
		return fmt.Errorf("解压命令执行失败: %w", err)
	}

	return nil
}

// executeCommand 执行系统命令
func executeCommand(cmd string) error {
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	} else {
		shell = "/bin/sh"
		flag = "-c"
	}

	process := exec.Command(shell, flag, cmd)
	return process.Run()
}

// Cleanup 清理临时文件
func (m *ModelManager) Cleanup() {
	if m.tempDir != "" {
		os.RemoveAll(m.tempDir)
		m.tempDir = ""
	}
}
