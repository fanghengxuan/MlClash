package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// newLiteApp 只覆盖这些测试需要的字段；它们不启动内核，不该被集成测试的
// mihomo 依赖跳过。
func newLiteApp(t *testing.T) *App {
	t.Helper()
	a := &App{dataDir: t.TempDir()}
	a.settings = defaultSettings()
	a.settings.RedirPort = freePort(t)
	a.settings.MixedPort = freePort(t)
	a.settings.APIPort = freePort(t)
	a.logRing = newRingBuffer(100)
	_ = os.MkdirAll(filepath.Join(a.dataDir, "providers"), 0o755)
	return a
}

func TestAutoExcludeFilter(t *testing.T) {
	a := newLiteApp(t)
	a.mu.Lock()
	a.subs = []Sub{{ID: "s1", Enabled: true, NodeCount: 3}}
	a.mu.Unlock()

	// 没有质量数据时不排除
	if got := a.autoExcludeFilter(); got != "" {
		t.Errorf("没有 risky 节点时应返回空，实际 %q", got)
	}

	a.qualityMu.Lock()
	a.quality.Items = map[string]NodeQuality{
		"[机场A] 日本 01": {Level: "risky"},
		"[机场A] 香港 01": {Level: "good"},
		"[机场A] 美国 01": {Level: "normal"},
	}
	a.qualityMu.Unlock()

	got := a.autoExcludeFilter()
	want := "^(?:" + regexp.QuoteMeta("[机场A] 日本 01") + ")$"
	if got != want {
		t.Errorf("排除正则不对：want %q got %q", want, got)
	}
	// 不能因子串相同误排除带后缀的节点
	re := regexp.MustCompile(got)
	if re.MatchString("[机场A] 日本 01 IPv6") {
		t.Errorf("排除正则不应匹配带后缀的节点：%q", got)
	}
	if !re.MatchString("[机场A] 日本 01") {
		t.Errorf("排除正则应匹配完整节点名：%q", got)
	}

	// 全部节点都 risky 时不能排除，否则自动选择组会为空
	a.mu.Lock()
	a.subs = []Sub{{ID: "s1", Enabled: true, NodeCount: 3}}
	a.mu.Unlock()
	a.qualityMu.Lock()
	a.quality.Items = map[string]NodeQuality{
		"n1": {Level: "risky"},
		"n2": {Level: "risky"},
		"n3": {Level: "risky"},
	}
	a.qualityMu.Unlock()
	if got := a.autoExcludeFilter(); got != "" {
		t.Errorf("全部 risky 时应返回空，实际 %q", got)
	}
}

func TestQualityPersistence(t *testing.T) {
	a := newLiteApp(t)
	a.qualityMu.Lock()
	a.quality = qualityStore{
		UpdatedAt: 1700000000,
		Items: map[string]NodeQuality{
			"n1": {Level: "risky", Type: "hosting", IP: "1.2.3.4", At: 1700000000},
		},
	}
	a.saveQualityLocked()
	a.qualityMu.Unlock()

	// 清空内存后重新加载
	a.qualityMu.Lock()
	a.quality = qualityStore{}
	a.qualityMu.Unlock()
	a.loadQuality()

	q, ok := a.qualityOf("n1")
	if !ok || q.Level != "risky" || q.IP != "1.2.3.4" {
		t.Fatalf("质量结果未正确持久化：ok=%v q=%+v", ok, q)
	}
	if a.quality.UpdatedAt != 1700000000 {
		t.Errorf("更新时间丢失：%d", a.quality.UpdatedAt)
	}
}

func TestBuildConfigQualityWiring(t *testing.T) {
	a := newLiteApp(t)
	a.mu.Lock()
	a.subs = []Sub{{ID: "s1", Name: "机场A", Enabled: true, NodeCount: 3}}
	a.mu.Unlock()
	writeFakeProvider(t, a, "s1")
	a.qualityMu.Lock()
	a.quality.Items = map[string]NodeQuality{
		"[机场A] n1": {Level: "risky"},
		"[机场A] n2": {Level: "good"},
	}
	a.qualityMu.Unlock()

	cfg, err := a.BuildConfig()
	if err != nil {
		t.Fatalf("生成配置失败：%v", err)
	}

	listeners, ok := cfg["listeners"].([]any)
	if !ok || len(listeners) != 1 {
		t.Fatalf("应生成一个质量检测 listener：%v", cfg["listeners"])
	}
	l := listeners[0].(map[string]any)
	if l["proxy"] != groupQuality || l["port"] != a.qualityPort() {
		t.Errorf("listener 配置不对：%v", l)
	}

	var auto, quality map[string]any
	for _, g := range cfg["proxy-groups"].([]any) {
		m := g.(map[string]any)
		switch m["name"] {
		case groupAuto:
			auto = m
		case groupQuality:
			quality = m
		}
	}
	if auto == nil || auto["exclude-filter"] == nil {
		t.Errorf("自动选择组应带 exclude-filter：%v", auto)
	}
	if quality == nil || quality["hidden"] != true {
		t.Errorf("质量检测组应为 hidden：%v", quality)
	}
}
