package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGeoFile(t *testing.T, a *App, size int) {
	t.Helper()
	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.geoSitePath(), make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGeoSiteReadyAndConfig(t *testing.T) {
	a := newTestApp(t)
	writeGeoFile(t, a, geoSiteMinBytes+1024)

	if a.geoSiteReady() {
		t.Fatal("没有校验戳时不应视为就绪")
	}
	if err := os.WriteFile(a.geoSiteStampPath(), []byte(fileSignature(a.geoSitePath())), 0o600); err != nil {
		t.Fatal(err)
	}
	if !a.geoSiteReady() {
		t.Fatal("写入匹配戳后应视为就绪")
	}

	// 造一条可用订阅，让 BuildConfig 能生成完整规则
	a.mu.Lock()
	a.subs = []Sub{{ID: "s1", Name: "机场A", Enabled: true, NodeCount: 1}}
	a.mu.Unlock()
	writeFakeProvider(t, a, "s1")
	if err := os.MkdirAll(filepath.Dir(a.ruleSetPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ruleSetPath(), []byte("1.2.3.0/24\n5.6.7.0/24\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := a.BuildConfig()
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}
	if cfg["geodata-loader"] != "memconservative" {
		t.Errorf("geodata-loader 未写入: %v", cfg["geodata-loader"])
	}
	if _, ok := cfg["geox-url"].(map[string]any); !ok {
		t.Error("geox-url 未写入")
	}
	rules := cfg["rules"].([]any)
	if len(rules) == 0 || rules[0] != "GEOSITE,cn,DIRECT" {
		t.Fatalf("域名规则应在第一条，实际 %v", rules)
	}

	// 删掉戳 -> 不应再生成 GEOSITE 规则（避免坏文件让内核启动失败）
	_ = os.Remove(a.geoSiteStampPath())
	cfg2, _ := a.BuildConfig()
	for _, r := range cfg2["rules"].([]any) {
		if strings.HasPrefix(fmt.Sprint(r), "GEOSITE,") {
			t.Fatalf("geosite 未就绪时不应生成 GEOSITE 规则: %v", r)
		}
	}
}

func TestEnsureGeoSiteDownloadAndProbe(t *testing.T) {
	a := newTestApp(t)
	body := make([]byte, geoSiteMinBytes+4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	oldURLs := geoSiteURLs
	geoSiteURLs = []string{srv.URL}
	defer func() { geoSiteURLs = oldURLs }()

	// 假内核：-t 成功并输出 records 数
	script := filepath.Join(a.dataDir, "fake-mihomo")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'Finished initial GeoSite rule cn => DIRECT, records: 123'\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.mihomoPath = script

	if err := a.EnsureGeoSite(true); err != nil {
		t.Fatalf("下载/校验失败: %v", err)
	}
	if !a.geoSiteReady() {
		t.Fatal("成功后应就绪")
	}
	m := a.loadGeoMeta()
	if m.Count != 123 || m.Source != srv.URL || m.Size != int64(len(body)) {
		t.Fatalf("meta 不对: %+v", m)
	}
	if fileExists(a.geoSiteProbePath()) {
		t.Error("probe 文件应被清理")
	}

	// 再强制更新一次且校验失败：应保留旧文件/旧戳
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, geoSiteMinBytes+8))
	}))
	defer bad.Close()
	geoSiteURLs = []string{bad.URL}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.EnsureGeoSite(true); err == nil {
		t.Fatal("校验失败时应返回错误")
	}
	if !a.geoSiteReady() {
		t.Fatal("失败后应保留原来的 geosite.dat 与校验戳")
	}
	if m2 := a.loadGeoMeta(); m2.Count != 123 || m2.Source != srv.URL {
		t.Fatalf("失败后 meta 不应被覆盖: %+v", m2)
	}
}

func writeGeoIPFile(t *testing.T, a *App, body []byte) {
	t.Helper()
	if err := os.WriteFile(a.geoIPPath(), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeMMDB(size int) []byte {
	b := make([]byte, size)
	copy(b[len(b)-len(mmdbMarker):], mmdbMarker)
	return b
}

func TestGeoIPReadyAndConfigOrder(t *testing.T) {
	a := newTestApp(t)
	writeGeoFile(t, a, geoSiteMinBytes+1024)
	if err := os.WriteFile(a.geoSiteStampPath(), []byte(fileSignature(a.geoSitePath())), 0o600); err != nil {
		t.Fatal(err)
	}
	writeGeoIPFile(t, a, fakeMMDB(geoIPMinBytes+1024))
	if err := os.WriteFile(a.geoIPStampPath(), []byte(fileSignature(a.geoIPPath())), 0o600); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.settings.GeoIPEnable = true
	a.subs = []Sub{{ID: "s1", Name: "机场A", Enabled: true, NodeCount: 1}}
	a.mu.Unlock()
	writeFakeProvider(t, a, "s1")
	if err := os.MkdirAll(filepath.Dir(a.ruleSetPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ruleSetPath(), []byte("1.2.3.0/24\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := a.BuildConfig()
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}
	rules := cfg["rules"].([]any)
	want := []string{"GEOSITE,cn,DIRECT", "GEOIP,CN,DIRECT", "RULE-SET,cn-ip,DIRECT"}
	if len(rules) < len(want) {
		t.Fatalf("规则不足: %v", rules)
	}
	for i, w := range want {
		if rules[i] != w {
			t.Fatalf("规则顺序不对，第 %d 条应为 %s，实际 %v", i, w, rules)
		}
	}

	// 关闭开关 -> 不应再生成 GEOIP 规则
	a.mu.Lock()
	a.settings.GeoIPEnable = false
	a.mu.Unlock()
	cfg2, _ := a.BuildConfig()
	for _, r := range cfg2["rules"].([]any) {
		if r == "GEOIP,CN,DIRECT" {
			t.Fatal("开关关闭时不应生成 GEOIP 规则")
		}
	}
}

func TestEnsureGeoIPDownload(t *testing.T) {
	a := newTestApp(t)
	body := fakeMMDB(geoIPMinBytes + 4096)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	old := geoIPURLs
	geoIPURLs = []string{srv.URL}
	defer func() { geoIPURLs = old }()

	if err := a.EnsureGeoIP(true); err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	if !a.geoIPReady() {
		t.Fatal("下载成功后应就绪")
	}
	if m := a.loadGeoIPMeta(); m.Source != srv.URL || m.Size != int64(len(body)) {
		t.Fatalf("meta 不对: %+v", m)
	}

	// 坏文件（不是 MMDB）不能覆盖旧文件
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>error</html>"))
	}))
	defer bad.Close()
	geoIPURLs = []string{bad.URL}
	if err := a.EnsureGeoIP(true); err == nil {
		t.Fatal("坏文件应返回错误")
	}
	if !a.geoIPReady() {
		t.Fatal("失败后应保留旧数据库")
	}
}
