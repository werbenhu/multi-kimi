package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// newTestStore 在临时 HOME 下构造 Store，并可选写入 kimi 的 live 凭据。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	home := t.TempDir()
	s := NewStoreAt(home)
	if len(s.Defs) != 1 || s.Defs[0].ID != "kimi" {
		t.Fatalf("CLI 定义不符: %+v", s.Defs)
	}
	return s
}

func writeLive(t *testing.T, s *Store, cliID, content string) {
	t.Helper()
	def, err := s.Def(cliID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(def.Source.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(def.Source.Path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readStoreCred(t *testing.T, s *Store, cliID, name string) string {
	t.Helper()
	def, _ := s.Def(cliID)
	data, err := os.ReadFile(filepath.Join(s.profileDir(cliID, name), def.Source.SaveAs))
	if err != nil {
		t.Fatalf("读取 profile 凭据失败: %v", err)
	}
	return string(data)
}

func readLiveCred(t *testing.T, s *Store, cliID string) string {
	t.Helper()
	def, _ := s.Def(cliID)
	data, err := os.ReadFile(def.Source.Path)
	if err != nil {
		t.Fatalf("读取 live 凭据失败: %v", err)
	}
	return string(data)
}

// 完整流程：捕获 A → 捕获 B → A→B 切换 → 回写轮换 → B→A 再切回。
func TestCaptureAndSwitch(t *testing.T) {
	s := newTestStore(t)

	// 未登录时 capture 应失败
	if err := s.capture("kimi", "a"); err == nil {
		t.Fatal("未登录时 capture 应该报错")
	}

	writeLive(t, s, "kimi", `{"token":"AAA"}`)
	if err := s.capture("kimi", "work"); err != nil {
		t.Fatalf("capture work: %v", err)
	}
	// 重名 capture 应失败
	if err := s.capture("kimi", "work"); err == nil {
		t.Fatal("重名 capture 应该报错")
	}
	// 非法名
	for _, bad := range []string{"", "a b", "a/b", "..", "con", "a\\b"} {
		if err := s.capture("kimi", bad); err == nil {
			t.Fatalf("非法名 %q 应该被拒绝", bad)
		}
	}

	// 重新登录为账号 B
	writeLive(t, s, "kimi", `{"token":"BBB"}`)
	if err := s.capture("kimi", "personal"); err != nil {
		t.Fatalf("capture personal: %v", err)
	}

	// 首次切换：无 active，直接激活 work
	if _, err := s.switchTo("kimi", "work"); err != nil {
		t.Fatalf("switch work: %v", err)
	}
	if got := readLiveCred(t, s, "kimi"); got != `{"token":"AAA"}` {
		t.Fatalf("切换后 live 应为 AAA，实际 %s", got)
	}
	if s.ActiveProfile("kimi") != "work" {
		t.Fatalf("active 应为 work")
	}

	// 模拟 CLI 运行期间 token 轮换
	writeLive(t, s, "kimi", `{"token":"AAA-rotated"}`)

	// work → personal：应先把轮换后的 live 回写进 work，再激活 personal
	res, err := s.switchTo("kimi", "personal")
	if err != nil {
		t.Fatalf("switch personal: %v", err)
	}
	if res.From != "work" || res.To != "personal" {
		t.Fatalf("切换结果不符: %+v", res)
	}
	if got := readStoreCred(t, s, "kimi", "work"); got != `{"token":"AAA-rotated"}` {
		t.Fatalf("回写失败：work 快照应为 AAA-rotated，实际 %s", got)
	}
	if got := readLiveCred(t, s, "kimi"); got != `{"token":"BBB"}` {
		t.Fatalf("切换后 live 应为 BBB，实际 %s", got)
	}

	// 幂等：再次切 personal → NoOp
	res2, err := s.switchTo("kimi", "personal")
	if err != nil || res2 == nil || !res2.NoOp {
		t.Fatalf("重复切换应为 no-op: %+v %v", res2, err)
	}

	// personal → work：恢复轮换后的 AAA
	if _, err := s.switchTo("kimi", "work"); err != nil {
		t.Fatalf("switch work: %v", err)
	}
	if got := readLiveCred(t, s, "kimi"); got != `{"token":"AAA-rotated"}` {
		t.Fatalf("切回 work 后 live 应为 AAA-rotated，实际 %s", got)
	}

	// freshness：live 与快照一致
	if st, _ := s.Freshness("kimi", "work"); st != "fresh" {
		t.Fatalf("freshness 应为 fresh，实际 %s", st)
	}
	writeLive(t, s, "kimi", `{"token":"XXX"}`)
	if st, _ := s.Freshness("kimi", "work"); st != "rotated" {
		t.Fatalf("freshness 应为 rotated，实际 %s", st)
	}
	os.Remove(mustDefPath(t, s, "kimi"))
	if st, _ := s.Freshness("kimi", "work"); st != "missing-live" {
		t.Fatalf("freshness 应为 missing-live，实际 %s", st)
	}
}

func mustDefPath(t *testing.T, s *Store, cliID string) string {
	t.Helper()
	def, err := s.Def(cliID)
	if err != nil {
		t.Fatal(err)
	}
	return def.Source.Path
}

// live 凭据缺失时切换：跳过回写但允许激活目标。
func TestSwitchWithMissingLive(t *testing.T) {
	s := newTestStore(t)
	writeLive(t, s, "kimi", "AAA")
	_ = s.capture("kimi", "a")
	_, _ = s.switchTo("kimi", "a")

	writeLive(t, s, "kimi", "BBB")
	_ = s.capture("kimi", "b")

	os.Remove(mustDefPath(t, s, "kimi")) // 登出
	res, err := s.switchTo("kimi", "b")
	if err != nil {
		t.Fatalf("live 缺失时切换不应失败: %v", err)
	}
	if got := readLiveCred(t, s, "kimi"); got != "BBB" {
		t.Fatalf("live 应恢复为 BBB，实际 %s", got)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("应包含 live 缺失的警告")
	}
	// a 的快照不应被空内容覆盖
	if got := readStoreCred(t, s, "kimi", "a"); got != "AAA" {
		t.Fatalf("a 快照不应被覆盖，实际 %s", got)
	}
}

// 删除 active profile 应清除指针且不动 live。
func TestDeleteActive(t *testing.T) {
	s := newTestStore(t)
	writeLive(t, s, "kimi", "AAA")
	_ = s.capture("kimi", "only")
	_, _ = s.switchTo("kimi", "only")

	if err := s.deleteProfile("kimi", "only"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveProfile("kimi") != "" {
		t.Fatal("删除 active 后指针应清空")
	}
	if got := readLiveCred(t, s, "kimi"); got != "AAA" {
		t.Fatalf("删除 profile 不应影响 live，实际 %s", got)
	}
}

// 同一账号的 token 轮换不算 rotated；live 换成其他账号才算。
func TestFreshnessSameUserRotation(t *testing.T) {
	fakeCreds := func(userID, tag string) string {
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"user_id":"` + userID + `"}`))
		return `{"access_token":"h.` + payload + `.` + tag + `","refresh_token":"r-` + tag + `"}`
	}

	s := newTestStore(t)
	writeLive(t, s, "kimi", fakeCreds("u1", "a"))
	if err := s.capture("kimi", "acc"); err != nil {
		t.Fatal(err)
	}

	// CLI 运行期间轮换 token：同一账号 → fresh
	writeLive(t, s, "kimi", fakeCreds("u1", "b"))
	if st, _ := s.Freshness("kimi", "acc"); st != "fresh" {
		t.Fatalf("同账号轮换应为 fresh，实际 %s", st)
	}

	// live 换成另一个账号 → rotated
	writeLive(t, s, "kimi", fakeCreds("u2", "a"))
	if st, _ := s.Freshness("kimi", "acc"); st != "rotated" {
		t.Fatalf("换成其他账号应为 rotated，实际 %s", st)
	}

	// 身份无法识别时保守视为 rotated
	writeLive(t, s, "kimi", `{"token":"XXX"}`)
	if st, _ := s.Freshness("kimi", "acc"); st != "rotated" {
		t.Fatalf("身份不可识别应为 rotated，实际 %s", st)
	}
}

// recapture 更新快照并保留创建时间。
func TestRecaptureKeepsMeta(t *testing.T) {
	s := newTestStore(t)
	writeLive(t, s, "kimi", "AAA")
	_ = s.capture("kimi", "a")
	metas, _ := s.ListProfiles("kimi")
	created := metas[0].CreatedAt

	writeLive(t, s, "kimi", "AAA2")
	if err := s.recapture("kimi", "a"); err != nil {
		t.Fatal(err)
	}
	metas, _ = s.ListProfiles("kimi")
	if len(metas) != 1 || !metas[0].CreatedAt.Equal(created) {
		t.Fatalf("recapture 应保留创建时间: %+v", metas)
	}
	if got := readStoreCred(t, s, "kimi", "a"); got != "AAA2" {
		t.Fatalf("recapture 应更新快照，实际 %s", got)
	}
}
