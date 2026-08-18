//go:build windows

package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// trayManager управляет иконкой в системном трее Windows.
// При клике по иконке — показывает окно. При закрытии окна — сворачивает в трей.
type trayManager struct {
	app   *App
	ctx   context.Context
	once  sync.Once
}

func newTrayManager(app *App) *trayManager {
	return &trayManager{app: app}
}

// Start запускает system tray в отдельном goroutine.
// Вызывать из OnStartup.
func (t *trayManager) Start(ctx context.Context) {
	t.ctx = ctx
	t.once.Do(func() {
		go systray.Run(t.onReady, t.onExit)
	})
}

// onReady вызывается при создании иконки в трее.
func (t *trayManager) onReady() {
	// Загружаем иконку из .ico рядом с exe
	icoPath := filepath.Join(exeDir(), "appicon.ico")
	data, err := os.ReadFile(icoPath)
	if err != nil {
		// Fallback: пытаемся встроенную иконку (через embed не работает для systray,
		// поэтому читаем с диска)
		log.Printf("[tray] не удалось загрузить иконку: %v", err)
	} else {
		systray.SetIcon(data)
	}

	systray.SetTitle("Snowden")
	systray.SetTooltip("snowden.system — нажми для управления")

	// Меню
	mShow := systray.AddMenuItem("Показать", "Открыть окно приложения")
	mAuto := systray.AddMenuItemCheckbox("Автозапуск", "Запускать с Windows", false)
	systray.AddSeparator()
	mExit := systray.AddMenuItem("Выход", "Закрыть приложение и VPN")

	// Обновляем состояние автозапуска
	if isAutostartEnabled() {
		mAuto.Check()
	}

	// Обработчики
	go func() {
		for range mShow.ClickedCh {
			runtime.WindowShow(t.ctx)
		}
	}()

	go func() {
		for range mAuto.ClickedCh {
			enabled := isAutostartEnabled()
			newState := !enabled
			_ = setAutostartRegistry(newState)
			if newState {
				mAuto.Check()
			} else {
				mAuto.Uncheck()
			}
		}
	}()

	go func() {
		for range mExit.ClickedCh {
			// Graceful shutdown
			_ = clearSystemProxy()
			systray.Quit()
			runtime.Quit(t.ctx)
		}
	}()
}

// onExit вызывается при выходе из трея.
func (t *trayManager) onExit() {
	log.Println("[tray] выход из system tray")
}

// exeDir возвращает директорию с .exe (где лежит appicon.ico).
func exeDir() string {
	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(ex)
}
