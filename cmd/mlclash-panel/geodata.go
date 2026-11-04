package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// 国内域名规则（GEOSITE,cn）：对齐 FlClash 的 geodata 做法。
// 路由器 jffs 空间有限，只下载 geosite.dat（约 4.3MB），不下载 16MB 的
// geoip.dat；IP 匹配继续用体积小得多的 cn-ip CIDR 列表兜底。
const (
	geoSiteFileName  = "geosite.dat"
	geoSiteStampName = ".geosite.stamp"
	geoSiteMetaName  = "geosite.json"
	geoSiteProbeName = ".geosite-probe.json"
	geoSiteMinBytes  = 1 << 20  // 小于 1MB 基本是错误页
	geoSiteMaxBytes  = 16 << 20 // 防止异常大文件塞满 jffs
)

// geoSiteURLs 与 FlClash 使用同一份 MetaCubeX 数据；用 jsdelivr 镜像，
// 因为路由器直连 GitHub 经常超时。第一个成功且能通过 mihomo -t 校验的生效。
var geoSiteURLs = []string{
	"https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat",
	"https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat",
	"https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/release/geosite.dat",
	"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
}

var (
	geoIPURL   = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
	geoMMDBURL = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb"
	geoASNURL  = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/GeoLite2-ASN.mmdb"
)

var (
	reGeoSiteRecords   = regexp.MustCompile(`records:\s*(\d+)`)
	reGeoSiteCNRecords = regexp.MustCompile(`rule cn => DIRECT, records:\s*(\d+)`)
)

// GeoDataMeta 是 geosite.dat 的现状，展示在规则页。
type GeoDataMeta struct {
	UpdatedAt int64  `json:"updatedAt"`
	Source    string `json:"source"`
	Size      int64  `json:"size"`
	Count     int    `json:"count"`
}

func (a *App) geoSitePath() string      { return filepath.Join(a.dataDir, geoSiteFileName) }
func (a *App) geoSiteStampPath() string { return filepath.Join(a.dataDir, geoSiteStampName) }
func (a *App) geoSiteMetaPath() string  { return filepath.Join(a.dataDir, geoSiteMetaName) }
func (a *App) geoSiteProbePath() string { return filepath.Join(a.dataDir, geoSiteProbeName) }

func (a *App) loadGeoMeta() GeoDataMeta {
	var m GeoDataMeta
	_ = readJSON(a.geoSiteMetaPath(), &m)
	if m.UpdatedAt == 0 {
		if fi, err := os.Stat(a.geoSitePath()); err == nil {
			m.UpdatedAt = fi.ModTime().Unix()
		}
	}
	if m.Size == 0 {
		if fi, err := os.Stat(a.geoSitePath()); err == nil {
			m.Size = fi.Size()
		}
	}
	return m
}

func (a *App) saveGeoMeta(m GeoDataMeta) {
	_ = writeJSONAtomic(a.geoSiteMetaPath(), m)
}

func fileSignature(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d|%d", fi.Size(), fi.ModTime().UnixNano())
}

// geoSiteReady 表示文件存在、大小合理，并且当前内容已经通过 mihomo -t 校验。
func (a *App) geoSiteReady() bool {
	fi, err := os.Stat(a.geoSitePath())
	if err != nil || fi.IsDir() || fi.Size() < geoSiteMinBytes || fi.Size() > geoSiteMaxBytes {
		return false
	}
	b, err := os.ReadFile(a.geoSiteStampPath())
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == fileSignature(a.geoSitePath())
}

// geoSiteStatus 供 /api/rules/set 使用。
func (a *App) geoSiteStatus() map[string]any {
	fi, err := os.Stat(a.geoSitePath())
	m := a.loadGeoMeta()
	st := map[string]any{
		"present":   err == nil && !fi.IsDir(),
		"ready":     a.geoSiteReady(),
		"size":      m.Size,
		"updatedAt": m.UpdatedAt,
		"count":     m.Count,
		"source":    m.Source,
	}
	if err == nil && !fi.IsDir() {
		st["size"] = fi.Size()
	}
	return st
}

// geoSiteDue 与 IP 规则共用「自动更新 + 间隔」设置。
func (a *App) geoSiteDue() bool {
	a.mu.RLock()
	auto := a.settings.RuleAutoUpdate
	hours := a.settings.RuleIntervalH
	a.mu.RUnlock()
	if !auto {
		return false
	}
	if !a.geoSiteReady() {
		return true
	}
	if hours <= 0 {
		hours = 24
	}
	m := a.loadGeoMeta()
	ts := m.UpdatedAt
	if ts == 0 {
		if fi, err := os.Stat(a.geoSitePath()); err == nil {
			ts = fi.ModTime().Unix()
		}
	}
	return time.Since(time.Unix(ts, 0)) >= time.Duration(hours)*time.Hour
}

// validateGeoSite 用最小配置让内核真正加载我们依赖的 geosite 标签，确认文件可用。
func (a *App) validateGeoSite() (int, error) {
	probe := map[string]any{
		"mixed-port":          0,
		"redir-port":          0,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "error",
		"external-controller": "127.0.0.1:19099",
		"geodata-loader":      "memconservative",
		"rules": []any{
			"GEOSITE,cn,DIRECT",
			"GEOSITE,apple-cn,DIRECT",
			"GEOSITE,google-cn,DIRECT",
			"GEOSITE,category-ads-all,REJECT",
			"MATCH,DIRECT",
		},
	}
	probePath := a.geoSiteProbePath()
	defer os.Remove(probePath)
	if err := writeJSONAtomic(probePath, probe); err != nil {
		return 0, err
	}
	out, err := run(90*time.Second, a.mihomoPath, "-t", "-d", a.dataDir, "-f", probePath)
	if err != nil {
		return 0, fmt.Errorf("geosite.dat 校验失败：%s", tail(out, 400))
	}
	count := 0
	if m := reGeoSiteCNRecords.FindStringSubmatch(out); m != nil {
		count, _ = atoiSafe(m[1])
	} else if m := reGeoSiteRecords.FindStringSubmatch(out); m != nil {
		count, _ = atoiSafe(m[1])
	}
	return count, nil
}

func (a *App) fetchGeoBytesLimit(src string, max int64) ([]byte, error) {
	a.mu.RLock()
	insecure := a.settings.SubInsecure
	a.mu.RUnlock()
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MlClash/"+panelVersion)
	resp, err := a.newHTTPClient(false, insecure).Do(req)
	if err != nil {
		return nil, describeNetErr(err, src)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("文件超过 %d 字节", max)
	}
	return b, nil
}

func (a *App) fetchGeoBytes(src string) ([]byte, error) {
	return a.fetchGeoBytesLimit(src, geoSiteMaxBytes)
}

// EnsureGeoSite 确保 geosite.dat 可用。force=false 时已有有效文件直接返回；
// 下载失败时保留旧文件，不影响 IP 规则与内核启动。
func (a *App) EnsureGeoSite(force bool) error {
	a.geoMu.Lock()
	defer a.geoMu.Unlock()

	path := a.geoSitePath()
	fi, statErr := os.Stat(path)
	hasFile := statErr == nil && !fi.IsDir() && fi.Size() >= geoSiteMinBytes && fi.Size() <= geoSiteMaxBytes

	if !force && a.geoSiteReady() {
		return nil
	}
	if !force && hasFile {
		// 文件可能是手动放入/备份恢复的：先校验，通过就补戳，不重新下载。
		a.prog("geosite", "域名规则", "check", "校验已有 geosite.dat…", false, false)
		count, err := a.validateGeoSite()
		if err == nil {
			_ = os.WriteFile(a.geoSiteStampPath(), []byte(fileSignature(path)), 0o600)
			m := a.loadGeoMeta()
			m.Size, m.Count = fi.Size(), count
			if m.Source == "" {
				m.Source = "本地已有文件"
			}
			a.saveGeoMeta(m)
			return nil
		}
		a.addLog("warning", "已有 geosite.dat 校验失败，尝试重新下载："+err.Error())
	}

	a.prog("geosite", "域名规则", "connect", "开始下载国内域名规则（geosite.dat）…", false, false)
	var lastErr error
	for _, src := range geoSiteURLs {
		body, err := a.fetchGeoBytes(src)
		if err != nil {
			lastErr = err
			continue
		}
		if len(body) < geoSiteMinBytes || len(body) > geoSiteMaxBytes {
			lastErr = fmt.Errorf("%s: 文件大小异常 %d", src, len(body))
			continue
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, body, 0o644); err != nil {
			lastErr = err
			continue
		}
		old := path + ".old"
		_ = os.Remove(old)
		if hasFile {
			_ = os.Rename(path, old)
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			if fileExists(old) {
				_ = os.Rename(old, path)
			}
			lastErr = err
			continue
		}
		count, verr := a.validateGeoSite()
		if verr != nil {
			_ = os.Remove(path)
			if fileExists(old) {
				_ = os.Rename(old, path)
			}
			lastErr = verr
			continue
		}
		_ = os.Remove(old)
		_ = os.WriteFile(a.geoSiteStampPath(), []byte(fileSignature(path)), 0o600)
		meta := GeoDataMeta{UpdatedAt: time.Now().Unix(), Source: src, Size: int64(len(body)), Count: count}
		a.saveGeoMeta(meta)
		a.prog("geosite", "域名规则", "done", fmt.Sprintf("域名规则已更新：%d 条", count), true, false)
		a.addLog("info", fmt.Sprintf("国内域名规则已更新：%d 条（来源 %s）", count, src))
		return nil
	}
	msg := "下载失败"
	if lastErr != nil {
		msg += "：" + lastErr.Error()
	}
	a.prog("geosite", "域名规则", "error", msg, true, true)
	return fmt.Errorf("域名规则%s", msg)
}

// ensureGeoSiteAsync 在后台补齐 geosite.dat，成功后热重载让 GEOSITE 规则生效。
func (a *App) ensureGeoSiteAsync() {
	if a.geoSiteReady() {
		return
	}
	a.bgTasks.Add(1)
	safeGo("geosite-init", func() {
		defer a.bgTasks.Done()
		if err := a.EnsureGeoSite(false); err != nil {
			a.addLog("warning", "域名规则初始化失败："+err.Error())
			return
		}
		if a.engine == nil {
			return
		}
		// 内核可能还在启动：status.running 早于 external-controller 就绪，
		// 直接 Reload 会拿到 connection refused。等 API 真正可用再重载。
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if !a.engine.Running() {
				return // 用户没有让引擎运行；下次启动时配置里就会带 GEOSITE
			}
			if _, err := a.clash.Version(); err == nil {
				if err := a.engine.Reload(); err != nil {
					a.addLog("warning", "域名规则已就绪，但热重载失败："+err.Error())
				}
				return
			}
			time.Sleep(2 * time.Second)
		}
		a.addLog("warning", "域名规则已就绪，但内核 30 秒内未就绪，暂未热重载")
	})
}

// ---------- 可选：GEOIP,CN（country.mmdb） ----------
//
// FlClash 会内置 GEOIP.dat / GEOIP.metadb 做 GEOIP,CN；路由器 jffs 空间有限，
// 这里做成默认关闭的可选开关，只下载 country.mmdb（约 7.6MB），
// 存成 mihomo 默认懒加载的文件名 geoip.metadb（内容为 MaxMind MMDB）。
const (
	geoIPFileName  = "geoip.metadb"
	geoIPStampName = ".geoip.stamp"
	geoIPMetaName  = "geoip.json"
	geoIPMinBytes  = 2 << 20
	geoIPMaxBytes  = 20 << 20
)

var geoIPURLs = []string{
	"https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb",
	"https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/country.mmdb",
	"https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/release/country.mmdb",
	"https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb",
}

var mmdbMarker = []byte("\xab\xcd\xefMaxMind.com")

func (a *App) geoIPPath() string      { return filepath.Join(a.dataDir, geoIPFileName) }
func (a *App) geoIPStampPath() string { return filepath.Join(a.dataDir, geoIPStampName) }
func (a *App) geoIPMetaPath() string  { return filepath.Join(a.dataDir, geoIPMetaName) }

func (a *App) loadGeoIPMeta() GeoDataMeta {
	var m GeoDataMeta
	_ = readJSON(a.geoIPMetaPath(), &m)
	if m.UpdatedAt == 0 {
		if fi, err := os.Stat(a.geoIPPath()); err == nil {
			m.UpdatedAt = fi.ModTime().Unix()
		}
	}
	if m.Size == 0 {
		if fi, err := os.Stat(a.geoIPPath()); err == nil {
			m.Size = fi.Size()
		}
	}
	return m
}

func (a *App) saveGeoIPMeta(m GeoDataMeta) { _ = writeJSONAtomic(a.geoIPMetaPath(), m) }

// validMMDB 以 MaxMind DB 的元数据标记做完整性校验，挡住错误页/截断文件。
func validMMDB(b []byte) bool {
	if len(b) < geoIPMinBytes || len(b) > geoIPMaxBytes {
		return false
	}
	tail := b
	if len(tail) > 512<<10 {
		tail = tail[len(tail)-(512<<10):]
	}
	return bytes.Contains(tail, mmdbMarker)
}

func (a *App) geoIPReady() bool {
	fi, err := os.Stat(a.geoIPPath())
	if err != nil || fi.IsDir() || fi.Size() < geoIPMinBytes || fi.Size() > geoIPMaxBytes {
		return false
	}
	b, err := os.ReadFile(a.geoIPStampPath())
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == fileSignature(a.geoIPPath())
}

func (a *App) geoIPStatus() map[string]any {
	fi, err := os.Stat(a.geoIPPath())
	m := a.loadGeoIPMeta()
	st := map[string]any{
		"present":   err == nil && !fi.IsDir(),
		"ready":     a.geoIPReady(),
		"size":      m.Size,
		"updatedAt": m.UpdatedAt,
		"source":    m.Source,
	}
	if err == nil && !fi.IsDir() {
		st["size"] = fi.Size()
	}
	return st
}

func (a *App) geoIPDue() bool {
	a.mu.RLock()
	enabled := a.settings.GeoIPEnable
	auto := a.settings.RuleAutoUpdate
	hours := a.settings.RuleIntervalH
	a.mu.RUnlock()
	if !enabled || !auto {
		return false
	}
	if !a.geoIPReady() {
		return true
	}
	if hours <= 0 {
		hours = 24
	}
	m := a.loadGeoIPMeta()
	ts := m.UpdatedAt
	if ts == 0 {
		if fi, err := os.Stat(a.geoIPPath()); err == nil {
			ts = fi.ModTime().Unix()
		}
	}
	return time.Since(time.Unix(ts, 0)) >= time.Duration(hours)*time.Hour
}

// EnsureGeoIP 下载并校验 country.mmdb。只有用户开启 GEOIP 开关时才需要。
func (a *App) EnsureGeoIP(force bool) error {
	a.geoMu.Lock()
	defer a.geoMu.Unlock()

	path := a.geoIPPath()
	fi, statErr := os.Stat(path)
	hasFile := statErr == nil && !fi.IsDir() && fi.Size() >= geoIPMinBytes && fi.Size() <= geoIPMaxBytes

	if !force && a.geoIPReady() {
		return nil
	}
	if !force && hasFile {
		body, err := os.ReadFile(path)
		if err == nil && validMMDB(body) {
			_ = os.WriteFile(a.geoIPStampPath(), []byte(fileSignature(path)), 0o600)
			m := a.loadGeoIPMeta()
			m.Size = fi.Size()
			if m.Source == "" {
				m.Source = "本地已有文件"
			}
			a.saveGeoIPMeta(m)
			return nil
		}
	}

	a.prog("geoip", "GEOIP 规则", "connect", "开始下载 country.mmdb（约 7.6MB）…", false, false)
	var lastErr error
	for _, src := range geoIPURLs {
		body, err := a.fetchGeoBytesLimit(src, geoIPMaxBytes)
		if err != nil {
			lastErr = err
			continue
		}
		if !validMMDB(body) {
			lastErr = fmt.Errorf("%s: 不是有效的 MaxMind MMDB", src)
			continue
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, body, 0o644); err != nil {
			lastErr = err
			continue
		}
		old := path + ".old"
		_ = os.Remove(old)
		if hasFile {
			_ = os.Rename(path, old)
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			if fileExists(old) {
				_ = os.Rename(old, path)
			}
			lastErr = err
			continue
		}
		_ = os.Remove(old)
		_ = os.WriteFile(a.geoIPStampPath(), []byte(fileSignature(path)), 0o600)
		meta := GeoDataMeta{UpdatedAt: time.Now().Unix(), Source: src, Size: int64(len(body))}
		a.saveGeoIPMeta(meta)
		a.prog("geoip", "GEOIP 规则", "done", "GEOIP 数据库已更新", true, false)
		a.addLog("info", fmt.Sprintf("GEOIP 数据库已更新（来源 %s）", src))
		return nil
	}
	msg := "下载失败"
	if lastErr != nil {
		msg += "：" + lastErr.Error()
	}
	a.prog("geoip", "GEOIP 规则", "error", msg, true, true)
	return fmt.Errorf("GEOIP 数据库%s", msg)
}

func (a *App) ensureGeoIPAsync() {
	a.mu.RLock()
	enabled := a.settings.GeoIPEnable
	a.mu.RUnlock()
	if !enabled || a.geoIPReady() {
		return
	}
	a.bgTasks.Add(1)
	safeGo("geoip-init", func() {
		defer a.bgTasks.Done()
		if err := a.EnsureGeoIP(false); err != nil {
			a.addLog("warning", "GEOIP 数据库初始化失败："+err.Error())
			return
		}
		if a.engine == nil {
			return
		}
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if !a.engine.Running() {
				return
			}
			if _, err := a.clash.Version(); err == nil {
				if err := a.engine.Reload(); err != nil {
					a.addLog("warning", "GEOIP 已就绪，但热重载失败："+err.Error())
				}
				return
			}
			time.Sleep(2 * time.Second)
		}
		a.addLog("warning", "GEOIP 已就绪，但内核 30 秒内未就绪，暂未热重载")
	})
}
