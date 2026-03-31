package main

import (
	"context"
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// iconData 托盘图标数据（可选）
var iconData []byte

func main() {
	// 创建应用实例
	app := NewApp()
	app.trayManager = NewTrayManager(app)

	// 创建 Wails 应用选项
	opts := &options.App{
		Title:             "打印服务器",
		Width:             1024,
		Height:            768,
		MinWidth:          800,
		MinHeight:         600,
		DisableResize:     false,
		Frameless:         false,
		StartHidden:       true,
		HideWindowOnClose: runtime.GOOS == "windows" && app.config.MinimizeTray,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		OnDomReady: func(ctx context.Context) {
			// DOM 准备好后显示窗口
			wailsRuntime.WindowShow(ctx)
			// 初始化托盘
			if runtime.GOOS == "windows" && app.config.MinimizeTray {
				go app.trayManager.Run()
			}
		},
		OnBeforeClose: func(ctx context.Context) bool {
			// Windows 下隐藏到托盘而不是关闭
			if runtime.GOOS == "windows" && app.config.MinimizeTray && !app.quitFromTray {
				app.HideWindow()
				return true
			}
			return false
		},
		Bind: []interface{}{
			app,
		},
	}

	// Windows 特定选项
	if runtime.GOOS == "windows" {
		opts.Windows = &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			WebviewUserDataPath:               "",
			Theme:                             windows.SystemDefault,
		}
	}

	// 创建 Wails 应用
	err := wails.Run(opts)
	if err != nil {
		log.Fatalf("启动应用失败: %s", err.Error())
	}
}
