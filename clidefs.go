package main

import (
	"path/filepath"
)

// storeRootName 工具自身数据目录名（位于用户 HOME 下）。
const storeRootName = ".multi-kimi"

// oldStoreRootName 旧版 multi-account 数据目录，启动时会自动迁移到 storeRootName。
const oldStoreRootName = ".multi-account-tool"

// CliDefs 返回受支持的 CLI 定义。home 为用户 HOME 目录的绝对路径。
//
// Kimi Code 凭据：~/.kimi-code/credentials/kimi-code.json（OAuth token；账号身份由它决定。
// config.toml 中的 api_key 为空、走 OAuth，故切换此文件即切换账号。
// 旧版 kimi-cli 的 ~/.kimi/config.toml 已迁移，不再使用。）
func CliDefs(home string) []CliDef {
	return []CliDef{
		{
			ID:   "kimi",
			Name: "Kimi Code",
			Source: FileSource{
				Path:   filepath.Join(home, ".kimi-code", "credentials", "kimi-code.json"),
				SaveAs: "kimi-code.json",
			},
			EnvBypass: []EnvBypass{
				{Kind: "name", Pattern: "KIMI_CODE_HOME", Note: "KIMI_CODE_HOME 会整体迁移数据目录，设置后本工具切换的凭据不会生效"},
				{Kind: "prefix", Pattern: "KIMI_MODEL_", Note: "KIMI_MODEL_* 环境变量会用自带 API key 合成临时 provider，覆盖账号切换结果"},
			},
		},
	}
}
