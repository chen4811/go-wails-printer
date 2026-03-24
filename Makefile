.PHONY: dev build build-all clean install copy-deps

# 复制依赖文件到 build 目录
copy-deps:
	@echo "复制 PDFtoPrinter.exe 到 build 目录..."
	@if exist PDFtoPrinter.exe (
		if not exist build\bin mkdir build\bin
		copy /Y PDFtoPrinter.exe build\bin\ >nul
		echo PDFtoPrinter.exe 已复制到 build\bin\
	) else (
		echo 警告: 未找到 PDFtoPrinter.exe
	)

# 开发模式
dev:
	@echo "检查依赖文件..."
	@if exist PDFtoPrinter.exe (
		echo PDFtoPrinter.exe 存在
	) else (
		echo 警告: 未找到 PDFtoPrinter.exe，PDF 打印功能将不可用
	)
	wails dev

# 构建当前平台
build:
	wails build
	@echo "复制依赖文件..."
	@if exist PDFtoPrinter.exe (
		if not exist build\bin mkdir build\bin
		copy /Y PDFtoPrinter.exe build\bin\ >nul
		echo PDFtoPrinter.exe 已复制到 build\bin\
	)

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
