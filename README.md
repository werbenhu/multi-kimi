# multi-kimi

一个用 [Go + Wails v2](https://wails.io) 实现的桌面小工具，用于在多个 **Kimi Code** 账号之间一键切换登录凭据，并显示每个账号的 **周用量** 与 **5 小时用量**。

用量查询对齐 [dsh-usage-stats](https://github.com/Ychris12138/dsh-usage-stats) 的 `kimi-token-plan` 适配器：

```
GET https://api.kimi.com/coding/v1/usages
Authorization: Bearer <OAuth access_token>
```

- `limits[].detail` → 5 小时滚动窗口
- `usage` → 周用量
- 已用率 = `(limit - remaining) / limit * 100`

access_token 过期时会用 `refresh_token` 向 `https://auth.kimi.com/api/oauth/token` 刷新，并写回凭据文件。

## 凭据位置

| CLI | live 凭据文件 | profile 快照名 |
| --- | --- | --- |
| Kimi Code | `~/.kimi-code/credentials/kimi-code.json` | `kimi-code.json` |

## 工作原理

所有 Profile 快照保存在本机 `~/.multi-kimi/`（若存在旧目录 `~/.multi-account-tool/` 会自动迁移）：

```
~/.multi-kimi/
├── config.json                  # { version, active: { kimi: "..." } }
└── profiles/
    └── kimi/
        └── <profile名>/
            ├── kimi-code.json   # ~/.kimi-code/credentials/kimi-code.json 的内容快照
            └── meta.json        # 名称/时间戳
```

**切换账号**：

1. 幂等短路：目标已是当前 Profile 则不做任何事；
2. **回写保护**：先把当前 live 凭据快照回当前 Profile —— 防止 CLI 运行期间 OAuth
   token 轮换后、切换时把旧 token 写回去导致失效；
3. **原子激活**：把目标 Profile 的快照原子写入 live 路径（临时文件 + rename，
   失败自动用内存备份回滚，绝不出现半新半旧状态）；
4. 更新 `config.json` 的 active 指针和 Profile 时间戳。

**环境变量告警**：切换前检测会绕过凭据文件的环境变量并给出警告（只显示变量名，
绝不显示值）—— `KIMI_CODE_HOME` / `KIMI_MODEL_*`。

## 使用方法

1. 在终端正常登录 CLI（`kimi login`）；
2. 打开本工具，点「保存」，给 Profile 起个名（如 `work`）；
3. 在 CLI 里退出登录、换另一个账号登录，再保存一次（如 `personal`）；
4. 之后随时在工具里点「切换」即可在账号间切换，无需再登录。
5. 每个账号卡片会显示 5 小时用量、周用量、重置倒计时；打开时拉取一次，之后用卡片头部的刷新按钮手动更新。

其他操作：

- **重新捕获**：用当前 live 凭据刷新某个 Profile 的快照。CLI 运行期间同一账号的
  token 轮换属于正常现象（切换时会自动回写同步），不会提示；只有 live 被换成
  **另一个账号** 时才会提示「live 已是其他账号的凭据」，此时确认后重新捕获即可；
- **刷新用量**：卡片头部的刷新按钮刷新全部账号的用量；
- **删除**：只删快照，不影响 live 凭据文件；删除 active Profile 会同时清除指针；
- **存储目录**：在资源管理器中打开 `~/.multi-kimi`。

## 开发

要求：Go ≥ 1.24、Node ≥ 18、Wails CLI v2（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）。

```bash
wails dev          # 开发模式（热重载）
wails build        # 产出 build/bin/multi-kimi.exe
go test ./...      # 核心逻辑单元测试（临时 HOME，不碰真实凭据）
```

重新生成图标：`go run tmpicon/main.go`（产出 `build/appicon.png` 与 `build/windows/icon.ico`）。

## 代码结构

```
main.go        Wails 入口（窗口、资源、绑定）
app.go         绑定到前端的方法与视图 DTO（GetOverview/Capture/Switch/Delete…）
clidefs.go     Kimi Code 的 CLI 定义（凭据路径、环境变量告警）
store.go       Profile 存储与全局配置（config.json）
switcher.go    核心：capture / recapture / switch / delete / freshness
usage.go       Kimi 周用量 / 5 小时用量查询与 OAuth 刷新
fsutil.go      原子写文件 + Profile 名校验
ambient.go     环境变量绕过检测
core_test.go   核心流程单元测试
usage_test.go  用量解析与 token 刷新测试
frontend/      Vue 3 + Vite 前端（中文界面）
```

## 安全说明

- 凭据只在本机两个目录之间复制（live ↔ `~/.multi-kimi`），不会上传到第三方；
- 用量查询仅请求 Kimi 官方接口（`api.kimi.com` / `auth.kimi.com`）；
- 所有写入均为原子写，目录权限 `0700`、文件 `0600`；
- 界面与日志中绝不展示凭据内容，环境变量告警只显示变量名。
