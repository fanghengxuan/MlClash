package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// anti-AD 增强广告列表：mihomo 原生 mrs（约 0.85MB，约 10 万条域名）。
// 与 geosite.dat 的 category-ads-all 相比覆盖更广，但可能误杀，默认关闭。
const (
	antiAdFileName = "anti-ad.mrs"
	antiAdMetaName = "anti-ad.json"
	antiAdProbe    = ".anti-ad-probe.json"
	antiAdMinBytes = 128 << 10
	antiAdMaxBytes = 8 << 20
)

var antiAdURLs = []string{
	"https://anti-ad.net/mihomo.mrs",
	"https://raw.githubusercontent.com/privacy-protection-tools/anti-ad.github.io/master/docs/mihomo.mrs",
	"https://cdn.jsdelivr.net/gh/privacy-protection-tools/anti-ad.github.io@master/docs/mihomo.mrs",
}

type AntiAdMeta struct {
	UpdatedAt int64  `json:"updatedAt"`
	Source    string `json:"source"`
	Size      int64  `json:"size"`
}

func (a *App) antiAdPath() string     { return filepath.Join(a.dataDir, "rules", antiAdFileName) }
func (a *App) antiAdMetaPath() string { return filepath.Join(a.dataDir, "rules", antiAdMetaName) }

func (a *App) loadAntiAdMeta() AntiAdMeta {
	var m AntiAdMeta
	if b, err := os.ReadFile(a.antiAdMetaPath()); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func (a *App) saveAntiAdMeta(m AntiAdMeta) {
	if b, err := json.Marshal(m); err == nil {
		_ = os.WriteFile(a.antiAdMetaPath(), b, 0o644)
	}
}

func (a *App) antiAdReady() bool {
	fi, err := os.Stat(a.antiAdPath())
	if err != nil || fi.IsDir() || fi.Size() < antiAdMinBytes || fi.Size() > antiAdMaxBytes {
		return false
	}
	m := a.loadAntiAdMeta()
	return m.UpdatedAt > 0 && m.Size == fi.Size()
}

func (a *App) antiAdStatus() map[string]any {
	fi, err := os.Stat(a.antiAdPath())
	m := a.loadAntiAdMeta()
	st := map[string]any{
		"present":   err == nil && !fi.IsDir(),
		"ready":     a.antiAdReady(),
		"size":      m.Size,
		"updatedAt": m.UpdatedAt,
		"source":    m.Source,
	}
	if err == nil && !fi.IsDir() {
		st["size"] = fi.Size()
	}
	return st
}

// validateAntiAd 用最小配置让内核真正加载 mrs 规则集。
func (a *App) validateAntiAd() error {
	probe := map[string]any{
		"mixed-port":          0,
		"redir-port":          0,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "error",
		"external-controller": "127.0.0.1:19098",
		"rule-providers": map[string]any{
			"anti-ad": map[string]any{
				"type":     "file",
				"behavior": "domain",
				"format":   "mrs",
				"path":     a.antiAdPath(),
			},
		},
		"rules": []any{"RULE-SET,anti-ad,REJECT", "MATCH,DIRECT"},
	}
	probePath := filepath.Join(a.dataDir, "rules", antiAdProbe)
	defer os.Remove(probePath)
	if err := writeJSONAtomic(probePath, probe); err != nil {
		return err
	}
	out, err := run(90*time.Second, a.mihomoPath, "-t", "-d", a.dataDir, "-f", probePath)
	if err != nil {
		return fmt.Errorf("anti-AD 规则校验失败：%s", tail(out, 400))
	}
	return nil
}

// EnsureAntiAd 下载并校验 anti-AD mrs。force=false 时已有有效文件直接返回；
// 下载/校验失败时保留旧文件，不影响现有规则。
func (a *App) EnsureAntiAd(force bool) error {
	a.antiAdMu.Lock()
	defer a.antiAdMu.Unlock()

	path := a.antiAdPath()
	fi, statErr := os.Stat(path)
	hasFile := statErr == nil && !fi.IsDir() && fi.Size() >= antiAdMinBytes && fi.Size() <= antiAdMaxBytes

	if !force && a.antiAdReady() {
		return nil
	}
	if !force && hasFile {
		a.prog("antiad", "增强广告规则", "check", "校验已有 anti-AD 规则…", false, false)
		if err := a.validateAntiAd(); err == nil {
			a.saveAntiAdMeta(AntiAdMeta{UpdatedAt: fi.ModTime().Unix(), Source: "本地已有文件", Size: fi.Size()})
			return nil
		} else {
			a.addLog("warning", "已有 anti-AD 规则校验失败，尝试重新下载："+err.Error())
		}
	}

	a.prog("antiad", "增强广告规则", "connect", "开始下载 anti-AD 增强广告规则…", false, false)
	var lastErr error
	for _, src := range antiAdURLs {
		body, err := a.fetchGeoBytesLimit(src, antiAdMaxBytes)
		if err != nil {
			lastErr = err
			continue
		}
		if len(body) < antiAdMinBytes || len(body) > antiAdMaxBytes {
			lastErr = fmt.Errorf("%s: 文件大小异常 %d", src, len(body))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("创建规则目录失败: %v", err)
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
		if err := a.validateAntiAd(); err != nil {
			_ = os.Remove(path)
			if fileExists(old) {
				_ = os.Rename(old, path)
			}
			lastErr = err
			continue
		}
		_ = os.Remove(old)
		a.saveAntiAdMeta(AntiAdMeta{UpdatedAt: time.Now().Unix(), Source: src, Size: int64(len(body))})
		a.prog("antiad", "增强广告规则", "done", "增强广告规则已更新", true, false)
		a.addLog("info", fmt.Sprintf("增强广告规则已更新（来源 %s，%d 字节）", src, len(body)))
		return nil
	}
	msg := "下载失败"
	if lastErr != nil {
		msg += "：" + lastErr.Error()
	}
	a.prog("antiad", "增强广告规则", "error", msg, true, true)
	return fmt.Errorf("增强广告规则%s", msg)
}

// ensureAntiAdAsync 在后台补齐 anti-AD 规则，完成后热重载让 RULE-SET 生效。
func (a *App) ensureAntiAdAsync() {
	if a.antiAdReady() {
		return
	}
	a.bgTasks.Add(1)
	safeGo("anti-ad-init", func() {
		defer a.bgTasks.Done()
		if err := a.EnsureAntiAd(false); err != nil {
			a.addLog("warning", "增强广告规则初始化失败："+err.Error())
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
					a.addLog("warning", "增强广告规则已就绪，但热重载失败："+err.Error())
				}
				return
			}
			time.Sleep(2 * time.Second)
		}
		a.addLog("warning", "增强广告规则已就绪，但内核 30 秒内未就绪，暂未热重载")
	})
}

// antiAdDue 与规则集共用自动更新开关与间隔。
func (a *App) antiAdDue() bool {
	a.mu.RLock()
	enabled := a.settings.AdBlock
	auto := a.settings.RuleAutoUpdate
	hours := a.settings.RuleIntervalH
	a.mu.RUnlock()
	if !enabled || !auto {
		return false
	}
	if hours <= 0 {
		hours = 24
	}
	m := a.loadAntiAdMeta()
	if m.UpdatedAt == 0 {
		return true
	}
	return time.Since(time.Unix(m.UpdatedAt, 0)) >= time.Duration(hours)*time.Hour
}
