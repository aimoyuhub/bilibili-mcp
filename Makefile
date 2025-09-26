# bilibili-mcp Makefile

# 变量定义
APP_NAME=bilibili-mcp
LOGIN_NAME=bilibili-login
WHISPER_INIT_NAME=whisper-init
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(shell go version | awk '{print $$3}')

# Go 编译参数
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)"

# 默认目标
.PHONY: all
all: build

# 构建
.PHONY: build
build: build-server build-login build-whisper-init

.PHONY: build-server
build-server:
	@echo "构建 MCP 服务器..."
	go build $(LDFLAGS) -o $(APP_NAME) ./cmd/server

.PHONY: build-login
build-login:
	@echo "构建登录工具..."
	go build $(LDFLAGS) -o $(LOGIN_NAME) ./cmd/login

.PHONY: build-whisper-init
build-whisper-init:
	@echo "构建 Whisper 初始化工具..."
	go build $(LDFLAGS) -o $(WHISPER_INIT_NAME) ./cmd/whisper-init

# 跨平台构建
.PHONY: build-all
build-all: clean
	@echo "开始跨平台构建..."
	
	# macOS Apple Silicon
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-arm64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(LOGIN_NAME)-darwin-arm64 ./cmd/login
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(WHISPER_INIT_NAME)-darwin-arm64 ./cmd/whisper-init
	
	# macOS Intel
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(LOGIN_NAME)-darwin-amd64 ./cmd/login
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(WHISPER_INIT_NAME)-darwin-amd64 ./cmd/whisper-init
	
	# Windows x64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/server
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(LOGIN_NAME)-windows-amd64.exe ./cmd/login
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(WHISPER_INIT_NAME)-windows-amd64.exe ./cmd/whisper-init
	
	# Linux x64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(LOGIN_NAME)-linux-amd64 ./cmd/login
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(WHISPER_INIT_NAME)-linux-amd64 ./cmd/whisper-init
	
	@echo "跨平台构建完成！"
	@ls -la dist/

# 检查模型文件
.PHONY: prepare-models
prepare-models:
	@echo "检查模型文件..."
	@if [ ! -f "models/ggml-base.bin" ]; then \
		echo "❌ 未找到 ggml-base.bin 模型文件"; \
		echo "💡 请运行以下命令下载模型:"; \
		echo "   ./scripts/download-whisper-models.sh"; \
		echo "   或者: make download-models"; \
		exit 1; \
	fi
	@echo "✅ 基础模型文件检查完成"
	@if [ -d "models/ggml-base-encoder.mlmodelc" ]; then \
		echo "✅ 找到 Core ML 加速模型"; \
	else \
		echo "⚠️  未找到 Core ML 模型，macOS 版本将不包含 Core ML 加速"; \
	fi

# 下载模型文件
.PHONY: download-models
download-models:
	@echo "下载 Whisper 模型文件..."
	./scripts/download-whisper-models.sh

# 安装依赖
.PHONY: deps
deps:
	@echo "安装依赖..."
	go mod download
	go mod tidy

# 安装 Playwright
.PHONY: install-playwright
install-playwright:
	@echo "安装 Playwright..."
	go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium

# 运行测试
.PHONY: test
test:
	@echo "运行测试..."
	go test -v ./...

# 代码格式化
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 代码检查
.PHONY: lint
lint:
	@echo "代码检查..."
	golangci-lint run

# 清理
.PHONY: clean
clean:
	@echo "清理构建文件..."
	rm -f $(APP_NAME) $(LOGIN_NAME) $(WHISPER_INIT_NAME)
	rm -rf dist/
	rm -rf logs/
	mkdir -p dist

# 创建发布包
.PHONY: release
release: build-all prepare-models
	@echo "创建分平台发布包..."
	
	# macOS Apple Silicon - 仅可执行文件
	@echo "📦 打包 macOS Apple Silicon..."
	cd dist && \
	mkdir -p darwin-arm64-package && \
	cp $(APP_NAME)-darwin-arm64 $(LOGIN_NAME)-darwin-arm64 darwin-arm64-package/ && \
	tar -czf $(APP_NAME)-v$(VERSION)-darwin-arm64.tar.gz -C darwin-arm64-package . && \
	rm -rf darwin-arm64-package
	
	# macOS Intel - 仅可执行文件
	@echo "📦 打包 macOS Intel..."
	cd dist && \
	mkdir -p darwin-amd64-package && \
	cp $(APP_NAME)-darwin-amd64 $(LOGIN_NAME)-darwin-amd64 darwin-amd64-package/ && \
	tar -czf $(APP_NAME)-v$(VERSION)-darwin-amd64.tar.gz -C darwin-amd64-package . && \
	rm -rf darwin-amd64-package
	
	# Windows - 仅可执行文件
	@echo "📦 打包 Windows..."
	cd dist && \
	mkdir -p windows-amd64-package && \
	cp $(APP_NAME)-windows-amd64.exe $(LOGIN_NAME)-windows-amd64.exe windows-amd64-package/ && \
	zip -r -q $(APP_NAME)-v$(VERSION)-windows-amd64.zip windows-amd64-package && \
	rm -rf windows-amd64-package
	
	# Linux - 仅可执行文件
	@echo "📦 打包 Linux..."
	cd dist && \
	mkdir -p linux-amd64-package && \
	cp $(APP_NAME)-linux-amd64 $(LOGIN_NAME)-linux-amd64 linux-amd64-package/ && \
	tar -czf $(APP_NAME)-v$(VERSION)-linux-amd64.tar.gz -C linux-amd64-package . && \
	rm -rf linux-amd64-package
	
	@echo "✅ 发布包创建完成！"
	@echo ""
	@echo "📋 发布包说明:"
	@echo "   所有平台: 轻量化可执行文件 (~10MB)"
	@echo "   首次使用需要下载模型文件 (./whisper-init)"
	@echo ""
	@ls -la dist/*.tar.gz dist/*.zip

# 运行服务器
.PHONY: run
run: build-server
	./$(APP_NAME)

# 运行登录工具
.PHONY: login
login: build-login
	./$(LOGIN_NAME)

# 开发模式运行
.PHONY: dev
dev:
	go run ./cmd/server -config config.yaml

# 初始化项目
.PHONY: init
init: deps install-playwright
	@echo "创建必要的目录..."
	mkdir -p logs cookies
	@echo "项目初始化完成！"

# Docker 相关
.PHONY: docker-build
docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker build -t $(APP_NAME):latest .

.PHONY: docker-run
docker-run:
	@echo "运行 Docker 容器..."
	docker run -p 18666:18666 -v $(PWD)/cookies:/app/cookies $(APP_NAME):latest

# 帮助信息
.PHONY: help
help:
	@echo "bilibili-mcp 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  build         构建所有二进制文件"
	@echo "  build-server  构建 MCP 服务器"
	@echo "  build-login   构建登录工具"
	@echo "  build-whisper-init 构建 Whisper 初始化工具 (whisper-init)"
	@echo "  build-all     跨平台构建"
	@echo "  release       创建分平台发布包 (macOS含Core ML)"
	@echo "  download-models 下载 Whisper 模型文件"
	@echo "  prepare-models 检查模型文件"
	@echo "  deps          安装依赖"
	@echo "  install-playwright 安装 Playwright"
	@echo "  test          运行测试"
	@echo "  fmt           格式化代码"
	@echo "  lint          代码检查"
	@echo "  clean         清理构建文件"
	@echo "  run           运行服务器"
	@echo "  login         运行登录工具"
	@echo "  dev           开发模式运行"
	@echo "  init          初始化项目"
	@echo "  docker-build  构建 Docker 镜像"
	@echo "  docker-run    运行 Docker 容器"
	@echo "  help          显示帮助信息"
