# 打印服务器

基于 Go + Wails + Vue 构建的跨平台打印服务软件。

## 功能特性

- 🖨️ 支持 PDF、图片、Word、Excel 等多种格式打印
- 🔌 WebSocket + HTTP 双协议支持
- 📡 默认端口 11211，可自定义
- 📋 实时任务状态追踪
- 🖥️ 跨平台支持（Windows、macOS、Linux）

## 技术栈

- **后端**: Go 1.21+ / Wails v2
- **前端**: Vue 3 + Vite + Pinia
- **通信**: WebSocket + HTTP API

## 开发环境

### 前置要求

1. Go 1.21 或更高版本
2. Node.js 18 或更高版本
3. Wails CLI

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 检查环境
wails doctor
```

### 本地开发

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 启动开发模式
wails dev
```

### 构建生产版本

```bash
# 构建当前平台
wails build

# 构建 Mac 版本
make build-mac

# 构建 Windows 版本
make build-win

# 构建 Linux 版本
make build-linux
```

## API 文档

### HTTP API

#### 获取服务状态
```
GET /api/status
```

响应:
```json
{
  "isRunning": true,
  "port": 11211,
  "clients": 2,
  "printers": [...],
  "tasks": [...]
}
```

#### 获取打印机列表
```
GET /api/printers
```

响应:
```json
{
  "success": true,
  "printers": [
    { "name": "HP LaserJet", "isDefault": true, "status": "ready" }
  ]
}
```

#### 打印文件
```
POST /api/print
Content-Type: application/json

{
  "fileType": "pdf",
  "printer": "HP LaserJet",
  "data": "base64编码的文件数据"
}
```

响应:
```json
{
  "success": true,
  "taskId": "abc123",
  "status": "pending"
}
```

#### 获取任务列表
```
GET /api/tasks
```

### WebSocket API

连接地址: `ws://localhost:11211/ws`

#### 打印消息
```json
{
  "type": "print",
  "fileType": "pdf",
  "printer": "HP LaserJet",
  "data": "\"base64编码的数据\""
}
```

#### 查询任务状态
```json
{
  "type": "task",
  "taskId": "abc123"
}
```

## 前端 SDK 使用

```html
<script src="sdk/printer-sdk.js"></script>
<script>
// WebSocket 方式
const printer = new PrinterSDK('ws://localhost:11211/ws')
await printer.connect()

// 打印 PDF
const result = await printer.printPDF(base64Data)

// 打印图片
await printer.printImage(base64Data)

// HTTP API 方式
const http = new PrinterHTTPClient('http://localhost:11211')
const status = await http.getStatus()
</script>
```

## 项目结构

```
printer/
├── main.go          # Wails 主入口
├── app.go           # 应用逻辑
├── socket.go        # WebSocket 服务
├── printer.go       # 打印服务
├── utils.go         # 工具函数
├── go.mod           # Go 模块
├── wails.json       # Wails 配置
├── Makefile         # 构建脚本
├── frontend/        # Vue 前端
│   ├── src/
│   │   ├── App.vue
│   │   ├── main.js
│   │   └── style.css
│   ├── index.html
│   ├── vite.config.js
│   └── package.json
└── sdk/             # 前端 SDK
    └── printer-sdk.js
```

## License

MIT
