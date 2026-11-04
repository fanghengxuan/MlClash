package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:1234": true,
		"[::1]:1234":     true,
		"192.168.1.2:80": false,
		"10.0.0.1:80":    false,
	}
	for addr, want := range cases {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}

func TestParsePreroutingIfaces(t *testing.T) {
	out := `-P PREROUTING ACCEPT
-A PREROUTING -i br0 -j MLCLASH
-A PREROUTING -i br1 -j MLCLASH
-A PREROUTING -i br2 -j MIHOMO
-A PREROUTING -d 192.168.1.2/32 -j VSERVER
-A PREROUTING -i eth0 -j OTHER`
	got := parsePreroutingIfaces(out, natChain)
	if len(got) != 2 || got[0] != "br0" || got[1] != "br1" {
		t.Fatalf("parsePreroutingIfaces(MLCLASH) = %v, want [br0 br1]", got)
	}
	legacy := parsePreroutingIfaces(out, legacyNatChain)
	if len(legacy) != 1 || legacy[0] != "br2" {
		t.Fatalf("parsePreroutingIfaces(MIHOMO) = %v, want [br2]", legacy)
	}
}

func TestServerRegexMatchesInlineYAML(t *testing.T) {
	text := `
proxies:
    - { name: 英国, type: anytls, server: 51.89.216.207, port: 443 }
    - { name: 美国, type: anytls, server: tj-us-1.example.com, port: 6002 }
proxy-server-nameserver: ['https://223.5.5.5/dns-query']
`
	var hosts []string
	for _, m := range reServerYAML.FindAllStringSubmatch(text, -1) {
		hosts = append(hosts, m[2])
	}
	if len(hosts) != 2 || hosts[0] != "51.89.216.207" || hosts[1] != "tj-us-1.example.com" {
		t.Fatalf("inline YAML server 解析 = %v", hosts)
	}
	for _, m := range reServerJSON.FindAllStringSubmatch(`{"server":"1.2.3.4"}`, -1) {
		if m[1] != "1.2.3.4" {
			t.Fatalf("JSON server 解析 = %v", m[1])
		}
	}
}

func TestIptablesApplyAuthLoopbackOnly(t *testing.T) {
	app := &App{}
	app.settings.PanelPassword = "secret"

	called := false
	h := app.withAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// 本机防火墙钩子：不应要求登录
	req := httptest.NewRequest("POST", "/api/iptables/apply", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("loopback 请求被拦截：code=%d called=%v", rec.Code, called)
	}

	// 局域网来源：仍必须登录
	called = false
	req = httptest.NewRequest("POST", "/api/iptables/apply", nil)
	req.RemoteAddr = "192.168.1.2:5555"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("非回环请求未被鉴权：code=%d called=%v", rec.Code, called)
	}
}
