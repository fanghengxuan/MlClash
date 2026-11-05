package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// groupQuality 是只供质量探测使用的内部策略组，不参与主流量选择。
	groupQuality        = "🧪 质量检测"
	qualityListenerName = "mlclash-quality"

	qualityScanDebounce     = 8 * time.Second
	qualityScanMaxNodes     = 250
	qualityScanNodeTimeout  = 6 * time.Second
	qualityScanTotalTimeout = 10 * time.Minute
	qualityScanPace         = 800 * time.Millisecond
)

// NodeQuality 是单个节点出口 IP 的质量结果。
type NodeQuality struct {
	Level string `json:"level"` // good | normal | risky | unknown
	Type  string `json:"type,omitempty"`
	IP    string `json:"ip,omitempty"`
	At    int64  `json:"at"`
	Err   string `json:"err,omitempty"`
}

type qualityStore struct {
	UpdatedAt int64                  `json:"updatedAt"`
	Items     map[string]NodeQuality `json:"items"`
}

func (a *App) qualityPath() string { return filepath.Join(a.dataDir, "quality.json") }

// loadQuality 读取上次的质量结果；文件缺失或损坏时保持空表。
func (a *App) loadQuality() {
	var st qualityStore
	_ = readJSON(a.qualityPath(), &st)
	if st.Items == nil {
		st.Items = map[string]NodeQuality{}
	}
	a.qualityMu.Lock()
	a.quality = st
	a.qualityMu.Unlock()
}

func (a *App) qualityOf(name string) (NodeQuality, bool) {
	a.qualityMu.Lock()
	q, ok := a.quality.Items[name]
	a.qualityMu.Unlock()
	return q, ok
}

// qualitySnapshot 复制一份质量表，供一次请求内批量读取，避免反复加锁。
func (a *App) qualitySnapshot() map[string]NodeQuality {
	a.qualityMu.Lock()
	defer a.qualityMu.Unlock()
	out := make(map[string]NodeQuality, len(a.quality.Items))
	for name, q := range a.quality.Items {
		out[name] = q
	}
	return out
}

func (a *App) saveQualityLocked() {
	_ = os.MkdirAll(a.dataDir, 0o755)
	_ = writeJSONAtomic(a.qualityPath(), a.quality)
}

func (a *App) qualityPortPath() string { return filepath.Join(a.dataDir, "quality-port") }

// loadQualityPort 读取持久化的探测端口。固定端口避免内核已监听旧端口时，
// 每次重启/热重载都把端口往后推。
func (a *App) loadQualityPort() {
	a.mu.RLock()
	s := a.settings
	a.mu.RUnlock()

	p := 0
	if b, err := os.ReadFile(a.qualityPortPath()); err == nil {
		p, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	if p > 1024 && p < 65536 && p != s.MixedPort && p != s.RedirPort && p != s.APIPort && p != panelPort {
		a.qualityPortNum = p
		return
	}
	a.qualityPortNum = pickQualityPort(s, true)
	a.saveQualityPort(a.qualityPortNum)
}

func (a *App) saveQualityPort(p int) {
	_ = writeFileAtomic(a.qualityPortPath(), []byte(itoa(p)), 0o600)
}

// qualityPort 返回进程内固定的质量探测端口。
// 固定值很关键：如果按「当前是否被占用」动态选端口，内核已经监听旧端口时
// 每次热重载都会换端口，配置永远无法稳定。只在启动时探测一次占用情况。
func (a *App) qualityPort() int {
	a.mu.RLock()
	s := a.settings
	a.mu.RUnlock()

	a.qualityPortMu.Lock()
	defer a.qualityPortMu.Unlock()
	p := a.qualityPortNum
	if p > 1024 && p < 65536 && p != s.MixedPort && p != s.RedirPort && p != s.APIPort && p != panelPort {
		return p
	}
	// 端口与内核端口冲突（用户改了设置）：重新选，但不再探测系统占用，
	// 因为内核可能正占着旧端口，探测会把固定值不断推向新端口。
	p = pickQualityPort(s, false)
	a.qualityPortNum = p
	a.saveQualityPort(p)
	return p
}

func pickQualityPort(s Settings, checkFree bool) int {
	used := map[int]bool{
		s.MixedPort: true,
		s.RedirPort: true,
		s.APIPort:   true,
		panelPort:   true,
	}
	p := s.MixedPort + 1
	if p <= 1024 {
		p = 17890
	}
	for i := 0; i < 64; i++ {
		if p > 1024 && p < 65536 && !used[p] && (!checkFree || portFree(p)) {
			return p
		}
		p++
	}
	return 17890
}

func portFree(p int) bool {
	ln, err := net.Listen("tcp", "127.0.0.1:"+itoa(p))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// autoExcludeFilter 把有风险的节点名拼成正则，供「自动选择」组排除。
// 全部已检测节点都有风险时返回空串：排除会让组为空，宁可保留全部。
func (a *App) autoExcludeFilter() string {
	a.qualityMu.Lock()
	total := len(a.quality.Items)
	risky := make([]string, 0, total)
	for name, q := range a.quality.Items {
		if q.Level == "risky" {
			risky = append(risky, name)
		}
	}
	a.qualityMu.Unlock()

	if len(risky) == 0 || len(risky) >= total {
		return ""
	}
	sort.Strings(risky)
	parts := make([]string, 0, len(risky))
	for _, n := range risky {
		parts = append(parts, regexp.QuoteMeta(n))
	}
	// mihomo 的 exclude-filter 是子串匹配：不锚定会把「[机场A] 加拿大 IPv6」
	// 也当成「[机场A] 加拿大」误排除。
	return "^(?:" + strings.Join(parts, "|") + ")$"
}

// scheduleQualityScan 在订阅更新后延迟合并触发一次全量质量检测。
func (a *App) scheduleQualityScan() {
	a.qualityMu.Lock()
	if a.qualityTimer != nil {
		a.qualityTimer.Stop()
	}
	a.qualityTimer = time.AfterFunc(qualityScanDebounce, func() {
		a.qualityMu.Lock()
		a.qualityTimer = nil
		if a.qualityBusy {
			// 扫描期间又有订阅更新：等本轮结束后再补一次。
			a.qualityPending = true
			a.qualityMu.Unlock()
			return
		}
		a.qualityBusy = true
		a.qualityMu.Unlock()

		a.bgTasks.Add(1)
		go func() {
			defer a.bgTasks.Done()
			a.scanNodeQuality()
			a.qualityMu.Lock()
			a.qualityBusy = false
			pending := a.qualityPending
			a.qualityPending = false
			a.qualityMu.Unlock()
			if pending {
				a.scheduleQualityScan()
			}
		}()
	})
	a.qualityMu.Unlock()
}

// scanNodeQuality 逐个切换内部质量组并探测出口 IP。
// 同一 select 组一次只能指向一个节点，因此只能串行。
func (a *App) scanNodeQuality() {
	if a.engine == nil || !a.engine.Running() {
		return
	}
	if !a.engine.waitCoreReady(15 * time.Second) {
		return
	}
	// 订阅更新后的热重载可能还没结束，短暂重试等待新配置里的质量检测组。
	var proxies ProxyMap
	found := false
	for attempt := 0; attempt < 3; attempt++ {
		var err error
		proxies, err = a.clash.Proxies()
		if err == nil {
			if _, ok := proxies[groupQuality]; ok {
				found = true
				break
			}
		}
		if attempt < 2 {
			time.Sleep(3 * time.Second)
		}
	}
	if !found {
		a.addLog("warning", "质量检测跳过：内核配置里没有质量检测组")
		return
	}

	names := a.qualityScanNodes()
	if len(names) == 0 {
		return
	}
	// 单次最多扫描 qualityScanMaxNodes 个；其余节点保留上次结果。
	scanNames := names
	if len(scanNames) > qualityScanMaxNodes {
		scanNames = scanNames[:qualityScanMaxNodes]
	}
	a.addLog("info", fmt.Sprintf("开始检测 %d 个节点的出口质量", len(scanNames)))

	client := a.qualityClient()
	deadline := time.Now().Add(qualityScanTotalTimeout)
	items := make(map[string]NodeQuality, len(scanNames))
	for i, name := range scanNames {
		if time.Now().After(deadline) {
			a.addLog("warning", "质量检测超时，保存已完成的结果")
			break
		}
		q := NodeQuality{Level: "unknown", At: time.Now().Unix()}
		if err := a.clash.Select(groupQuality, name); err != nil {
			q.Err = "切换节点失败：" + err.Error()
		} else {
			// 给内核一点时间应用选择，避免请求仍走上一个节点。
			time.Sleep(80 * time.Millisecond)
			q = a.probeQualityVia(client)
		}
		items[name] = q
		if i < len(scanNames)-1 {
			time.Sleep(qualityScanPace)
		}
	}

	// 合并而不是整体覆盖：中途超时/失败的节点保留上次结果，同时清掉已不存在的节点。
	valid := make(map[string]bool, len(names))
	for _, n := range names {
		valid[n] = true
	}
	a.qualityMu.Lock()
	if a.quality.Items == nil {
		a.quality.Items = map[string]NodeQuality{}
	}
	for name := range a.quality.Items {
		if !valid[name] {
			delete(a.quality.Items, name)
		}
	}
	for name, q := range items {
		a.quality.Items[name] = q
	}
	a.quality.UpdatedAt = time.Now().Unix()
	a.saveQualityLocked()
	a.qualityMu.Unlock()

	risky := 0
	for _, q := range items {
		if q.Level == "risky" {
			risky++
		}
	}
	a.addLog("info", fmt.Sprintf("质量检测完成：%d 个节点，%d 个有风险", len(items), risky))
	// 让「自动选择」的 exclude-filter 立即生效。
	_ = a.appReload()
}

// qualityScanNodes 取启用订阅的节点最终名（与 /proxies、分组里的名字一致）。
func (a *App) qualityScanNodes() []string {
	provs, err := a.clash.Providers()
	if err != nil {
		return nil
	}
	a.mu.RLock()
	enabled := map[string]bool{}
	for _, s := range a.subs {
		if s.Enabled {
			enabled[s.ID] = true
		}
	}
	a.mu.RUnlock()

	seen := map[string]bool{}
	var names []string
	for id, pr := range provs {
		if !enabled[id] {
			continue
		}
		for _, p := range pr.Proxies {
			if p.Name == "" || isGroupType(p.Type) || isPseudoProxy(p.Type) || seen[p.Name] {
				continue
			}
			seen[p.Name] = true
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// qualityClient 经内部 mixed 监听端口出站，每次请求都新建连接。
func (a *App) qualityClient() *http.Client {
	proxyURL, _ := url.Parse("http://127.0.0.1:" + itoa(a.qualityPort()))
	return &http.Client{
		Timeout: qualityScanNodeTimeout,
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			DisableKeepAlives: true,
		},
	}
}

func (a *App) probeQualityVia(client *http.Client) NodeQuality {
	now := time.Now().Unix()
	req, err := http.NewRequest("GET", ipAPIURL, nil)
	if err != nil {
		return NodeQuality{Level: "unknown", At: now, Err: err.Error()}
	}
	req.Header.Set("User-Agent", "MlClash/"+panelVersion)
	resp, err := client.Do(req)
	if err != nil {
		return NodeQuality{Level: "unknown", At: now, Err: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return NodeQuality{Level: "unknown", At: now, Err: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return NodeQuality{Level: "unknown", At: now, Err: "HTTP " + itoa(resp.StatusCode)}
	}
	q, err := parseIPQualityIpAPI(body)
	if err != nil {
		return NodeQuality{Level: "unknown", At: now, Err: err.Error()}
	}
	return NodeQuality{Level: q.Level, Type: q.Type, IP: q.IP, At: now}
}
