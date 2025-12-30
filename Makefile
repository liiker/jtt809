# Makefile - Go交叉编译管理

.PHONY: all build win win64 win32 linux linux64 linux32 mac clean test deps rabbit help

# 项目配置
PROJECT_NAME := jtt809
MAIN_PACKAGE := ./cmd/server
VERSION := $(shell git describe --tags 2>/dev/null || echo "v0.1.0")
BUILD_TIME := $(shell date +'%Y-%m-%d_%H:%M:%S')
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION := $(shell go version | awk '{print $$3}')

# 构建参数
GO_TEST_FLAGS := -v -race

# 默认目标
all: deps test build

# 下载依赖
deps:
	@echo "下载依赖..."
	go mod download
	go mod tidy

# 测试
test:
	@echo "运行测试..."
	go test $(GO_TEST_FLAGS) ./...

# 编译所有平台
build: win linux mac

# 编译Windows程序
win: win64 win32

# 编译Windows 64位
win64:
	@echo "编译Windows 64位..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build \
		-o "build/windows/$(PROJECT_NAME)-amd64.exe" \
		$(MAIN_PACKAGE)

# 编译Windows 32位
win32:
	@echo "编译Windows 32位..."
	GOOS=windows GOARCH=386 CGO_ENABLED=0 \
		go build  \
		-o "build/windows/$(PROJECT_NAME)-386.exe" \
		$(MAIN_PACKAGE)

# 编译Linux程序
linux: linux64 linux32

# 编译Linux 64位
linux64:
	@echo "编译Linux 64位..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build  \
		-o "build/linux/$(PROJECT_NAME)-linux-amd64" \
		$(MAIN_PACKAGE)

# 编译Linux 32位
linux32:
	@echo "编译Linux 32位..."
	GOOS=linux GOARCH=386 CGO_ENABLED=0 \
		go build  \
		-o "build/linux/$(PROJECT_NAME)-linux-386" \
		$(MAIN_PACKAGE)

# 编译macOS
mac:
	@echo "编译macOS..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
		go build  \
		-o "build/darwin/$(PROJECT_NAME)-darwin-amd64" \
		$(MAIN_PACKAGE)

# 本地开发构建
dev:
	@echo "本地开发构建..."
	go build  \
		-o "$(PROJECT_NAME)" \
		$(MAIN_PACKAGE)
	@echo "构建完成: ./$(PROJECT_NAME)"

# 编译RabbitMQ测试客户端
rabbit:
	@echo "编译RabbitMQ测试客户端..."
	go build -o rabbit ./cmd/rabbit
	@echo "构建完成: ./rabbit"

# 安装到GOPATH/bin
install:
	@echo "安装到GOPATH/bin..."
	go install  $(MAIN_PACKAGE)

# 使用UPX压缩
compress:
	@which upx >/dev/null 2>&1 || (echo "UPX未安装，跳过压缩" && exit 0)
	@echo "压缩可执行文件..."
	upx --best build/windows/*.exe 2>/dev/null || true
	upx --best build/linux/* 2>/dev/null || true
	upx --best build/darwin/* 2>/dev/null || true

# 创建发布包
release: build compress
	@echo "创建发布包..."
	@mkdir -p dist
	
	# Windows包
	@cp -r config/windows/* build/windows/ 2>/dev/null || true
	cd build/windows && \
		zip -r "../../dist/$(PROJECT_NAME)-windows-$(VERSION).zip" ./*
	
	# Linux包
	@cp -r config/linux/* build/linux/ 2>/dev/null || true
	cd build/linux && \
		tar czf "../../dist/$(PROJECT_NAME)-linux-$(VERSION).tar.gz" ./*
	
	# macOS包
	@cp -r config/darwin/* build/darwin/ 2>/dev/null || true
	cd build/darwin && \
		tar czf "../../dist/$(PROJECT_NAME)-darwin-$(VERSION).tar.gz" ./*
	
	@echo "发布包已创建在 dist/ 目录:"
	@ls -lh dist/

# 清理
clean:
	@echo "清理构建文件..."
	rm -rf build dist $(PROJECT_NAME) rabbit
	go clean

# 显示帮助
help:
	@echo "可用命令:"
	@echo "  make all        - 下载依赖、测试并构建所有平台"
	@echo "  make deps       - 下载依赖"
	@echo "  make test       - 运行测试"
	@echo "  make build      - 构建所有平台"
	@echo "  make win        - 构建Windows程序(64+32位)"
	@echo "  make win64      - 构建Windows 64位"
	@echo "  make win32      - 构建Windows 32位"
	@echo "  make linux      - 构建Linux程序"
	@echo "  make mac        - 构建macOS程序"
	@echo "  make dev        - 本地开发构建"
	@echo "  make rabbit     - 编译RabbitMQ测试客户端"
	@echo "  make install    - 安装到GOPATH/bin"
	@echo "  make compress   - 使用UPX压缩"
	@echo "  make release    - 创建发布包"
	@echo "  make clean      - 清理构建文件"
	@echo ""
	@echo "项目信息:"
	@echo "  名称: $(PROJECT_NAME)"
	@echo "  主包: $(MAIN_PACKAGE)"
	@echo "  版本: $(VERSION)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  提交: $(COMMIT_HASH)"
	@echo "  Go版本: $(GO_VERSION)"

# 初始化构建目录
init:
	@mkdir -p build/{windows,linux,darwin}
	@mkdir -p dist
	@mkdir -p config/{windows,linux,darwin}
	@echo "初始化完成"
