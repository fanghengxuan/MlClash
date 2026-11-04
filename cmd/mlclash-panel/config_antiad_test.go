package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func markAntiAdReady(t *testing.T, a *App) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(a.dataDir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	size := antiAdMinBytes
	if err := os.WriteFile(a.antiAdPath(), make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(AntiAdMeta{UpdatedAt: time.Now().Unix(), Source: "test", Size: int64(size)})
	if err := os.WriteFile(a.antiAdMetaPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newAntiAdTestApp(t *testing.T, mode, preset string, withGeo, withFile bool) *App {
	a := newAdBlockTestApp(t, true, mode, preset, withGeo)
	if withFile {
		markAntiAdReady(t, a)
	}
	return a
}

func TestBuildConfigAntiAdFirst(t *testing.T) {
	a := newAntiAdTestApp(t, "rule", "cn-direct", true, true)
	rules := buildRules(t, a)
	if got := rules[0]; got != "RULE-SET,anti-ad,REJECT" {
		t.Fatalf("第一条规则 = %v，应为 anti-AD 规则：%v", got, rules)
	}
	if got := rules[1]; got != "GEOSITE,category-ads-all,REJECT" {
		t.Fatalf("第二条规则 = %v，应为内置广告兜底：%v", got, rules)
	}
	cfg, err := a.BuildConfig()
	if err != nil {
		t.Fatal(err)
	}
	rp, ok := cfg["rule-providers"].(map[string]any)
	if !ok {
		t.Fatal("config 缺少 rule-providers")
	}
	anti, ok := rp["anti-ad"].(map[string]any)
	if !ok {
		t.Fatalf("rule-providers 缺少 anti-ad：%v", rp)
	}
	if anti["format"] != "mrs" || anti["behavior"] != "domain" {
		t.Fatalf("anti-ad 参数不对：%v", anti)
	}
	for _, want := range []string{"GEOSITE,apple-cn,DIRECT", "GEOSITE,google-cn,DIRECT"} {
		if ruleIndex(rules, want) < 0 {
			t.Fatalf("缺少 %s：%v", want, rules)
		}
	}
}

func TestBuildConfigAntiAdOff(t *testing.T) {
	a := newAntiAdTestApp(t, "rule", "cn-direct", true, true)
	a.settings.AdBlock = false
	rules := buildRules(t, a)
	if ruleIndex(rules, "RULE-SET,anti-ad,REJECT") >= 0 {
		t.Fatalf("关闭去广告后不应出现 anti-ad 规则：%v", rules)
	}
	if ruleIndex(rules, "GEOSITE,category-ads-all,REJECT") >= 0 {
		t.Fatalf("关闭去广告后不应出现内置广告规则：%v", rules)
	}
}

func TestBuildConfigAntiAdWithoutFile(t *testing.T) {
	a := newAntiAdTestApp(t, "rule", "cn-direct", true, false)
	rules := buildRules(t, a)
	if ruleIndex(rules, "RULE-SET,anti-ad,REJECT") >= 0 {
		t.Fatalf("anti-ad 文件未就绪时不应写入 anti-ad 规则：%v", rules)
	}
	// 内置广告列表应作为兜底。
	if ruleIndex(rules, "GEOSITE,category-ads-all,REJECT") < 0 {
		t.Fatalf("anti-ad 未就绪时缺少内置广告兜底：%v", rules)
	}
	if ruleIndex(rules, "GEOSITE,cn,DIRECT") < 0 {
		t.Fatalf("基础国内域名规则缺失：%v", rules)
	}
}

func TestBuildConfigAntiAdProxyAll(t *testing.T) {
	a := newAntiAdTestApp(t, "rule", "proxy-all", true, true)
	rules := buildRules(t, a)
	if got := rules[0]; got != "RULE-SET,anti-ad,REJECT" {
		t.Fatalf("proxy-all 第一条规则 = %v，应为 anti-AD 规则：%v", got, rules)
	}
	if got := rules[1]; got != "GEOSITE,category-ads-all,REJECT" {
		t.Fatalf("proxy-all 第二条规则 = %v，应为内置广告兜底：%v", got, rules)
	}
}

func TestBuildConfigAntiAdOnlyInRuleMode(t *testing.T) {
	for _, mode := range []string{"global", "direct"} {
		a := newAntiAdTestApp(t, mode, "cn-direct", true, true)
		rules := buildRules(t, a)
		if ruleIndex(rules, "RULE-SET,anti-ad,REJECT") >= 0 {
			t.Fatalf("%s 模式不应写入增强广告规则：%v", mode, rules)
		}
	}
}
