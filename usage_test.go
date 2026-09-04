package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseKimiUsageDSHFixture(t *testing.T) {
	body := []byte(`{
		"plan": "Coding Pro",
		"limits": [{ "detail": { "limit": 1000, "remaining": 750, "resetTime": "2026-08-14T05:00:00Z" } }],
		"usage": { "limit": 10000, "remaining": 6000, "resetTime": "2026-08-17T00:00:00Z" }
	}`)
	u, err := parseKimiUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.Status != "ok" || u.Plan != "Coding Pro" {
		t.Fatalf("status/plan: %+v", u)
	}
	if u.Session == nil || u.Session.UsedPercent != 25 || u.Session.RemainingPercent != 75 {
		t.Fatalf("session: %+v", u.Session)
	}
	if u.Weekly == nil || u.Weekly.UsedPercent != 40 || u.Weekly.RemainingPercent != 60 {
		t.Fatalf("weekly: %+v", u.Weekly)
	}
	if u.Session.ResetsAt != "2026-08-14T05:00:00Z" {
		t.Fatalf("session reset: %s", u.Session.ResetsAt)
	}
}

func TestParseKimiUsageNestedDataAndStringNumbers(t *testing.T) {
	body := []byte(`{
		"data": {
			"subType": "Allegro",
			"limits": [{
				"window": { "duration": 300, "timeUnit": "TIME_UNIT_MINUTE" },
				"detail": { "limit": "200", "remaining": "50" }
			}],
			"usage": { "limit": "1000", "used": "200" }
		}
	}`)
	u, err := parseKimiUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.Plan != "Allegro" {
		t.Fatalf("plan: %s", u.Plan)
	}
	if u.Session == nil || u.Session.UsedPercent != 75 {
		t.Fatalf("session: %+v", u.Session)
	}
	if u.Weekly == nil || u.Weekly.UsedPercent != 20 {
		t.Fatalf("weekly: %+v", u.Weekly)
	}
}

func TestParseKimiUsagePlanEnumFallsBack(t *testing.T) {
	body := []byte(`{
		"plan": "TYPE_PURCHASE",
		"usage": { "limit": 1000, "remaining": 500 }
	}`)
	u, err := parseKimiUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.Plan != "Kimi For Coding" {
		t.Fatalf("plan: %s", u.Plan)
	}
}

func TestParseKimiUsageEmptyFiveHourWindow(t *testing.T) {
	body := []byte(`{
		"limits": [{ "window": { "duration": 300, "timeUnit": "TIME_UNIT_MINUTE" } }],
		"usage": { "limit": 10000, "remaining": 9000, "resetTime": "2026-08-17T00:00:00Z" }
	}`)
	u, err := parseKimiUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.Session == nil || u.Session.UsedPercent != 0 || u.Session.RemainingPercent != 100 {
		t.Fatalf("empty session should be 0%%: %+v", u.Session)
	}
	if u.Weekly == nil || u.Weekly.UsedPercent != 10 {
		t.Fatalf("weekly: %+v", u.Weekly)
	}
}

func TestParseKimiUsageWeeklyFromLimitsDuration(t *testing.T) {
	body := []byte(`{
		"limits": [
			{
				"window": { "duration": 300, "timeUnit": "TIME_UNIT_MINUTE" },
				"detail": { "limit": 100, "remaining": 40 }
			},
			{
				"window": { "duration": 10080, "timeUnit": "TIME_UNIT_MINUTE" },
				"detail": { "limit": 1000, "remaining": 250 }
			}
		]
	}`)
	u, err := parseKimiUsage(body)
	if err != nil {
		t.Fatal(err)
	}
	if u.Session == nil || u.Session.UsedPercent != 60 {
		t.Fatalf("session: %+v", u.Session)
	}
	if u.Weekly == nil || u.Weekly.UsedPercent != 75 {
		t.Fatalf("weekly: %+v", u.Weekly)
	}
}

func TestFormatRemain(t *testing.T) {
	if got := formatRemain(-time.Second); got != "已重置" {
		t.Fatalf("negative: %s", got)
	}
	if got := formatRemain(3 * time.Minute); got != "3分钟" {
		t.Fatalf("minutes: %s", got)
	}
	if got := formatRemain(2*time.Hour + 5*time.Minute); got != "2小时5分" {
		t.Fatalf("hours: %s", got)
	}
	if got := formatRemain(26 * time.Hour); got != "1天2小时" {
		t.Fatalf("days: %s", got)
	}
}

func TestFetchKimiUsageHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"plan": "Pro",
			"limits": [{ "detail": { "limit": 100, "remaining": 80 } }],
			"usage": { "limit": 1000, "remaining": 500 }
		}`))
	}))
	defer ts.Close()

	u, err := fetchKimiUsage(ts.Client(), t.Context(), ts.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if u.Status != "ok" || u.Session.UsedPercent != 20 || u.Weekly.UsedPercent != 50 {
		t.Fatalf("unexpected: %+v", u)
	}

	_, err = fetchKimiUsage(ts.Client(), t.Context(), ts.URL, "bad")
	if !isAuthErr(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestRefreshKimiToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "rt-old" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Form.Get("client_id") != kimiOAuthClientID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-new",
			"refresh_token": "rt-new",
			"expires_in":    900,
			"token_type":    "Bearer",
			"scope":         "kimi-code",
		})
	}))
	defer ts.Close()

	next, err := refreshKimiToken(ts.Client(), ts.URL, "rt-old")
	if err != nil {
		t.Fatal(err)
	}
	if next.AccessToken != "at-new" || next.RefreshToken != "rt-new" || next.ExpiresIn != 900 {
		t.Fatalf("unexpected token: %+v", next)
	}
}

func TestFetchUsageFromFileRefreshesExpiredToken(t *testing.T) {
	var usageAuth string
	oauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "fresh-at",
			"refresh_token": "fresh-rt",
			"expires_in":    900,
		})
	}))
	defer oauth.Close()
	usage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usageAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"limits": []any{map[string]any{"detail": map[string]any{"limit": 100, "remaining": 90}}},
			"usage":  map[string]any{"limit": 1000, "remaining": 800},
		})
	}))
	defer usage.Close()

	oldUsage, oldOAuth := kimiUsageURL, kimiOAuthURL
	kimiUsageURL, kimiOAuthURL = usage.URL, oauth.URL
	defer func() { kimiUsageURL, kimiOAuthURL = oldUsage, oldOAuth }()

	dir := t.TempDir()
	path := filepath.Join(dir, "kimi-code.json")
	expired := map[string]any{
		"access_token":  "stale-at",
		"refresh_token": "rt-old",
		"expires_at":    time.Now().Unix() - 10,
		"scope":         "kimi-code",
		"token_type":    "Bearer",
	}
	raw, _ := json.Marshal(expired)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	app := &App{httpc: http.DefaultClient, usage: newUsageCache()}
	got := app.fetchUsageFromFile(path, nil)
	if got.Status != "ok" {
		t.Fatalf("status: %+v", got)
	}
	if usageAuth != "Bearer fresh-at" {
		t.Fatalf("did not use refreshed token: %s", usageAuth)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "fresh-at") || !strings.Contains(string(saved), "fresh-rt") {
		t.Fatalf("did not write refreshed tokens: %s", saved)
	}
}

func TestResolveStoreRootMigratesOldDir(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, oldStoreRootName)
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(old, "config.json")
	if err := os.WriteFile(marker, []byte(`{"version":1,"active":{"kimi":"work"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewStoreAt(home)
	want := filepath.Join(home, storeRootName)
	if s.Root != want {
		t.Fatalf("root %s, want %s", s.Root, want)
	}
	if _, err := os.Stat(filepath.Join(want, "config.json")); err != nil {
		t.Fatalf("migrated config missing: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old dir should be gone, err=%v", err)
	}
}

func TestCliDefsKimiOnly(t *testing.T) {
	s := NewStoreAt(t.TempDir())
	if len(s.Defs) != 1 || s.Defs[0].ID != "kimi" {
		t.Fatalf("expected only kimi, got %+v", s.Defs)
	}
}
