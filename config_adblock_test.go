package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newAdBlockTestApp(t *testing.T, adBlock bool, mode, preset string, withGeo bool) *App {
	t.Helper()
	dir := t.TempDir()
	a := &App{
		dataDir: dir,
		settings: Settings{
			Mode: mode, RulePreset: preset, AdBlock: adBlock,
			MixedPort: 7890, RedirPort: 3333, APIPort: 9090, LogLevel: "info",
		},
	}
	a.subs = []Sub{{ID: "s1", Name: "test", Enabled: true}}
	for _, d := range []string{"providers", "rules"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(a.providerPath("s1"), []byte(`{"proxies":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ruleSetPath(), []byte("1.0.1.0/24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if withGeo {
		// geoSiteReady 要求文件大小 >= 1MB 且 stamp 与文件签名一致。
		if err := os.WriteFile(a.geoSitePath(), make([]byte, geoSiteMinBytes), 0o644); err != nil {
			t.Fatal(err)
		}
		sig := fileSignature(a.geoSitePath())
		if err := os.WriteFile(a.geoSiteStampPath(), []byte(sig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func buildRules(t *testing.T, a *App) []any {
	t.Helper()
	cfg, err := a.BuildConfig()
	if err != nil {
		t.Fatal(err)
	}
	rules, ok := cfg["rules"].([]any)
	if !ok {
		t.Fatal("config 缺少 rules")
	}
	return rules
}

func ruleIndex(rules []any, want string) int {
	for i, r := range rules {
		if s, ok := r.(string); ok && s == want {
			return i
		}
	}
	return -1
}

func TestBuildConfigAdBlockFirst(t *testing.T) {
	a := newAdBlockTestApp(t, true, "rule", "cn-direct", true)
	rules := buildRules(t, a)
	if got := rules[0]; got != "GEOSITE,category-ads-all,REJECT" {
		t.Fatalf("第一条规则 = %v，应为去广告规则：%v", got, rules)
	}
	ad := ruleIndex(rules, "GEOSITE,category-ads-all,REJECT")
	cn := ruleIndex(rules, "GEOSITE,cn,DIRECT")
	if cn < 0 {
		t.Fatalf("缺少 GEOSITE,cn,DIRECT：%v", rules)
	}
	if ad > cn {
		t.Fatalf("去广告规则应在国内域名直连之前：%v", rules)
	}
}

func TestBuildConfigAdBlockOff(t *testing.T) {
	a := newAdBlockTestApp(t, false, "rule", "cn-direct", true)
	rules := buildRules(t, a)
	if ruleIndex(rules, "GEOSITE,category-ads-all,REJECT") >= 0 {
		t.Fatalf("关闭去广告后不应出现拦截规则：%v", rules)
	}
}

func TestBuildConfigAdBlockWithoutGeoSite(t *testing.T) {
	a := newAdBlockTestApp(t, true, "rule", "cn-direct", false)
	rules := buildRules(t, a)
	if ruleIndex(rules, "GEOSITE,category-ads-all,REJECT") >= 0 {
		t.Fatalf("geosite.dat 未就绪时不应写入 GEOSITE 规则：%v", rules)
	}
}

func TestBuildConfigAdBlockProxyAll(t *testing.T) {
	a := newAdBlockTestApp(t, true, "rule", "proxy-all", true)
	rules := buildRules(t, a)
	if got := rules[0]; got != "GEOSITE,category-ads-all,REJECT" {
		t.Fatalf("proxy-all 下第一条规则 = %v，应为去广告规则：%v", got, rules)
	}
	last, _ := rules[len(rules)-1].(string)
	if !strings.HasPrefix(last, "MATCH,") {
		t.Fatalf("最后一条规则应为 MATCH：%v", rules)
	}
}

func TestBuildConfigAdBlockOnlyInRuleMode(t *testing.T) {
	for _, mode := range []string{"global", "direct"} {
		a := newAdBlockTestApp(t, true, mode, "cn-direct", true)
		rules := buildRules(t, a)
		if ruleIndex(rules, "GEOSITE,category-ads-all,REJECT") >= 0 {
			t.Fatalf("%s 模式不应写入去广告规则：%v", mode, rules)
		}
	}
}

func TestApplyPatchAdBlock(t *testing.T) {
	s := defaultSettings()
	if s.AdBlock {
		t.Fatal("默认应为关闭")
	}
	if err := s.applyPatch(map[string]any{"adBlock": true}); err != nil {
		t.Fatal(err)
	}
	if !s.AdBlock {
		t.Fatal("adBlock 未生效")
	}
	if err := s.applyPatch(map[string]any{"adBlock": false}); err != nil {
		t.Fatal(err)
	}
	if s.AdBlock {
		t.Fatal("adBlock 未关闭")
	}
}

func TestPublicSettingsAdBlock(t *testing.T) {
	got := publicSettings(Settings{AdBlock: true})
	if got["adBlock"] != true {
		t.Fatalf("publicSettings 未暴露 adBlock：%v", got)
	}
}
