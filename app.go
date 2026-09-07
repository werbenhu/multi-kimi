package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 绑定到前端的入口。
type App struct {
	ctx            context.Context
	store          *Store
	mu             sync.Mutex
	tray           *trayState
	httpc          *http.Client
	usage          *usageCache
	usageRefreshMu sync.Mutex
}

func NewApp() *App {
	store, err := NewStore()
	if err != nil {
		// 启动时拿不到 HOME 属极端情况，留待首次调用时报错
		store = nil
	}
	return &App{
		store: store,
		tray:  newTrayState(),
		httpc: &http.Client{Timeout: usageHTTPTimeout},
		usage: newUsageCache(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startTray()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopTray()
}

func (a *App) mustStore() (*Store, error) {
	if a.store == nil {
		return nil, fmt.Errorf("初始化失败：无法定位用户 HOME 目录")
	}
	return a.store, nil
}

// ---- 前端视图 DTO ----

// ProfileView 单个 profile 的展示数据。
type ProfileView struct {
	Name          string `json:"name"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	HasCredential bool   `json:"hasCredential"`
	IsActive      bool   `json:"isActive"`
	// 仅对 active profile 有意义: fresh / rotated / missing-live / missing-snap / ""
	Freshness     string     `json:"freshness"`
	FreshnessNote string     `json:"freshnessNote"`
	Usage         *KimiUsage `json:"usage,omitempty"`
}

// CliView 单个 CLI 面板的展示数据。
type CliView struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	LivePath      string        `json:"livePath"`
	LivePathShort string        `json:"livePathShort"`
	LiveExists    bool          `json:"liveExists"`
	ActiveProfile string        `json:"activeProfile"`
	Profiles      []ProfileView `json:"profiles"`
	EnvWarnings   []string      `json:"envWarnings"`
	LiveUsage     *KimiUsage    `json:"liveUsage,omitempty"`
}

// GetOverview 返回两个 CLI 的完整状态，前端每次操作后都会重新调用。
func (a *App) GetOverview() ([]CliView, error) {
	a.mu.Lock()
	views, err := a.getOverviewLocked()
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	a.attachUsage(views)
	a.maybeRefreshUsage()
	return views, nil
}

func (a *App) getOverviewLocked() ([]CliView, error) {
	s, err := a.mustStore()
	if err != nil {
		return nil, err
	}
	var out []CliView
	for _, def := range s.Defs {
		view := CliView{
			ID:            def.ID,
			Name:          def.Name,
			LivePath:      def.Source.Path,
			LivePathShort: homeShorten(def.Source.Path),
			EnvWarnings:   envBypassWarnings(def),
			ActiveProfile: s.ActiveProfile(def.ID),
		}
		if _, err := os.Stat(def.Source.Path); err == nil {
			view.LiveExists = true
		}
		metas, err := s.ListProfiles(def.ID)
		if err != nil {
			return nil, err
		}
		for _, m := range metas {
			pv := ProfileView{
				Name:          m.Name,
				CreatedAt:     m.CreatedAt.Format("2006-01-02 15:04"),
				UpdatedAt:     m.UpdatedAt.Format("2006-01-02 15:04"),
				HasCredential: false,
				IsActive:      m.Name == view.ActiveProfile,
			}
			if _, err := os.Stat(filepath.Join(s.profileDir(def.ID, m.Name), def.Source.SaveAs)); err == nil {
				pv.HasCredential = true
			}
			if pv.IsActive {
				pv.Freshness, pv.FreshnessNote = s.Freshness(def.ID, m.Name)
			}
			view.Profiles = append(view.Profiles, pv)
		}
		if view.Profiles == nil {
			view.Profiles = []ProfileView{}
		}
		out = append(out, view)
	}
	return out, nil
}

// CaptureProfile 把当前 live 凭据保存为新 profile。
func (a *App) CaptureProfile(cliID, name string) error {
	a.mu.Lock()
	s, err := a.mustStore()
	if err != nil {
		a.mu.Unlock()
		return err
	}
	err = s.capture(cliID, name)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.invalidateLiveAndProfile(cliID, name)
	a.overviewChanged()
	return nil
}

// RecaptureProfile 用当前 live 凭据覆盖指定 profile 的快照。
func (a *App) RecaptureProfile(cliID, name string) error {
	a.mu.Lock()
	s, err := a.mustStore()
	if err != nil {
		a.mu.Unlock()
		return err
	}
	err = s.recapture(cliID, name)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.invalidateLiveAndProfile(cliID, name)
	a.overviewChanged()
	return nil
}

// SwitchProfile 切换到目标 profile。
func (a *App) SwitchProfile(cliID, target string) (*SwitchResult, error) {
	a.mu.Lock()
	s, err := a.mustStore()
	if err != nil {
		a.mu.Unlock()
		return nil, err
	}
	from := ""
	if s != nil {
		from = s.ActiveProfile(cliID)
	}
	res, err := s.switchTo(cliID, target)
	a.mu.Unlock()
	if err != nil {
		return res, err
	}
	a.invalidateLiveAndProfile(cliID, from)
	a.invalidateLiveAndProfile(cliID, target)
	a.overviewChanged()
	return res, nil
}

// DeleteProfile 删除 profile（若为 active 同时清除指针）。
func (a *App) DeleteProfile(cliID, name string) error {
	a.mu.Lock()
	s, err := a.mustStore()
	if err != nil {
		a.mu.Unlock()
		return err
	}
	err = s.deleteProfile(cliID, name)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if a.usage != nil {
		a.usage.invalidate(usageKeyProfile(cliID, name))
	}
	a.overviewChanged()
	return nil
}

func (a *App) overviewChanged() {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "overview-changed", nil)
	}
	_ = a.refreshTrayMenu()
}

// openWithSystem 用系统默认方式打开目录或文件。
func openWithSystem(target string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", target).Start()
	case "darwin":
		return exec.Command("open", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

// OpenStoreDir 在系统资源管理器中打开 profile 存储目录。
func (a *App) OpenStoreDir() error {
	s, err := a.mustStore()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return err
	}
	return openWithSystem(s.Root)
}

// OpenLiveDir 打开某 CLI 的 live 凭据所在目录（不存在则提示）。
func (a *App) OpenLiveDir(cliID string) error {
	s, err := a.mustStore()
	if err != nil {
		return err
	}
	def, err := s.Def(cliID)
	if err != nil {
		return err
	}
	dir := filepath.Dir(def.Source.Path)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("目录不存在（尚未登录过该 CLI）: %s", dir)
	}
	return openWithSystem(dir)
}

// OpenLiveFile 用系统默认程序打开某 CLI 的 live 凭据文件（不存在则提示）。
func (a *App) OpenLiveFile(cliID string) error {
	s, err := a.mustStore()
	if err != nil {
		return err
	}
	def, err := s.Def(cliID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(def.Source.Path); err != nil {
		return fmt.Errorf("凭据文件不存在（尚未登录过该 CLI）: %s", def.Source.Path)
	}
	return openWithSystem(def.Source.Path)
}

// AppInfo 供「关于」展示。
func (a *App) AppInfo() map[string]string {
	s, _ := a.mustStore()
	root := ""
	if s != nil {
		root = s.Root
	}
	return map[string]string{
		"version":   version,
		"storeRoot": root,
		"go":        runtime.Version(),
		"now":       time.Now().Format("2006-01-02 15:04:05"),
	}
}
