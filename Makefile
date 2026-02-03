.PHONY: build clean test run install docker

# 变量定义
APP_NAME=alertengine
VERSION?=1.0.0
BUILD_DIR=build
BINARY=$(BUILD_DIR)/$(APP_NAME)
DOCKER_IMAGE=$(APP_NAME):$(VERSION)

# 构建参数
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# 默认目标
all: clean build

# 构建二进制文件
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BINARY) ./cmd/alertengine

# 清理
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 运行应用
run: build
	@echo "Running $(APP_NAME)..."
	$(BINARY) -config config.example.yml

# 安装到系统
install: build
	@echo "Installing $(APP_NAME)..."
	install -d /usr/local/bin
	install -m 755 $(BINARY) /usr/local/bin/$(APP_NAME)
	install -d /etc/alertengine
	install -m 644 config.example.yml /etc/alertengine/config.yml

# 构建Docker镜像
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# 代码格式化
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
lint:
	@echo "Linting code..."
	golangci-lint run

# 生成依赖
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# 生成文档
docs:
	@echo "Generating documentation..."
	godoc -http=:6060

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  build     - Build the binary"
	@echo "  clean     - Clean build artifacts"
	@echo "  test      - Run tests"
	@echo "  run       - Build and run the application"
	@echo "  install   - Install to /usr/local/bin"
	@echo "  docker    - Build Docker image"
	@echo "  fmt       - Format code"
	@echo "  lint      - Run linter"
	@echo "  deps      - Download dependencies"
	@echo "  docs      - Start documentation server"
	@echo "  help      - Show this help message"
