package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
	ReadBufferSize:  10 * 1024 * 1024, // 10MB
	WriteBufferSize: 1024,
}

// SocketServer WebSocket 服务器
type SocketServer struct {
	port     int
	server   *http.Server
	clients  map[string]*websocket.Conn
	mu       sync.RWMutex
	app      *App
	running  bool
}

// SocketMessage Socket 消息结构
type SocketMessage struct {
	Type      string          `json:"type"`      // print, print-url, status, printers, task
	TaskID    string          `json:"taskId"`    
	FileType  string          `json:"fileType"`  // pdf, image, word, excel
	Printer   string          `json:"printer"`
	Data      json.RawMessage `json:"data"`      // base64 编码的文件数据
	URL       string          `json:"url"`       // 远程文件URL
}

// SocketResponse Socket 响应结构
type SocketResponse struct {
	Type      string      `json:"type"`
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	TaskID    string      `json:"taskId,omitempty"`
	Status    string      `json:"status,omitempty"`
	Progress  int         `json:"progress,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// NewSocketServer 创建新的 Socket 服务器
func NewSocketServer(port int, app *App) *SocketServer {
	return &SocketServer{
		port:    port,
		clients: make(map[string]*websocket.Conn),
		app:     app,
	}
}

// Start 启动服务器
func (s *SocketServer) Start() error {
	s.mu.Lock()
	
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("服务器已在运行")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/printers", s.handlePrinters)
	mux.HandleFunc("/api/print", s.handlePrintHTTP)
	mux.HandleFunc("/api/tasks", s.handleTasks)

	addr := fmt.Sprintf(":%d", s.port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动服务器
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("启动服务器失败 (端口 %d 被占用?): %v", s.port, err)
	}

	s.running = true
	s.mu.Unlock()
	
	log.Printf("WebSocket 服务器启动在端口 %d", s.port)

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("服务器错误: %v", err)
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *SocketServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return
	}

	// 关闭所有客户端连接
	for _, conn := range s.clients {
		conn.Close()
	}
	s.clients = make(map[string]*websocket.Conn)

	// 关闭服务器
	if s.server != nil {
		s.server.Close()
	}
	
	s.running = false
	log.Println("WebSocket 服务器已停止")
}

// IsRunning 检查服务器是否运行
func (s *SocketServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetClientCount 获取客户端数量
func (s *SocketServer) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// SetPort 设置端口
func (s *SocketServer) SetPort(port int) {
	s.port = port
}

// handleWebSocket 处理 WebSocket 连接
func (s *SocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 生成客户端 ID
	clientID := uuid.New().String()[:8]
	s.mu.Lock()
	s.clients[clientID] = conn
	s.mu.Unlock()

	log.Printf("客户端 %s 已连接", clientID)

	// 发送欢迎消息
	s.sendResponse(conn, SocketResponse{
		Type:    "connected",
		Success: true,
		Message: "已连接到打印服务器",
		Data: map[string]interface{}{
			"clientId": clientID,
			"printers": s.app.GetPrinters(),
		},
	})

	// 监听消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("客户端 %s 断开连接: %v", clientID, err)
			break
		}

		var msg SocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			s.sendResponse(conn, SocketResponse{
				Type:    "error",
				Success: false,
				Message: "无效的消息格式",
			})
			continue
		}

		s.handleMessage(conn, clientID, &msg)
	}

	// 移除客户端
	s.mu.Lock()
	delete(s.clients, clientID)
	s.mu.Unlock()
}

// handleMessage 处理 WebSocket 消息
func (s *SocketServer) handleMessage(conn *websocket.Conn, clientID string, msg *SocketMessage) {
	switch msg.Type {
	case "print":
		s.handlePrint(conn, clientID, msg)
	case "print-url":
		s.handlePrintURL(conn, clientID, msg)
	case "status":
		s.sendResponse(conn, SocketResponse{
			Type:    "status",
			Success: true,
			Data:    s.app.GetStatus(),
		})
	case "printers":
		s.sendResponse(conn, SocketResponse{
			Type:    "printers",
			Success: true,
			Data:    s.app.GetPrinters(),
		})
	case "task":
		s.handleTaskStatus(conn, msg)
	default:
		s.sendResponse(conn, SocketResponse{
			Type:    "error",
			Success: false,
			Message: fmt.Sprintf("未知的消息类型: %s", msg.Type),
		})
	}
}

// handlePrint 处理打印请求
func (s *SocketServer) handlePrint(conn *websocket.Conn, clientID string, msg *SocketMessage) {
	taskID := msg.TaskID
	if taskID == "" {
		taskID = uuid.New().String()[:8]
	}

	// 解码 Base64 数据
	var base64Data string
	if err := json.Unmarshal(msg.Data, &base64Data); err != nil {
		s.sendResponse(conn, SocketResponse{
			Type:    "error",
			TaskID:  taskID,
			Success: false,
			Message: "无效的数据格式",
		})
		return
	}

	// 解码 Base64
	data, err := decodeBase64(base64Data)
	if err != nil {
		s.sendResponse(conn, SocketResponse{
			Type:    "error",
			TaskID:  taskID,
			Success: false,
			Message: fmt.Sprintf("数据解码失败: %v", err),
		})
		return
	}

	// 创建任务
	task := TaskInfo{
		ID:        taskID,
		Type:      msg.FileType,
		Printer:   msg.Printer,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.app.AddTask(task)

	// 发送任务已接收响应
	s.sendResponse(conn, SocketResponse{
		Type:    "task",
		TaskID:  taskID,
		Success: true,
		Status:  "pending",
		Message: "打印任务已接收",
	})

	// 异步处理打印任务
	go func() {
		s.app.ProcessPrintTask(taskID, data, msg.FileType, msg.Printer)
	}()
}

// handleTaskStatus 处理任务状态查询
func (s *SocketServer) handleTaskStatus(conn *websocket.Conn, msg *SocketMessage) {
	tasks := s.app.GetTasks()
	
	if msg.TaskID != "" {
		// 查找特定任务
		for _, task := range tasks {
			if task.ID == msg.TaskID {
				s.sendResponse(conn, SocketResponse{
					Type:     "task",
					TaskID:   task.ID,
					Success:  true,
					Status:   task.Status,
					Progress: task.Progress,
					Error:    task.Error,
				})
				return
			}
		}
		s.sendResponse(conn, SocketResponse{
			Type:    "error",
			TaskID:  msg.TaskID,
			Success: false,
			Message: "任务不存在",
		})
		return
	}

	// 返回所有任务
	s.sendResponse(conn, SocketResponse{
		Type:    "tasks",
		Success: true,
		Data:    tasks,
	})
}

// handlePrintURL 处理远程URL打印请求
func (s *SocketServer) handlePrintURL(conn *websocket.Conn, clientID string, msg *SocketMessage) {
	taskID := msg.TaskID
	if taskID == "" {
		taskID = uuid.New().String()[:8]
	}

	if msg.URL == "" {
		s.sendResponse(conn, SocketResponse{
			Type:    "error",
			TaskID:  taskID,
			Success: false,
			Message: "URL 不能为空",
		})
		return
	}

	// 创建任务
	task := TaskInfo{
		ID:        taskID,
		Type:      msg.FileType,
		Printer:   msg.Printer,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.app.AddTask(task)

	// 发送任务已接收响应
	s.sendResponse(conn, SocketResponse{
		Type:    "task",
		TaskID:  taskID,
		Success: true,
		Status:  "pending",
		Message: "打印任务已接收，正在下载文件...",
	})

	// 异步处理打印任务
	go func() {
		s.app.ProcessPrintURLTask(taskID, msg.URL, msg.FileType, msg.Printer)
	}()
}

// handleStatus HTTP 状态接口
func (s *SocketServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(s.app.GetStatus())
}

// handlePrinters HTTP 打印机列表接口
func (s *SocketServer) handlePrinters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"printers": s.app.GetPrinters(),
	})
}

// handlePrintHTTP HTTP 打印接口
func (s *SocketServer) handlePrintHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "只支持 POST 方法",
		})
		return
	}

	var req struct {
		TaskID   string `json:"taskId"`
		FileType string `json:"fileType"`
		Printer  string `json:"printer"`
		Data     string `json:"data"` // base64
		URL      string `json:"url"`  // 远程文件URL
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "无效的请求体",
		})
		return
	}

	taskID := req.TaskID
	if taskID == "" {
		taskID = uuid.New().String()[:8]
	}

	// 如果提供了URL，使用URL打印
	if req.URL != "" {
		task := TaskInfo{
			ID:        taskID,
			Type:      req.FileType,
			Printer:   req.Printer,
			Status:    "pending",
			Progress:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		s.app.AddTask(task)

		go func() {
			s.app.ProcessPrintURLTask(taskID, req.URL, req.FileType, req.Printer)
		}()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"taskId":  taskID,
			"status":  "pending",
			"message": "打印任务已接收，正在下载文件...",
		})
		return
	}

	// 否则使用Base64数据打印
	data, err := decodeBase64(req.Data)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"taskId":  taskID,
			"error":   fmt.Sprintf("数据解码失败: %v", err),
		})
		return
	}

	task := TaskInfo{
		ID:        taskID,
		Type:      req.FileType,
		Printer:   req.Printer,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.app.AddTask(task)

	go func() {
		s.app.ProcessPrintTask(taskID, data, req.FileType, req.Printer)
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"taskId":  taskID,
		"status":  "pending",
		"message": "打印任务已接收",
	})
}

// handleTasks HTTP 任务列表接口
func (s *SocketServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"tasks":   s.app.GetTasks(),
	})
}

// sendResponse 发送 WebSocket 响应
func (s *SocketServer) sendResponse(conn *websocket.Conn, resp SocketResponse) {
	conn.WriteJSON(resp)
}
