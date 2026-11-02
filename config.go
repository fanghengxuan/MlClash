package main

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const groupMain = "🚀 节点选择"
const groupAuto = "♻️ 自动选择"
const testURL = "http://www.gstatic.com/generate_204"

func (a *App) configPath() string  { return filepath.Join(a.dataDir, "config.json") }
func (a *App) ruleSetPath() string { return filepath.Join(a.dataDir, "rules", "chnroute.txt") }
func (a *App) ruleProviderPath() string {
	return filepath.Join(a.dataDir, "rules", "chnroute.yaml")
}
func (a *App) logPath() string       { return filepath.Join(a.dataDir, "logs", "mihomo.log") }
func (a *App) mihomoAPIPath() string { return "127.0.0.1:9090" }
func (a *App) enabledSubsLocked() []Sub {
	var out []Sub
	for _, s := range a.subs {
		if !s.Enabled {
			continue
		}
		if !fileExists(a.providerPath(s.ID)) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// ensureRuleProviderYAML 把纯文本 chnroute.txt 转成 mihomo 期望的 YAML payload 格式。
// mihomo 对 file + behavior=ipcidr 的规则集按 YamlRule 解析；直接喂纯文本会得到
// ruleCount=0，所有流量都会落到 MATCH 走代理（国内站点因此被拖慢甚至超时）。
func (a *App) ensureRuleProviderYAML() bool {
	src := a.ruleSetPath()
	fi, err := os.Stat(src)
	if err != nil || fi.IsDir() {
		return false
	}
	dst := a.ruleProviderPath()
	if di, err := os.Stat(dst); err == nil && di.Size() > 0 && !di.ModTime().Before(fi.ModTime()) {
		return true
	}

	b, err := os.ReadFile(src)
	if err != nil {
		return false
	}
	var body strings.Builder
	body.WriteString("payload:\n")
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, _, err := net.ParseCIDR(line); err != nil {
			continue
		}
		body.WriteString("  - \"")
		body.WriteString(line)
		body.WriteString("\"\n")
		n++
	}
	if n == 0 {
		return false
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, []byte(body.String()), 0o644); err != nil {
		return false
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return false
	}
	return true
}

// ensureRuleSet 检查国内 IP 列表是否存在；缺失时退化为不做国内直连。
func (a *App) ensureRuleSet() {
	if fileExists(a.ruleSetPath()) {
		return
	}
	// 安装包会把规则放在 /koolshare/mlclash/rules/chnroute.txt；
	// 这里再兜底找一次面板自身目录下的 rules/。旧版 /koolshare/mihomo 也兼容。
	for _, c := range []string{
		filepath.Join(a.dataDir, "dist-rules", "chnroute.txt"),
		"rules/chnroute.txt",
		"/koolshare/mlclash/rules/chnroute.txt",
		"/koolshare/mihomo/rules/chnroute.txt",
	} {
		if fileExists(c) {
			_ = copyFile(c, a.ruleSetPath())
			return
		}
	}
}

// BuildConfig 依据订阅与设置生成 mihomo 配置。
func (a *App) BuildConfig() (map[string]any, error) {
	s := a.settings
	if len(a.subs) == 0 {
		return nil, errors.New("还没有订阅，请先添加订阅")
	}
	enabled := a.enabledSubsLocked()
	if len(enabled) == 0 {
		return nil, errors.New("没有可用的订阅节点，请先更新订阅")
	}

	cfg := map[string]any{
		"mixed-port":          s.MixedPort,
		"redir-port":          s.RedirPort,
		"allow-lan":           s.AllowLan,
		"bind-address":        "*",
		"mode":                s.Mode,
		"log-level":           s.LogLevel,
		"external-controller": "127.0.0.1:" + itoa(s.APIPort),
		"secret":              s.ClashSecret,
		"unified-delay":       true,
		"tcp-concurrent":      true,
		"ipv6":                false,
		"find-process-mode":   "off",
		"profile": map[string]any{
			"store-selected": true,
			"store-fake-ip":  true,
		},
		// 对齐 FlClash 的 geodata 设置：内核按需加载 geosite/geoip，
		// 由面板负责把 geosite.dat 下载到数据目录并校验。
		"geodata-loader":      "memconservative",
		"geo-auto-update":     false,
		"geo-update-interval": 24,
		"geox-url": map[string]any{
			"geoip":   geoIPURL,
			"geosite": geoSiteURLs[0],
			"mmdb":    geoMMDBURL,
			"asn":     geoASNURL,
		},
	}
	if s.DNSEnable {
		cfg["dns"] = buildDNSSection(s)
	}
	if s.Sniffer {
		cfg["sniffer"] = map[string]any{
			"enable":               true,
			"parse-pure-ip":        true,
			"override-destination": true,
			"sniff": map[string]any{
				"HTTP": map[string]any{"ports": []any{80, "8080-8880"}, "override-destination": true},
				"TLS":  map[string]any{"ports": []any{443, 8443}},
			},
		}
	}

	providers := map[string]any{}
	var useIDs []string
	var subGroups []any
	for _, sub := range enabled {
		id := sub.ID
		entry := map[string]any{
			"type": "file",
			"path": a.providerPath(id),
			"override": map[string]any{
				"additional-prefix": "[" + sub.Name + "] ",
				"udp":               true,
			},
		}
		if f := strings.TrimSpace(sub.IncludeFilter); f != "" {
			entry["filter"] = f
		}
		if f := strings.TrimSpace(sub.ExcludeFilter); f != "" {
			entry["exclude-filter"] = f
		}
		providers[id] = entry
		useIDs = append(useIDs, id)
		// 子分组只挂自己的 provider：默认选中第一个节点，而不是 DIRECT
		subGroups = append(subGroups, map[string]any{
			"name": "📦 " + sub.Name,
			"type": "select",
			"use":  []any{id},
		})
	}
	cfg["proxy-providers"] = providers

	mainProxies := []any{groupAuto}
	mainProxies = append(mainProxies, subGroups2Names(subGroups)...)
	mainProxies = append(mainProxies, "DIRECT")

	// url-test 的测速节奏可由面板调整；老配置/零值退回默认 300s / 50ms
	autoInterval := s.AutoIntervalSec
	if autoInterval < 30 {
		autoInterval = 300
	}
	autoTolerance := s.AutoToleranceMs
	if autoTolerance < 0 {
		autoTolerance = 50
	}

	groups := []any{
		map[string]any{
			"name":    groupMain,
			"type":    "select",
			"proxies": mainProxies,
			"use":     toAnySlice(useIDs),
		},
		map[string]any{
			"name":      groupAuto,
			"type":      "url-test",
			"use":       toAnySlice(useIDs),
			"url":       testURL,
			"interval":  autoInterval,
			"tolerance": autoTolerance,
		},
	}
	groups = append(groups, subGroups...)
	cfg["proxy-groups"] = groups

	// 规则
	var rules []any
	// 去广告必须排在最前：广告域命中 REJECT 后再走直连/代理判断。
	// 只在规则模式生效；geosite.dat 未就绪时先不加，下载完成后会自动热重载。
	if s.Mode == "rule" && s.AdBlock && a.geoSiteReady() {
		rules = append(rules, "GEOSITE,category-ads-all,REJECT")
	}
	if s.RulePreset == "cn-direct" {
		// 域名优先：国内域名解析到海外 CDN IP 时（境外 DoH/ECS 很常见），
		// 只看 IP 会误判成代理；geosite,cn 按域名先直连，和 FlClash 一致。
		if a.geoSiteReady() {
			rules = append(rules, "GEOSITE,cn,DIRECT")
		}
		// 可选：country.mmdb 的 GEOIP,CN（默认关闭，用户可在规则页开启）。
		if s.GeoIPEnable && a.geoIPReady() {
			rules = append(rules, "GEOIP,CN,DIRECT")
		}
		if a.ensureRuleProviderYAML() {
			cfg["rule-providers"] = map[string]any{
				"cn-ip": map[string]any{
					"type":     "file",
					"behavior": "ipcidr",
					"path":     a.ruleProviderPath(),
				},
			}
			rules = append(rules, "RULE-SET,cn-ip,DIRECT")
		}
		rules = append(rules,
			"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
			"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		)
	}
	if s.Mode == "direct" {
		rules = append(rules, "MATCH,DIRECT")
	} else {
		rules = append(rules, "MATCH,"+groupMain)
	}
	cfg["rules"] = rules

	return cfg, nil
}

func subGroups2Names(groups []any) []any {
	out := make([]any, 0, len(groups))
	for _, g := range groups {
		if m, ok := g.(map[string]any); ok {
			if n, ok := m["name"].(string); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// WriteConfig 落盘配置。
func (a *App) WriteConfig() error {
	cfg, err := a.BuildConfig()
	if err != nil {
		return err
	}
	return writeJSONAtomic(a.configPath(), cfg)
}

func normalizeProviderPath(p string) string {
	return filepath.ToSlash(strings.TrimSpace(p))
}

var _ = normalizeProviderPath
var _ = os.Remove

// buildDNSSection 按设置生成 mihomo 的 dns 段。
func buildDNSSection(s Settings) map[string]any {
	nameservers := splitCommaList(s.DNSNameserver)
	fallbacks := splitCommaList(s.DNSFallback)
	dns := map[string]any{
		"enable":             true,
		"ipv6":               false,
		"enhanced-mode":      s.DNSMode,
		"use-hosts":          true,
		"use-system-hosts":   true,
		"respect-rules":      false,
		"default-nameserver": []any{"223.5.5.5", "119.29.29.29"},
		"nameserver":         toAnySlice(nameservers),
	}
	if len(fallbacks) > 0 {
		dns["fallback"] = toAnySlice(fallbacks)
		// mihomo 的 fallback-filter.geoip 默认是 true；一旦配置了 fallback，
		// 内核启动/校验时会去下载 GeoIP MMDB。路由器直连 GitHub 常常超时，
		// 结果整个配置校验失败、内核被停掉。这里显式关掉 geoip 过滤：
		// fallback 仍按 ipcidr 生效，不依赖可能缺失的 MMDB。
		dns["fallback-filter"] = map[string]any{
			"geoip":  false,
			"ipcidr": []any{"240.0.0.0/4"},
		}
	}
	if s.DNSMode == "fake-ip" {
		rng := strings.TrimSpace(s.DNSFakeIPRange)
		if rng == "" {
			rng = "198.18.0.1/16"
		}
		dns["fake-ip-range"] = rng
		dns["fake-ip-filter"] = []any{
			"*.lan", "*.local", "*.localdomain", "localhost.ptlogin2.qq.com",
			"+.msftconnecttest.com", "+.msftncsi.com",
		}
	}
	if listen := strings.TrimSpace(s.DNSListen); listen != "" {
		dns["listen"] = listen
	}
	return dns
}

func splitCommaList(v string) []string {
	out := []string{}
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
