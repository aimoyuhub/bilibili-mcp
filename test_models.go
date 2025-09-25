package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	// 检查models目录中的文件
	fmt.Println("🔍 检查 models 目录:")

	modelsDir := "./models"
	files, err := os.ReadDir(modelsDir)
	if err != nil {
		fmt.Printf("读取目录失败: %v\n", err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			fmt.Printf("📁 %s (目录)\n", file.Name())
		} else {
			info, _ := file.Info()
			size := float64(info.Size()) / (1024 * 1024) // MB
			fmt.Printf("📄 %s (%.1f MB)\n", file.Name(), size)
		}
	}

	// 检查Core ML模型
	fmt.Println("\n🚀 检查 Core ML 模型:")
	coreMLModels := []string{
		"ggml-tiny.en-encoder.mlmodelc",
		"ggml-base.en-encoder.mlmodelc",
	}

	for _, model := range coreMLModels {
		path := filepath.Join(modelsDir, model)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("✅ %s\n", model)
		} else {
			fmt.Printf("❌ %s (不存在)\n", model)
		}
	}
}
