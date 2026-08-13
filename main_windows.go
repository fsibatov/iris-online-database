//go:build windows

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const desktopInstanceID = "a4d8c84f-0785-4e1e-b321-0d69217a86da"

type DesktopBridge struct {
	mu  sync.RWMutex
	ctx context.Context
}

func (b *DesktopBridge) startup(ctx context.Context) {
	b.mu.Lock()
	b.ctx = ctx
	b.mu.Unlock()
}

func (b *DesktopBridge) shutdown() {
	b.mu.Lock()
	b.ctx = nil
	b.mu.Unlock()
}

func (b *DesktopBridge) focusWindow() {
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
	wailsruntime.Show(ctx)
}

func (b *DesktopBridge) OpenExternalURL(raw string) error {
	target, err := validateExternalURL(raw)
	if err != nil {
		return err
	}
	b.mu.RLock()
	ctx := b.ctx
	b.mu.RUnlock()
	if ctx == nil {
		return fmt.Errorf("окно приложения ещё не готово")
	}
	wailsruntime.BrowserOpenURL(ctx, target)
	return nil
}

func main() {
	if err := runDesktop(); err != nil {
		showStartupMessage("Iris Online Database не удалось запустить.\n\n" + err.Error())
		os.Exit(1)
	}
}

func runDesktop() error {
	app, err := newApplication()
	if err != nil {
		return err
	}
	webAssets, err := fs.Sub(embedded, "web")
	if err != nil {
		_ = app.shutdown()
		return fmt.Errorf("встроенный интерфейс: %w", err)
	}

	bridge := &DesktopBridge{}
	err = wails.Run(&options.App{
		Title:                            "Iris Online Database",
		Width:                            1280,
		Height:                           820,
		MinWidth:                         720,
		MinHeight:                        520,
		WindowStartState:                 options.Normal,
		BackgroundColour:                 options.NewRGB(16, 20, 27),
		EnableDefaultContextMenu:         false,
		EnableFraudulentWebsiteDetection: false,
		BindingsAllowedOrigins:           "",
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     false,
			DisableWebViewDrop: true,
		},
		AssetServer: &assetserver.Options{
			Assets:     webAssets,
			Handler:    app.routes(),
			Middleware: withSecurityHeaders,
		},
		OnStartup: func(ctx context.Context) {
			bridge.startup(ctx)
			app.logger.Printf("desktop-приложение запущено: version=%s", appVersion)
		},
		OnShutdown: func(_ context.Context) {
			bridge.shutdown()
			_ = app.shutdown()
		},
		Bind: []interface{}{bridge},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: desktopInstanceID,
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				bridge.focusWindow()
			},
		},
		Windows: &windows.Options{
			WebviewUserDataPath:  app.paths.WebViewData,
			IsZoomControlEnabled: true,
			DisablePinchZoom:     false,
			Theme:                windows.SystemDefault,
			WindowClassName:      "IrisOnlineDatabaseWindow",
			DLLSearchPaths:       windows.DLLSearchApplicationDir | windows.DLLSearchSystem32,
			Messages: &windows.Messages{
				InstallationRequired: "Для Iris Online Database требуется Microsoft Edge WebView2 Runtime. Нажмите OK, чтобы безопасно скачать и установить Evergreen Runtime.",
				UpdateRequired:       "Microsoft Edge WebView2 Runtime нужно обновить. Нажмите OK, чтобы скачать актуальную Evergreen-версию.",
				MissingRequirements:  "Необходим компонент WebView2",
				Webview2NotInstalled: "Microsoft Edge WebView2 Runtime не установлен",
				Error:                "Ошибка Iris Online Database",
				FailedToInstall:      "WebView2 Runtime не удалось установить. Проверьте подключение к интернету или обратитесь к администратору.",
				DownloadPage:         "Приложению требуется Microsoft Edge WebView2 Runtime. Нажмите OK, чтобы открыть официальную страницу загрузки. Минимальная версия: ",
				PressOKToInstall:     "Нажмите OK для установки.",
				ContactAdmin:         "Для запуска требуется WebView2 Runtime. Обратитесь к системному администратору.",
				InvalidFixedWebview2: "Указанный WebView2 Runtime недействителен.",
				WebView2ProcessCrash: "Процесс WebView2 аварийно завершился. Перезапустите приложение.",
			},
		},
	})
	if err != nil {
		_ = app.shutdown()
		return fmt.Errorf("desktop runtime: %w", err)
	}
	return nil
}
