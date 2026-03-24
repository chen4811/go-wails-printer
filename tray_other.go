//go:build !windows

package main

// TrayManager 空实现（非 Windows 平台）
type TrayManager struct {
	app *App
}

// NewTrayManager 创建托盘管理器
func NewTrayManager(app *App) *TrayManager {
	return &TrayManager{app: app}
}

// Run 启动托盘
func (t *TrayManager) Run() {
	// 非 Windows 平台不实现托盘
}

// Quit 退出托盘
func (t *TrayManager) Quit() {
	t.app.quitFromTray = true
}

// UpdateStatus 更新状态
func (t *TrayManager) UpdateStatus() {
}
