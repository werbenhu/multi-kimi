package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SwitchResult 一次切换的结果与警告信息。
type SwitchResult struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	NoOp     bool     `json:"noOp"`
	Warnings []string `json:"warnings"`
}

// capture 将当前 live 凭据保存为新的 profile。
func (s *Store) capture(cliID, name string) error {
	def, err := s.Def(cliID)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if err := validateProfileName(name); err != nil {
		return err
	}
	if s.ProfileExists(cliID, name) {
		return fmt.Errorf("profile %q 已存在，可改用「重新捕获」覆盖", name)
	}
	live, err := os.ReadFile(def.Source.Path)
	if err != nil {
		return fmt.Errorf("读取 live 凭据失败（请先登录该 CLI）: %s", def.Source.Path)
	}
	dir := s.profileDir(cliID, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := atomicWriteFile(credPath(dir, def), live, 0o600); err != nil {
		return err
	}
	now := time.Now()
	return s.writeMeta(cliID, ProfileMeta{Name: name, CreatedAt: now, UpdatedAt: now})
}

// recapture 用当前 live 凭据覆盖已存在的 profile（保留创建时间）。
func (s *Store) recapture(cliID, name string) error {
	def, err := s.Def(cliID)
	if err != nil {
		return err
	}
	if err := validateProfileName(name); err != nil {
		return err
	}
	if !s.ProfileExists(cliID, name) {
		return fmt.Errorf("profile %q 不存在", name)
	}
	live, err := os.ReadFile(def.Source.Path)
	if err != nil {
		return fmt.Errorf("读取 live 凭据失败（请先登录该 CLI）: %s", def.Source.Path)
	}
	if err := atomicWriteFile(credPath(s.profileDir(cliID, name), def), live, 0o600); err != nil {
		return err
	}
	s.touchMeta(cliID, name)
	return nil
}

// switchTo 切换到目标 profile：
//  1. 幂等短路：目标已是当前 active 且存在；
//  2. 回写：把当前 live 凭据快照回当前 active profile（抢救 OAuth token 轮换）；
//  3. 激活：把目标 profile 存储的凭据原子写到 live 路径（失败则用内存备份回滚）；
//  4. 更新 config.json 的 active 指针与 meta.updatedAt。
func (s *Store) switchTo(cliID, target string) (*SwitchResult, error) {
	def, err := s.Def(cliID)
	if err != nil {
		return nil, err
	}
	if err := validateProfileName(target); err != nil {
		return nil, err
	}
	if !s.ProfileExists(cliID, target) {
		return nil, fmt.Errorf("profile %q 不存在", target)
	}
	res := &SwitchResult{To: target, Warnings: []string{}}
	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, err
	}
	current := cfg.Active[cliID]
	res.From = current

	// 1. 幂等 no-op
	if current != "" && current == target {
		res.NoOp = true
		res.Warnings = append(res.Warnings, envBypassWarnings(def)...)
		return res, nil
	}

	// 2. 回写当前 live 到当前 active profile，防止 token 轮换丢失
	if current != "" {
		if s.ProfileExists(cliID, current) {
			live, rerr := os.ReadFile(def.Source.Path)
			if rerr == nil {
				if werr := atomicWriteFile(credPath(s.profileDir(cliID, current), def), live, 0o600); werr == nil {
					s.touchMeta(cliID, current)
				} else {
					res.Warnings = append(res.Warnings, fmt.Sprintf("回写当前 profile %q 失败: %v", current, werr))
				}
			} else {
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("live 凭据文件不存在（%s），已跳过对 profile %q 的回写", def.Source.Path, current))
			}
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf("当前 active profile %q 不存在，已跳过回写", current))
		}
	}

	// 3. 激活目标 profile（先取 live 备份，写失败时回滚）
	stored, err := os.ReadFile(credPath(s.profileDir(cliID, target), def))
	if err != nil {
		return nil, fmt.Errorf("profile %q 缺少凭据文件（快照损坏）: %w", target, err)
	}
	liveBackup, liveExisted := os.ReadFile(def.Source.Path)
	if err := atomicWriteFile(def.Source.Path, stored, 0o600); err != nil {
		if liveExisted == nil {
			_ = atomicWriteFile(def.Source.Path, liveBackup, 0o600)
		} else {
			_ = os.Remove(def.Source.Path)
		}
		return nil, fmt.Errorf("写入 live 凭据失败（已回滚）: %w", err)
	}

	// 4. 更新 active 指针
	cfg.Active[cliID] = target
	if err := s.SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("凭据已切换，但更新 config.json 失败: %w", err)
	}
	s.touchMeta(cliID, target)

	res.Warnings = append(res.Warnings, envBypassWarnings(def)...)
	if !res.NoOp {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("凭据文件已切换为 %q。如果已有 %s 进程在运行，请重启该进程或重新打开终端，否则旧进程会继续使用缓存的账号", target, def.Name))
	}
	return res, nil
}

// deleteProfile 删除 profile；若它正是当前 active，则同时清除 active 指针。
func (s *Store) deleteProfile(cliID, name string) error {
	if _, err := s.Def(cliID); err != nil {
		return err
	}
	if err := validateProfileName(name); err != nil {
		return err
	}
	if !s.ProfileExists(cliID, name) {
		return fmt.Errorf("profile %q 不存在", name)
	}
	if err := os.RemoveAll(s.profileDir(cliID, name)); err != nil {
		return err
	}
	if s.ActiveProfile(cliID) == name {
		cfg, err := s.LoadConfig()
		if err != nil {
			return err
		}
		delete(cfg.Active, cliID)
		if err := s.SaveConfig(cfg); err != nil {
			return err
		}
	}
	return nil
}

// Freshness 比较 live 凭据与 profile 快照。
//   - "fresh"        一致，或仅 token 被 CLI 轮换但仍是同一账号（正常现象，
//     切换时的回写保护会自动同步快照，无需打扰用户）
//   - "rotated"      live 已是另一个账号的凭据
//   - "missing-live" live 凭据文件不存在
//   - "missing-snap" profile 快照缺失
func (s *Store) Freshness(cliID, name string) (string, string) {
	def, err := s.Def(cliID)
	if err != nil {
		return "", ""
	}
	live, lerr := os.ReadFile(def.Source.Path)
	if lerr != nil {
		return "missing-live", "未登录或已登出"
	}
	snap, serr := os.ReadFile(credPath(s.profileDir(cliID, name), def))
	if serr != nil {
		return "missing-snap", "快照缺失"
	}
	if bytes.Equal(live, snap) {
		return "fresh", "凭据与快照一致"
	}
	// 字节不同但 JWT 身份相同 → 只是 token 轮换；身份都取不到时保守视为不同账号
	if id := credsUserID(live); id != "" && id == credsUserID(snap) {
		return "fresh", "凭据与快照一致"
	}
	return "rotated", "live 已是其他账号的凭据（如需覆盖快照请重新捕获）"
}

// credPath profile 目录内凭据文件的路径。
func credPath(dir string, def CliDef) string {
	return filepath.Join(dir, def.Source.SaveAs)
}
