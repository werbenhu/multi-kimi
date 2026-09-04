package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// atomicWriteFile 原子写文件：先写同目录临时文件，再 rename 覆盖目标。
// 避免切换凭据时出现半写状态。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后为 no-op
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// homeShorten 把用户主目录前缀折叠成 ~，并用正斜杠，供界面展示用。
func homeShorten(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	return filepath.ToSlash(path)
}

// windowsReserved Windows 保留设备名，不能用作目录名。
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// validateProfileName 校验 profile 名：1~40 个字符，允许字母（含中文）、数字、下划线、点、连字符。
func validateProfileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("profile 名不能为空")
	}
	r := []rune(name)
	if len(r) > 40 {
		return fmt.Errorf("profile 名过长（最多 40 个字符）")
	}
	for _, c := range r {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '.' && c != '-' {
			return fmt.Errorf("profile 名只能包含中文/字母/数字/下划线/点/连字符: %q", name)
		}
	}
	if strings.Trim(name, ".") == "" { // ".", "..", "..."
		return fmt.Errorf("profile 名不能全为点号")
	}
	if windowsReserved[strings.ToUpper(name)] {
		return fmt.Errorf("profile 名 %q 是 Windows 保留名", name)
	}
	return nil
}
