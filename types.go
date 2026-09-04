package main

import "time"

// FileSource 描述某个 CLI 的一份 live 凭据文件，以及它在 profile 目录中的存储文件名。
type FileSource struct {
	Path   string `json:"path"`   // live 凭据绝对路径，如 ~/.kimi-code/credentials/kimi-code.json（已展开）
	SaveAs string `json:"saveAs"` // 在 profile 目录中保存的文件名，如 kimi-code.json
}

// EnvBypass 描述一个可能绕过凭据文件生效的环境变量（按名称或前缀匹配）。
type EnvBypass struct {
	Kind    string `json:"kind"`    // "name" 或 "prefix"
	Pattern string `json:"pattern"` // 变量名或前缀
	Note    string `json:"note"`
}

// CliDef 一个受支持的 CLI 定义。
type CliDef struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Source    FileSource  `json:"source"`
	EnvBypass []EnvBypass `json:"envBypass"`
}

// ProfileMeta profile 元数据，存储于 profiles/<cli>/<name>/meta.json。
type ProfileMeta struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GlobalConfig 全局配置 ~/.multi-kimi/config.json，记录每个 CLI 当前激活的 profile。
type GlobalConfig struct {
	Version int               `json:"version"`
	Active  map[string]string `json:"active"`
}
