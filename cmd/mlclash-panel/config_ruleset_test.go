package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureRuleProviderYAML(t *testing.T) {
	dir := t.TempDir()
	a := &App{dataDir: dir}
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "1.0.1.0/24\n# comment\n\n223.255.252.0/23\nnot-a-cidr\n"
	if err := os.WriteFile(a.ruleSetPath(), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if !a.ensureRuleProviderYAML() {
		t.Fatal("ensureRuleProviderYAML 返回 false")
	}
	b, err := os.ReadFile(a.ruleProviderPath())
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.HasPrefix(got, "payload:\n") {
		t.Fatalf("缺少 payload 头：%q", got)
	}
	for _, want := range []string{`"1.0.1.0/24"`, `"223.255.252.0/23"`} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少规则 %s：%q", want, got)
		}
	}
	for _, bad := range []string{"# comment", "not-a-cidr"} {
		if strings.Contains(got, bad) {
			t.Errorf("不应包含 %q：%q", bad, got)
		}
	}
}

func TestBuildConfigUsesYAMLRuleProvider(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		dataDir: dir,
		settings: Settings{
			Mode: "rule", RulePreset: "cn-direct",
			MixedPort: 7890, RedirPort: 3333, APIPort: 9090, LogLevel: "info",
		},
	}
	a.subs = []Sub{{ID: "s1", Name: "test", Enabled: true}}
	if err := os.MkdirAll(filepath.Join(dir, "providers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.providerPath("s1"), []byte(`{"proxies":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ruleSetPath(), []byte("1.0.1.0/24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := a.BuildConfig()
	if err != nil {
		t.Fatal(err)
	}
	rp, ok := cfg["rule-providers"].(map[string]any)
	if !ok {
		t.Fatal("config 缺少 rule-providers")
	}
	cn, ok := rp["cn-ip"].(map[string]any)
	if !ok {
		t.Fatal("rule-providers 缺少 cn-ip")
	}
	if got := cn["path"]; got != a.ruleProviderPath() {
		t.Fatalf("cn-ip path = %v，应为 %s", got, a.ruleProviderPath())
	}
	rules, _ := cfg["rules"].([]any)
	found := false
	for _, r := range rules {
		if r == "RULE-SET,cn-ip,DIRECT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("rules 缺少 RULE-SET,cn-ip,DIRECT：%v", rules)
	}
}
