package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store 管理 profile 存储与全局配置，所有路径都基于注入的 home 目录，便于测试。
type Store struct {
	Home string // 用户 HOME
	Root string // 工具数据目录 ~/.multi-kimi
	Defs []CliDef
}

// NewStore 用真实用户 HOME 构造 Store。
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取用户 HOME: %w", err)
	}
	return NewStoreAt(home), nil
}

// NewStoreAt 用指定 home 构造 Store（测试用）。
func NewStoreAt(home string) *Store {
	return &Store{
		Home: home,
		Root: resolveStoreRoot(home),
		Defs: CliDefs(home),
	}
}

// resolveStoreRoot 优先使用 ~/.multi-kimi；若只有旧目录 ~/.multi-account-tool 则尝试迁移。
func resolveStoreRoot(home string) string {
	newRoot := filepath.Join(home, storeRootName)
	oldRoot := filepath.Join(home, oldStoreRootName)
	if _, err := os.Stat(newRoot); err == nil {
		return newRoot
	}
	if _, err := os.Stat(oldRoot); err == nil {
		if err := os.Rename(oldRoot, newRoot); err == nil {
			return newRoot
		}
		return oldRoot
	}
	return newRoot
}

// Def 返回指定 CLI 的定义。
func (s *Store) Def(cliID string) (CliDef, error) {
	for _, d := range s.Defs {
		if d.ID == cliID {
			return d, nil
		}
	}
	return CliDef{}, fmt.Errorf("不支持的 CLI: %q", cliID)
}

// profileDir profiles/<cli>/<name> 的绝对路径。
func (s *Store) profileDir(cliID, name string) string {
	return filepath.Join(s.Root, "profiles", cliID, name)
}

// ProfileExists 判断 profile 目录是否存在。
func (s *Store) ProfileExists(cliID, name string) bool {
	fi, err := os.Stat(s.profileDir(cliID, name))
	return err == nil && fi.IsDir()
}

// ListProfiles 列出某 CLI 的全部 profile 元数据，按名称排序。
func (s *Store) ListProfiles(cliID string) ([]ProfileMeta, error) {
	if _, err := s.Def(cliID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.Root, "profiles", cliID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ProfileMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta, err := s.readMeta(cliID, e.Name())
		if err != nil || meta.Name == "" {
			// 兼容缺少/损坏 meta.json 的目录
			meta = ProfileMeta{Name: e.Name()}
		}
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// readMeta 读取 profile 的 meta.json。
func (s *Store) readMeta(cliID, name string) (ProfileMeta, error) {
	var meta ProfileMeta
	data, err := os.ReadFile(filepath.Join(s.profileDir(cliID, name), "meta.json"))
	if err != nil {
		return meta, err
	}
	err = json.Unmarshal(data, &meta)
	return meta, err
}

// writeMeta 原子写 profile 的 meta.json。
func (s *Store) writeMeta(cliID string, meta ProfileMeta) error {
	meta.Name = strings.TrimSpace(meta.Name)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(filepath.Join(s.profileDir(cliID, meta.Name), "meta.json"), data, 0o600)
}

// touchMeta 更新 profile 的 updatedAt。
func (s *Store) touchMeta(cliID, name string) {
	meta, err := s.readMeta(cliID, name)
	if err != nil {
		return
	}
	meta.UpdatedAt = time.Now()
	_ = s.writeMeta(cliID, meta)
}

// configPath 全局配置文件路径。
func (s *Store) configPath() string {
	return filepath.Join(s.Root, "config.json")
}

// LoadConfig 读取全局配置；文件不存在时返回初始化值。
func (s *Store) LoadConfig() (GlobalConfig, error) {
	cfg := GlobalConfig{Version: 1, Active: map[string]string{}}
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config.json 解析失败: %w", err)
	}
	if cfg.Active == nil {
		cfg.Active = map[string]string{}
	}
	return cfg, nil
}

// SaveConfig 原子写全局配置。
func (s *Store) SaveConfig(cfg GlobalConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(s.configPath(), data, 0o600)
}

// ActiveProfile 返回某 CLI 当前激活的 profile 名（可能为空）。
func (s *Store) ActiveProfile(cliID string) string {
	cfg, err := s.LoadConfig()
	if err != nil {
		return ""
	}
	return cfg.Active[cliID]
}
