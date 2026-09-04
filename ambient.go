package main

import (
	"fmt"
	"os"
	"strings"
)

// envBypassWarnings 返回当前进程中会绕过凭据文件的环境变量警告。
// 只输出变量名，绝不输出值，避免泄露密钥。
func envBypassWarnings(def CliDef) []string {
	var out []string
	for _, eb := range def.EnvBypass {
		switch eb.Kind {
		case "name":
			if os.Getenv(eb.Pattern) != "" {
				out = append(out, fmt.Sprintf("检测到环境变量 %s 已设置，%s（它优先生效，切换结果可能被覆盖）", eb.Pattern, eb.Note))
			}
		case "prefix":
			for _, kv := range os.Environ() {
				k, v, ok := strings.Cut(kv, "=")
				if ok && strings.HasPrefix(k, eb.Pattern) && v != "" {
					out = append(out, fmt.Sprintf("检测到环境变量 %s 已设置，%s（它优先生效，切换结果可能被覆盖）", k, eb.Note))
				}
			}
		}
	}
	return out
}
