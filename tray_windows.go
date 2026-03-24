//go:build windows

package main

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	WM_USER          = 0x0400
	WM_TRAYMSG       = WM_USER + 1
	WM_COMMAND       = 0x0111
	WM_DESTROY       = 0x0002
	WM_QUIT          = 0x0012
	WM_RBUTTONUP     = 0x0205
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203

	NIM_ADD    = 0x00000000
	NIM_MODIFY = 0x00000001
	NIM_DELETE = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	IDM_SHOW = 1000
	IDM_QUIT = 1001

	MF_SEPARATOR = 0x00000800

	IMAGE_ICON      = 1
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040

	CW_USEDEFAULT = ^0x7fffffff

	WS_OVERLAPPED = 0x00000000
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procLoadImageW          = user32.NewProc("LoadImageW")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
)

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type POINT struct {
	X, Y int32
}

// TrayManager Windows 托盘管理器
type TrayManager struct {
	app        *App
	hwnd       uintptr
	menu       uintptr
	notifyData NOTIFYICONDATA
	running    bool
	mu         sync.Mutex
}

// NewTrayManager 创建托盘管理器
func NewTrayManager(app *App) *TrayManager {
	return &TrayManager{app: app}
}

// Run 启动托盘（独立隐藏窗口 + 消息循环）
func (t *TrayManager) Run() {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.running = true
	t.mu.Unlock()

	// 在 Windows 主线程中运行
	t.runOnMainThread()
}

// runOnMainThread 在主线程运行托盘
func (t *TrayManager) runOnMainThread() {
	// 获取模块句柄
	hInstance, _, _ := procGetModuleHandleW.Call(0)

	// 注册窗口类
	className, _ := windows.UTF16PtrFromString("PrinterServerTrayClass")

	wndClass := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   syscall.NewCallback(t.wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
	}

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wndClass)))

	// 创建隐藏窗口
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0,
		0, 0, 0, 0,
		0, // HWND_MESSAGE
		0, hInstance, 0,
	)
	t.hwnd = hwnd

	// 创建托盘图标
	t.createTrayIcon()

	// 创建菜单
	t.createMenu()

	// 消息循环
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	t.running = false
}

// wndProc 窗口过程
func (t *TrayManager) wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYMSG:
		switch lParam {
		case WM_RBUTTONUP:
			t.showMenu()
			return 0
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			t.app.ShowWindow()
			return 0
		}
	case WM_COMMAND:
		switch wParam {
		case IDM_SHOW:
			t.app.ShowWindow()
			return 0
		case IDM_QUIT:
			t.Quit()
			return 0
		}
	case WM_DESTROY:
		t.removeTrayIcon()
		procPostQuitMessage.Call(0)
		return 0
	case WM_QUIT:
		t.removeTrayIcon()
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

// createTrayIcon 创建托盘图标
func (t *TrayManager) createTrayIcon() {
	tip, _ := windows.UTF16PtrFromString("打印服务器 - 双击显示")

	t.notifyData = NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATA{})),
		HWnd:             t.hwnd,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: WM_TRAYMSG,
		HIcon:            t.loadIcon(),
	}
	copy(t.notifyData.SzTip[:], (*[128]uint16)(unsafe.Pointer(tip))[:])

	ret, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.notifyData)))
	_ = ret
}

// loadIcon 加载图标
func (t *TrayManager) loadIcon() uintptr {
	// 尝试多个路径
	paths := []string{
		"build/windows/icon.ico",
		"bin/icon.ico",
		"icon.ico",
	}

	for _, p := range paths {
		iconPath, _ := windows.UTF16PtrFromString(p)
		hIcon, _, _ := procLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(iconPath)),
			IMAGE_ICON,
			16, 16,
			LR_LOADFROMFILE,
		)
		if hIcon != 0 {
			return hIcon
		}
	}

	// 使用默认应用图标
	const IDI_APPLICATION = 32512
	hIcon, _, _ := procLoadImageW.Call(
		0,
		IDI_APPLICATION,
		IMAGE_ICON,
		16, 16,
		LR_DEFAULTSIZE,
	)

	return hIcon
}

// createMenu 创建菜单
func (t *TrayManager) createMenu() {
	t.menu, _, _ = procCreatePopupMenu.Call()

	showText, _ := windows.UTF16PtrFromString("显示窗口")
	quitText, _ := windows.UTF16PtrFromString("退出")

	procAppendMenuW.Call(t.menu, 0, IDM_SHOW, uintptr(unsafe.Pointer(showText)))
	procAppendMenuW.Call(t.menu, MF_SEPARATOR, 0, 0)
	procAppendMenuW.Call(t.menu, 0, IDM_QUIT, uintptr(unsafe.Pointer(quitText)))
}

// showMenu 显示菜单
func (t *TrayManager) showMenu() {
	const (
		TPM_RIGHTALIGN  = 0x0008
		TPM_BOTTOMALIGN = 0x0020
		TPM_NONOTIFY    = 0x0080
		TPM_RETURNCMD   = 0x0100
	)

	// 获取鼠标位置
	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// 设置前台窗口（确保菜单能正确关闭）
	procSetForegroundWindow.Call(t.hwnd)

	// 显示菜单并获取选择的命令
	cmd, _, _ := procTrackPopupMenu.Call(
		t.menu,
		TPM_RIGHTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD|TPM_NONOTIFY,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		t.hwnd,
		0,
	)

	// 发送空消息确保菜单关闭
	procPostMessageW.Call(t.hwnd, 0, 0, 0)

	// 处理选择的命令
	if cmd != 0 {
		switch cmd {
		case IDM_SHOW:
			t.app.ShowWindow()
		case IDM_QUIT:
			t.Quit()
		}
	}
}

// Quit 退出托盘和应用
func (t *TrayManager) Quit() {
	t.app.quitFromTray = true
	t.removeTrayIcon()

	// 退出消息循环
	procPostQuitMessage.Call(0)

	// 退出 Wails
	t.app.Quit()
}

// removeTrayIcon 移除托盘图标
func (t *TrayManager) removeTrayIcon() {
	if t.notifyData.HWnd != 0 {
		procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.notifyData)))
		t.notifyData.HWnd = 0
	}
}

// UpdateStatus 更新状态
func (t *TrayManager) UpdateStatus() {}

// Init 初始化（兼容接口）
func (t *TrayManager) Init(hwnd uintptr) {}
