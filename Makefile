.PHONY: dev build build-all clean install

# 开发模式
dev:
	wails dev

# 构建当前平台
build:
	wails build

# 构建所有平台
build-all:
	wails build -platform=darwin/amd64
	wails build -platform=darwin/arm64
	wails build -platform=windows/amd64
	wails build -platform=linux/amd64

# 构建Mac版本
build-mac:
	wails build -platform=darwin/universal

# 构建Windows版本
build-win:
	wails build -platform=windows/amd64

# 构建Linux版本
build-linux:
	wails build -platform=linux/amd64

# 安装依赖
install:
	cd frontend && npm install

# 清理
clean:
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf frontend/node_modules

# 格式化Go代码
fmt:
	go fmt ./...

# 运行测试
test:
	go test -v ./...
