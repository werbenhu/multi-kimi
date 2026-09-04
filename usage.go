package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Kimi Coding Plan 用量接口，解析口径对齐
// https://github.com/Ychris12138/dsh-usage-stats（kimi-token-plan）：
//
//	GET https://api.kimi.com/coding/v1/usages
//	Authorization: Bearer <access_token 或 sk-kimi- API key>
//	limits[].detail  → 5 小时滚动窗口
//	usage            → 周用量
//	已用率 = (limit - remaining) / limit * 100
const (
	kimiUsageURLDefault = "https://api.kimi.com/coding/v1/usages"
	kimiOAuthURLDefault = "https://auth.kimi.com/api/oauth/token"
	kimiOAuthClientID   = "17e5f671-d194-4dfb-9706-5516cb48c098"
	usageCacheTTL       = 90 * time.Second
	usageHTTPTimeout    = 12 * time.Second
	tokenRefreshLeeway  = 60 * time.Second
)

// 可在测试中覆盖。
var (
	kimiUsageURL = kimiUsageURLDefault
	kimiOAuthURL = kimiOAuthURLDefault
)

// UsageWindow 单个额度窗口（5 小时或周）。
type UsageWindow struct {
	Kind             string  `json:"kind"` // session | weekly
	Label            string  `json:"label"`
	UsedPercent      float64 `json:"usedPercent"`
	RemainingPercent float64 `json:"remainingPercent"`
	ResetsAt         string  `json:"resetsAt,omitempty"`
	ResetsIn         string  `json:"resetsIn,omitempty"`
}

// KimiUsage 某个账号的 Coding Plan 用量。
type KimiUsage struct {
	Status    string       `json:"status"` // ok | loading | missing | unauthorized | unavailable | empty
	Plan      string       `json:"plan,omitempty"`
	Session   *UsageWindow `json:"session,omitempty"`
	Weekly    *UsageWindow `json:"weekly,omitempty"`
	Error     string       `json:"error,omitempty"`
	FetchedAt string       `json:"fetchedAt,omitempty"`
}

type usageCache struct {
	mu      sync.Mutex
	entries map[string]usageEntry
}

type usageEntry struct {
	usage     *KimiUsage
	fetchedAt time.Time
}

func newUsageCache() *usageCache {
	return &usageCache{entries: map[string]usageEntry{}}
}

func (c *usageCache) get(key string) *KimiUsage {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil
	}
	return cloneUsage(e.usage)
}

func (c *usageCache) put(key string, u *KimiUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if u == nil {
		delete(c.entries, key)
		return
	}
	c.entries[key] = usageEntry{usage: cloneUsage(u), fetchedAt: time.Now()}
}

func (c *usageCache) fresh(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || e.usage == nil {
		return false
	}
	return time.Since(e.fetchedAt) < usageCacheTTL
}

func (c *usageCache) invalidate(keys ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		delete(c.entries, k)
	}
}

func cloneUsage(u *KimiUsage) *KimiUsage {
	if u == nil {
		return nil
	}
	out := *u
	if u.Session != nil {
		s := *u.Session
		out.Session = &s
	}
	if u.Weekly != nil {
		w := *u.Weekly
		out.Weekly = &w
	}
	return &out
}

func usageKeyLive(cliID string) string { return cliID + "/live" }

func usageKeyProfile(cliID, name string) string { return cliID + "/profile/" + name }

// RefreshUsage 强制刷新全部 Kimi 账号的周用量 / 5 小时用量。
func (a *App) RefreshUsage() error {
	return a.refreshAllUsage(true)
}

func (a *App) maybeRefreshUsage() {
	if a.usage == nil {
		return
	}
	if a.usageNeedsRefresh() {
		go func() {
			_ = a.refreshAllUsage(false)
		}()
	}
}

func (a *App) usageNeedsRefresh() bool {
	s, err := a.mustStore()
	if err != nil {
		return false
	}
	def, err := s.Def("kimi")
	if err != nil {
		return false
	}
	if fileExists(def.Source.Path) && !a.usage.fresh(usageKeyLive("kimi")) {
		return true
	}
	metas, err := s.ListProfiles("kimi")
	if err != nil {
		return false
	}
	active := s.ActiveProfile("kimi")
	for _, m := range metas {
		if m.Name == active && fileExists(def.Source.Path) {
			continue
		}
		if !fileExists(credPath(s.profileDir("kimi", m.Name), def)) {
			continue
		}
		if !a.usage.fresh(usageKeyProfile("kimi", m.Name)) {
			return true
		}
	}
	return false
}

func (a *App) refreshAllUsage(force bool) error {
	a.usageRefreshMu.Lock()
	defer a.usageRefreshMu.Unlock()

	if !force && !a.usageNeedsRefresh() {
		a.emitUsageUpdated()
		return nil
	}

	s, err := a.mustStore()
	if err != nil {
		return err
	}
	def, err := s.Def("kimi")
	if err != nil {
		return err
	}

	type job struct {
		key        string
		path       string
		extraWrite []string
	}
	var jobs []job

	livePath := def.Source.Path
	liveOK := fileExists(livePath)
	active := s.ActiveProfile("kimi")
	if liveOK {
		extra := []string{}
		if active != "" {
			p := credPath(s.profileDir("kimi", active), def)
			if fileExists(p) {
				extra = append(extra, p)
			}
		}
		jobs = append(jobs, job{key: usageKeyLive("kimi"), path: livePath, extraWrite: extra})
	}

	metas, _ := s.ListProfiles("kimi")
	for _, m := range metas {
		p := credPath(s.profileDir("kimi", m.Name), def)
		if !fileExists(p) {
			a.usage.put(usageKeyProfile("kimi", m.Name), &KimiUsage{
				Status: "missing",
				Error:  "快照缺失",
			})
			continue
		}
		if liveOK && m.Name == active {
			continue
		}
		if !force && a.usage.fresh(usageKeyProfile("kimi", m.Name)) {
			continue
		}
		jobs = append(jobs, job{key: usageKeyProfile("kimi", m.Name), path: p})
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, j := range jobs {
		if !force && a.usage.fresh(j.key) {
			continue
		}
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			u := a.fetchUsageFromFile(j.path, j.extraWrite)
			a.usage.put(j.key, u)
			if j.key == usageKeyLive("kimi") && active != "" {
				a.usage.put(usageKeyProfile("kimi", active), u)
			}
		}()
	}
	wg.Wait()
	a.emitUsageUpdated()
	_ = a.refreshTrayMenu()
	return nil
}

func (a *App) emitUsageUpdated() {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "usage-updated", nil)
	}
}

func (a *App) attachUsage(views []CliView) {
	if a.usage == nil {
		return
	}
	for i := range views {
		v := &views[i]
		if v.ID != "kimi" {
			continue
		}
		if u := a.usage.get(usageKeyLive("kimi")); u != nil {
			v.LiveUsage = u
		} else if v.LiveExists {
			v.LiveUsage = &KimiUsage{Status: "loading"}
		}
		for j := range v.Profiles {
			p := &v.Profiles[j]
			if p.IsActive && v.LiveUsage != nil {
				p.Usage = cloneUsage(v.LiveUsage)
				continue
			}
			if u := a.usage.get(usageKeyProfile("kimi", p.Name)); u != nil {
				p.Usage = u
			} else if p.HasCredential {
				p.Usage = &KimiUsage{Status: "loading"}
			}
		}
	}
}

func (a *App) invalidateLiveAndProfile(cliID, name string) {
	if a.usage == nil {
		return
	}
	keys := []string{usageKeyLive(cliID)}
	if name != "" {
		keys = append(keys, usageKeyProfile(cliID, name))
	}
	a.usage.invalidate(keys...)
}

func (a *App) fetchUsageFromFile(path string, extraWrite []string) *KimiUsage {
	now := time.Now()
	stamp := func(u *KimiUsage) *KimiUsage {
		if u == nil {
			u = &KimiUsage{Status: "unavailable", Error: "未知错误"}
		}
		u.FetchedAt = now.Format(time.RFC3339)
		return u
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return stamp(&KimiUsage{Status: "missing", Error: "凭据文件不存在"})
	}

	raw, creds, err := parseKimiCreds(data)
	if err != nil {
		return stamp(&KimiUsage{Status: "unavailable", Error: "凭据文件无法解析"})
	}
	token := strings.TrimSpace(creds.AccessToken)
	if token == "" {
		return stamp(&KimiUsage{Status: "missing", Error: "凭据中没有 access_token"})
	}

	writeBack := func(next []byte) {
		a.writeCredsIfUnchanged(path, data, next)
		for _, extra := range extraWrite {
			if extra != "" && extra != path {
				a.writeCredsIfUnchanged(extra, data, next)
			}
		}
	}

	if !looksLikeAPIKey(token) && creds.needsRefresh() && creds.RefreshToken != "" {
		refreshed, newData, rerr := a.refreshAndEncode(raw, creds.RefreshToken)
		if rerr != nil {
			// 预刷新失败时仍尝试旧 token，可能还没过期
		} else {
			token = refreshed.AccessToken
			writeBack(newData)
			creds = refreshed
			raw = newData
		}
	}

	usage, ferr := a.doFetchUsage(token)
	if ferr != nil && isAuthErr(ferr) && !looksLikeAPIKey(token) && creds.RefreshToken != "" {
		refreshed, newData, rerr := a.refreshAndEncode(raw, creds.RefreshToken)
		if rerr == nil {
			writeBack(newData)
			usage, ferr = a.doFetchUsage(refreshed.AccessToken)
		} else {
			ferr = rerr
		}
	}
	if ferr != nil {
		return stamp(usageFromError(ferr))
	}
	return stamp(usage)
}

func (a *App) refreshAndEncode(original []byte, refreshToken string) (kimiCreds, []byte, error) {
	next, err := refreshKimiToken(a.httpClient(), kimiOAuthURL, refreshToken)
	if err != nil {
		return kimiCreds{}, nil, err
	}
	encoded, err := applyRefreshedTokens(original, next)
	if err != nil {
		return kimiCreds{}, nil, err
	}
	return next, encoded, nil
}

func (a *App) doFetchUsage(token string) (*KimiUsage, error) {
	parent := context.Background()
	if a.ctx != nil {
		parent = a.ctx
	}
	ctx, cancel := context.WithTimeout(parent, usageHTTPTimeout)
	defer cancel()
	return fetchKimiUsage(a.httpClient(), ctx, kimiUsageURL, token)
}

func (a *App) writeCredsIfUnchanged(path string, original, next []byte) {
	if a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if !bytes.Equal(current, original) {
		return
	}
	_ = atomicWriteFile(path, next, 0o600)
}

func (a *App) httpClient() *http.Client {
	if a.httpc != nil {
		return a.httpc
	}
	return &http.Client{Timeout: usageHTTPTimeout}
}

type kimiCreds struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
	ExpiresIn    int
	TokenType    string
	Scope        string
}

func (c kimiCreds) needsRefresh() bool {
	if c.ExpiresAt <= 0 {
		return false
	}
	return time.Now().Unix() >= c.ExpiresAt-int64(tokenRefreshLeeway.Seconds())
}

func parseKimiCreds(data []byte) ([]byte, kimiCreds, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return data, kimiCreds{}, err
	}
	c := kimiCreds{
		AccessToken:  strings.TrimSpace(asString(raw["access_token"])),
		RefreshToken: strings.TrimSpace(asString(raw["refresh_token"])),
		TokenType:    asString(raw["token_type"]),
		Scope:        asString(raw["scope"]),
	}
	if c.AccessToken == "" {
		c.AccessToken = strings.TrimSpace(asString(raw["api_key"]))
	}
	if v, ok := asFloat(raw["expires_at"]); ok {
		c.ExpiresAt = unixSeconds(v)
	}
	if v, ok := asFloat(raw["expires_in"]); ok {
		c.ExpiresIn = int(v)
	}
	return data, c, nil
}

func applyRefreshedTokens(original []byte, next kimiCreds) ([]byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(original, &raw); err != nil || raw == nil {
		raw = map[string]any{}
	}
	raw["access_token"] = next.AccessToken
	if next.RefreshToken != "" {
		raw["refresh_token"] = next.RefreshToken
	}
	if next.ExpiresIn > 0 {
		raw["expires_in"] = next.ExpiresIn
		raw["expires_at"] = time.Now().Unix() + int64(next.ExpiresIn)
	} else if next.ExpiresAt > 0 {
		raw["expires_at"] = next.ExpiresAt
	}
	if next.TokenType != "" {
		raw["token_type"] = next.TokenType
	}
	if next.Scope != "" {
		raw["scope"] = next.Scope
	}
	return json.MarshalIndent(raw, "", "  ")
}

func looksLikeAPIKey(token string) bool {
	return strings.HasPrefix(token, "sk-")
}

// credsUserID 从凭据 JSON 中提取账号身份（access_token JWT 的 user_id / sub），
// 用于区分「同一账号的 token 轮换」与「live 换成了另一个账号」。提取失败返回空串。
func credsUserID(data []byte) string {
	_, creds, err := parseKimiCreds(data)
	if err != nil {
		return ""
	}
	return jwtUserID(creds.AccessToken)
}

// jwtUserID 不解签名地解析 JWT payload，取 user_id 或 sub。
func jwtUserID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	if v := asString(claims["user_id"]); v != "" {
		return v
	}
	return asString(claims["sub"])
}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

func isAuthErr(err error) bool {
	_, ok := err.(*authError)
	return ok
}

func usageFromError(err error) *KimiUsage {
	if isAuthErr(err) {
		return &KimiUsage{Status: "unauthorized", Error: "凭据失效，请重新登录后捕获"}
	}
	return &KimiUsage{Status: "unavailable", Error: err.Error()}
}

func fetchKimiUsage(client *http.Client, ctx context.Context, usageURL, token string) (*KimiUsage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "multi-kimi/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("用量请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &authError{msg: fmt.Sprintf("Kimi 用量接口返回 HTTP %d", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Kimi 用量接口返回 HTTP %d", resp.StatusCode)
	}
	u, err := parseKimiUsage(body)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func refreshKimiToken(client *http.Client, oauthURL, refreshToken string) (kimiCreds, error) {
	form := url.Values{}
	form.Set("client_id", kimiOAuthClientID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	ctx, cancel := context.WithTimeout(context.Background(), usageHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return kimiCreds{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "multi-kimi/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return kimiCreds{}, fmt.Errorf("刷新 Kimi token 失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return kimiCreds{}, &authError{msg: "refresh_token 已失效，请重新登录"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return kimiCreds{}, fmt.Errorf("刷新 Kimi token 返回 HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return kimiCreds{}, fmt.Errorf("刷新 token 响应无法解析")
	}
	access := strings.TrimSpace(asString(raw["access_token"]))
	if access == "" {
		return kimiCreds{}, &authError{msg: "刷新 token 未返回 access_token"}
	}
	next := kimiCreds{
		AccessToken:  access,
		RefreshToken: strings.TrimSpace(asString(raw["refresh_token"])),
		TokenType:    asString(raw["token_type"]),
		Scope:        asString(raw["scope"]),
	}
	if next.RefreshToken == "" {
		next.RefreshToken = refreshToken
	}
	if v, ok := asFloat(raw["expires_in"]); ok {
		next.ExpiresIn = int(v)
		next.ExpiresAt = time.Now().Unix() + int64(next.ExpiresIn)
	}
	return next, nil
}

func parseKimiUsage(body []byte) (*KimiUsage, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("用量响应无法解析")
	}
	data, _ := raw.(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("用量响应无法解析")
	}
	if inner, ok := data["data"].(map[string]any); ok {
		data = inner
	}

	plan := firstNonEmpty(
		asString(data["plan"]),
		asString(data["planName"]),
		asString(data["subType"]),
		"Kimi For Coding",
	)
	// 接口有时返回 TYPE_PURCHASE 之类的枚举值，不适合直接展示
	if strings.HasPrefix(plan, "TYPE_") {
		plan = "Kimi For Coding"
	}

	weekly := parseWindow(data["usage"], "weekly", "周用量")

	var session *UsageWindow
	sawEmptySession := false
	if limits, ok := data["limits"].([]any); ok {
		for _, item := range limits {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			kind, label := classifyLimitEntry(m)
			detail := m
			if d, ok := m["detail"].(map[string]any); ok {
				detail = d
			}
			w := parseWindow(detail, kind, label)
			if w == nil {
				if kind == "session" {
					sawEmptySession = true
				}
				continue
			}
			switch w.Kind {
			case "session":
				if session == nil {
					session = w
				}
			case "weekly":
				if weekly == nil {
					weekly = w
				}
			}
		}
	}
	if session == nil && sawEmptySession {
		session = &UsageWindow{Kind: "session", Label: "5小时", RemainingPercent: 100}
	}

	out := &KimiUsage{Plan: plan, Session: session, Weekly: weekly}
	if session == nil && weekly == nil {
		out.Status = "empty"
		out.Error = "未获取到用量（可能不是 Coding Plan）"
		return out, nil
	}
	out.Status = "ok"
	return out, nil
}

func classifyLimitEntry(m map[string]any) (kind, label string) {
	w, _ := m["window"].(map[string]any)
	if w == nil {
		return "session", "5小时"
	}
	dur, ok := asFloat(w["duration"])
	if !ok {
		return "session", "5小时"
	}
	unit := strings.ToUpper(fmt.Sprint(w["timeUnit"]))
	minutes := dur
	switch {
	case strings.Contains(unit, "HOUR"):
		minutes = dur * 60
	case strings.Contains(unit, "DAY"):
		minutes = dur * 1440
	case strings.Contains(unit, "SEC"):
		minutes = dur / 60
	}
	if minutes >= 9000 && minutes <= 12000 { // ~7 天
		return "weekly", "周用量"
	}
	if minutes >= 240 && minutes <= 400 { // ~5 小时
		return "session", "5小时"
	}
	return "session", "5小时"
}

func parseWindow(v any, kind, label string) *UsageWindow {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	limit, hasLimit := asFloat(firstPresent(m, "limit", "total"))
	remaining, hasRem := asFloat(m["remaining"])
	used, hasUsed := asFloat(m["used"])
	if !hasLimit || limit <= 0 {
		return nil
	}
	if !hasRem && hasUsed {
		remaining = limit - used
		hasRem = true
	}
	if !hasRem {
		return nil
	}
	pct := clampPercent((limit - remaining) / limit * 100)
	w := &UsageWindow{
		Kind:             kind,
		Label:            label,
		UsedPercent:      round1(pct),
		RemainingPercent: round1(100 - pct),
		ResetsAt:         asTimeISO(firstPresent(m, "resetTime", "reset_time", "resetsAt")),
	}
	if w.ResetsAt != "" {
		if t, err := time.Parse(time.RFC3339, w.ResetsAt); err == nil {
			w.ResetsIn = formatRemain(time.Until(t))
		}
	}
	return w
}

func formatRemain(d time.Duration) string {
	if d <= 0 {
		return "已重置"
	}
	mins := int(d.Minutes() + 0.5)
	if mins < 1 {
		return "不到1分钟"
	}
	days := mins / (60 * 24)
	hours := (mins / 60) % 24
	m := mins % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d天%d小时", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d小时%d分", hours, m)
	default:
		return fmt.Sprintf("%d分钟", m)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		if t == nil {
			return ""
		}
		return fmt.Sprint(t)
	}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconvParseFloat(strings.TrimSpace(t))
		return f, err == nil
	default:
		return 0, false
	}
}

func strconvParseFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseFloat(s, 64)
}

func unixSeconds(v float64) int64 {
	if v > 1e12 {
		return int64(v / 1000)
	}
	return int64(v)
}

func asTimeISO(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return ""
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
		if f, err := strconvParseFloat(s); err == nil {
			return time.Unix(unixSeconds(f), 0).UTC().Format(time.RFC3339)
		}
		return ""
	default:
		if f, ok := asFloat(t); ok {
			return time.Unix(unixSeconds(f), 0).UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func clampPercent(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func compactUsageText(u *KimiUsage) string {
	if u == nil || u.Status != "ok" {
		return ""
	}
	var parts []string
	if u.Session != nil {
		parts = append(parts, fmt.Sprintf("5小时 %.0f%%", u.Session.UsedPercent))
	}
	if u.Weekly != nil {
		parts = append(parts, fmt.Sprintf("周用量 %.0f%%", u.Weekly.UsedPercent))
	}
	return strings.Join(parts, " · ")
}
