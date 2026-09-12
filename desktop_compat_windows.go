//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wailsapp/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

var legacyCompatibilityMarker string

func checkDesktopCompatibility() error {
	if !legacyWindowsBuild {
		return nil
	}
	if !strings.HasPrefix(legacyCompatibilityMarker, "IrisWindowsLegacy/go") {
		return fmt.Errorf("сборка для старых Windows не подготовлена: используйте 00_RELEASE_WINDOWS")
	}
	version := windows.RtlGetVersion()
	if version.MajorVersion < 6 || (version.MajorVersion == 6 && (version.MinorVersion == 0 || (version.MinorVersion == 1 && version.BuildNumber < 7601))) {
		return fmt.Errorf("нужна Windows 7 с пакетом обновления SP1 или более новая Windows")
	}
	if err := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetDefaultDllDirectories").Find(); err != nil {
		return fmt.Errorf("не установлены необходимые обновления Windows: требуется поддержка SetDefaultDllDirectories (KB2533623 или заменяющее обновление)")
	}
	installed, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
	if err != nil {
		return fmt.Errorf("проверка WebView2: %w", err)
	}
	if installed != "" {
		major, err := strconv.Atoi(strings.SplitN(installed, ".", 2)[0])
		if err != nil || major < 109 {
			return fmt.Errorf("установлен WebView2 %s; обновите компонент официальным установщиком Microsoft: для Windows 7/8/8.1 нужна версия 109", installed)
		}
	}
	return nil
}
