package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 应用主结构
type App struct {
	ctx         context.Context
	socket      *SocketServer
	printer     *PrinterService
	config      *Config
	configPath  string
	status      *ServiceStatus
	mu          sync.RWMutex
}

// Config 配置结构
type Config struct {
	Port         int    `json:"port"`
	AutoStart    bool   `json:"autoStart"`
	MinimizeTray bool   `json:"minimizeTray"`
	Notifications bool  `json:"notifications"`
	DefaultPrinter string `json:"defaultPrinter"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	IsRunning  bool             `json:"isRunning"`
	Port       int              `json:"port"`
	Clients    int              `json:"clients"`
	Printers   []PrinterInfo    `json:"printers"`
	Tasks      []TaskInfo       `json:"tasks"`
}

// PrinterInfo 打印机信息
type PrinterInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	Status    string `json:"status"`
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Printer   string    `json:"printer"`
	Status    string    `json:"status"` // pending, processing, completed, failed
	Progress  int       `json:"progress"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NewApp 创建新应用
func NewApp() *App {
	return &App{
		status: &ServiceStatus{
			IsRunning: false,
			Port:      11211,
			Tasks:     make([]TaskInfo, 0),
		},
		config: &Config{
			Port:         11211,
			AutoStart:    true,
			MinimizeTray: true,
			Notifications: true,
		},
	}
}

// startup 应用启动
func (a *App) startup(ctx context.Context) {
	fmt.Println("[startup] 应用启动中...")
	a.ctx = ctx
	
	// 初始化配置路径
	configDir := filepath.Join(xdg.ConfigHome, "printer-server")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("[startup] 创建配置目录失败: %v\n", err)
	}
	a.configPath = filepath.Join(configDir, "config.json")
	
	// 加载配置
	a.loadConfig()
	fmt.Printf("[startup] 配置加载完成，端口: %d, 自动启动: %v\n", a.config.Port, a.config.AutoStart)
	
	// 初始化打印服务
	fmt.Println("[startup] 初始化打印服务...")
	a.printer = NewPrinterService()
	printers, err := a.printer.GetPrinters()
	if err != nil {
		fmt.Printf("[startup] 获取打印机列表失败: %v\n", err)
	} else {
		a.status.Printers = printers
		fmt.Printf("[startup] 已加载 %d 台打印机\n", len(printers))
	}
	
	// 初始化 Socket 服务
	fmt.Println("[startup] 初始化 Socket 服务...")
	a.socket = NewSocketServer(a.config.Port, a)
	
	// 自动启动 - 延迟启动确保UI已加载
	if a.config.AutoStart {
		go func() {
			time.Sleep(500 * time.Millisecond)
			fmt.Println("[startup] 自动启动服务...")
			if err := a.StartServer(); err != nil {
				fmt.Printf("[startup] 自动启动服务失败: %v\n", err)
			}
		}()
	}
	
	fmt.Println("[startup] 应用启动完成!")
}

// shutdown 应用关闭
func (a *App) shutdown(ctx context.Context) {
	if a.socket != nil && a.socket.IsRunning() {
		a.socket.Stop()
	}
	a.saveConfig()
}

// loadConfig 加载配置
func (a *App) loadConfig() {
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return
	}
	
	// 简单的 JSON 解析
	config := &Config{}
	if err := parseJSON(data, config); err == nil {
		a.config = config
		a.status.Port = config.Port
	}
}

// saveConfig 保存配置
func (a *App) saveConfig() {
	data := formatJSON(a.config)
	os.WriteFile(a.configPath, data, 0644)
}

// StartServer 启动服务
func (a *App) StartServer() error {
	fmt.Println("[StartServer] 开始启动服务...")
	
	// 不使用锁，避免阻塞
	if a.socket == nil {
		err := fmt.Errorf("socket 服务未初始化")
		fmt.Printf("[StartServer] 错误: %v\n", err)
		return err
	}
	
	if a.socket.IsRunning() {
		err := fmt.Errorf("服务已在运行")
		fmt.Printf("[StartServer] 错误: %v\n", err)
		return err
	}
	
	fmt.Printf("[StartServer] 正在启动 Socket 服务器，端口: %d\n", a.config.Port)
	if err := a.socket.Start(); err != nil {
		fmt.Printf("[StartServer] 启动失败: %v\n", err)
		return err
	}
	
	a.mu.Lock()
	a.status.IsRunning = true
	a.status.Port = a.config.Port
	a.mu.Unlock()
	
	fmt.Println("[StartServer] 服务启动成功!")
	a.emitStatusChange()
	
	return nil
}

// StopServer 停止服务
func (a *App) StopServer() error {
	if a.socket == nil || !a.socket.IsRunning() {
		return fmt.Errorf("服务未运行")
	}
	
	a.socket.Stop()
	
	a.mu.Lock()
	a.status.IsRunning = false
	a.mu.Unlock()
	
	a.emitStatusChange()
	
	return nil
}

// GetStatus 获取服务状态
func (a *App) GetStatus() ServiceStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	status := *a.status
	status.Clients = a.socket.GetClientCount()
	return status
}

// GetPrinters 获取打印机列表
func (a *App) GetPrinters() []PrinterInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status.Printers
}

// GetConfig 获取配置
func (a *App) GetConfig() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return *a.config
}

// SaveConfig 保存配置
func (a *App) SaveConfig(config Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.config = &config
	a.status.Port = config.Port
	
	// 更新 socket 端口
	if a.socket != nil {
		a.socket.SetPort(config.Port)
	}
	
	a.saveConfig()
	return nil
}

// GetTasks 获取任务列表
func (a *App) GetTasks() []TaskInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status.Tasks
}

// ClearTasks 清空任务列表
func (a *App) ClearTasks() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.Tasks = make([]TaskInfo, 0)
}

// AddTask 添加任务
func (a *App) AddTask(task TaskInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// 限制任务列表大小
	if len(a.status.Tasks) >= 100 {
		a.status.Tasks = a.status.Tasks[:99]
	}
	a.status.Tasks = append([]TaskInfo{task}, a.status.Tasks...)
}

// UpdateTask 更新任务状态
func (a *App) UpdateTask(taskID string, status string, progress int, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	for i, task := range a.status.Tasks {
		if task.ID == taskID {
			a.status.Tasks[i].Status = status
			a.status.Tasks[i].Progress = progress
			a.status.Tasks[i].Error = errMsg
			a.status.Tasks[i].UpdatedAt = time.Now()
			break
		}
	}
}

// ProcessPrintTask 处理打印任务
func (a *App) ProcessPrintTask(taskID string, data []byte, fileType string, printerName string) {
	// 更新任务状态为处理中
	a.UpdateTask(taskID, "processing", 10, "")
	
	// 选择打印机
	if printerName == "" {
		printerName = a.config.DefaultPrinter
	}
	if printerName == "" && len(a.status.Printers) > 0 {
		for _, p := range a.status.Printers {
			if p.IsDefault {
				printerName = p.Name
				break
			}
		}
	}
	if printerName == "" && len(a.status.Printers) > 0 {
		printerName = a.status.Printers[0].Name
	}
	
	// 执行打印
	err := a.printer.Print(data, fileType, printerName)
	
	if err != nil {
		a.UpdateTask(taskID, "failed", 0, err.Error())
		a.emitTaskError(taskID, err.Error())
	} else {
		a.UpdateTask(taskID, "completed", 100, "")
		a.emitTaskComplete(taskID)
	}
}

// emitStatusChange 发送状态变更事件
func (a *App) emitStatusChange() {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "status-change", a.GetStatus())
	}
}

// emitTaskError 发送任务错误事件
func (a *App) emitTaskError(taskID string, errMsg string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "task-error", map[string]string{
			"taskId": taskID,
			"error":  errMsg,
		})
	}
}

// emitTaskComplete 发送任务完成事件
func (a *App) emitTaskComplete(taskID string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "task-complete", map[string]string{
			"taskId": taskID,
		})
	}
}
