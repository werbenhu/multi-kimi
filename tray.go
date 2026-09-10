package main

import (
	_ "embed"
	"runtime"
	"strings"
	"sync"

	"github.com/energye/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

type trayState struct {
	mu       sync.Mutex
	ready    bool
	status   *systray.MenuItem
	quitOnce sync.Once
}

func newTrayState() *trayState {
	return &trayState{}
}

func (a *App) startTray() {
	// systray 的 Windows 消息泵跑在调用方线程上，必须锁定 OS 线程：
	// 否则 goroutine 一旦被调度迁移，托盘点击消息就再也分发不到，图标假死。
	go func() {
		runtime.LockOSThread()
		systray.Run(a.onTrayReady, func() {})
	}()
}

func (a *App) stopTray() {
	if a.tray == nil {
		return
	}
	a.tray.quitOnce.Do(systray.Quit)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("multi-kimi")

	// 左键单击直接唤出主窗口。右键不注册回调，由 systray 默认弹出菜单。
	systray.SetOnClick(func(systray.IMenu) {
		a.showMainWindow()
	})

	show := systray.AddMenuItem("显示主窗口", "打开 multi-kimi")
	a.tray.status = systray.AddMenuItem("切换账号请打开主窗口", "")
	a.tray.status.Disable()
	systray.AddSeparator()

	refresh := systray.AddMenuItem("刷新用量", "重新读取账号状态与用量")
	quit := systray.AddMenuItem("退出", "完全退出 multi-kimi")

	a.tray.mu.Lock()
	a.tray.ready = true
	a.tray.mu.Unlock()

	show.Click(a.showMainWindow)
	refresh.Click(func() {
		a.setTrayStatus("正在刷新…")
		if err := a.refreshTrayMenu(); err != nil {
			a.setTrayStatus("刷新失败：" + compactTrayText(err.Error()))
			return
		}
		a.setTrayStatus("已刷新 · 切换账号请打开主窗口")
	})
	quit.Click(func() {
		a.stopTray()
		if a.ctx != nil {
			wruntime.Quit(a.ctx)
		}
	})

	if err := a.refreshTrayMenu(); err != nil {
		a.setTrayStatus("读取账号失败：" + compactTrayText(err.Error()))
	}
}

func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
}

func (a *App) setTrayStatus(message string) {
	if a.tray == nil {
		return
	}
	a.tray.mu.Lock()
	defer a.tray.mu.Unlock()
	if a.tray.status != nil {
		a.tray.status.SetTitle(message)
	}
}

// refreshTrayMenu 只更新托盘 tooltip（当前账号 + 用量），账号切换在主窗口进行。
func (a *App) refreshTrayMenu() error {
	if a.tray == nil {
		return nil
	}
	a.tray.mu.Lock()
	ready := a.tray.ready
	a.tray.mu.Unlock()
	if !ready {
		return nil
	}

	views, err := a.GetOverview()
	if err != nil {
		return err
	}

	if tip := usageTooltip(views); tip != "" {
		systray.SetTooltip(tip)
	} else {
		systray.SetTooltip("multi-kimi")
	}
	return nil
}

func usageTooltip(views []CliView) string {
	for _, cli := range views {
		if text := compactUsageText(cli.LiveUsage); text != "" {
			name := cli.ActiveProfile
			if name == "" {
				return "multi-kimi  " + text
			}
			return "multi-kimi · " + name + "  " + text
		}
	}
	return ""
}

func compactTrayText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= 48 {
		return value
	}
	return string([]rune(value)[:48]) + "…"
}
