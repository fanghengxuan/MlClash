# MlClash

[English](#english) | [中文](#中文)

[![CI](https://github.com/fanghengxuan/MlClash/actions/workflows/ci.yml/badge.svg)](https://github.com/fanghengxuan/MlClash/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/fanghengxuan/MlClash)](https://github.com/fanghengxuan/MlClash/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

MlClash is a mihomo-based proxy plugin for ASUS Merlin routers. It ships a Go web panel, transparent IPv4/IPv6 forwarding, subscription and rule management, optional ad blocking, and kernel updates. It supports Asuswrt-Merlin/gnuton and koolshare/SWRT.

![Overview](docs/screenshot-overview.png)
![Settings](docs/screenshot-settings.png)

---

## English

### Overview

MlClash runs a slim armv7 build of [mihomo](https://github.com/MetaCubeX/mihomo) on the router and serves an embedded web panel on port 9380. Subscriptions, node groups, rules, DNS, forwarding, backups, storage and kernel updates are managed from the panel. The panel is compiled into the Go binary; no Node.js or frontend build step is required.

### Features

- mihomo core with `rule`, `global` and `direct` modes
- Subscriptions: URL or pasted config, include/exclude regex, per-subscription update interval
- Node groups: manual select and URL test
- Transparent TCP proxy for IPv4 and IPv6 via `iptables` / `ip6tables` REDIRECT
- IPv4/IPv6 QUIC blocking to force TCP fallback
- China direct preset: `GEOSITE,cn` + optional `GEOIP,CN` + `RULE-SET,cn-ip`; Apple/Google China domains direct
- Optional ad blocking: anti-AD `mihomo.mrs` (~100k domains) with built-in `category-ads-all` fallback
- Optional DNS: `redir-host` or `fake-ip`, configurable upstreams
- Panel: overview, nodes, subscriptions, rules, connections, SSE logs, 12 settings pages
- Backup/restore (JSON), storage cleanup, kernel upload, online update and rollback
- Auto-start, scheduled restart, panel password, LAN access control, log levels

### Requirements

- armv7 ASUS router. Tested on TUF-AX3000 (axhnd.675x, kernel 4.1) with gnuton Merlin; the koolshare package is build- and smoke-tested.
- JFFS enabled and at least 18 MB free. The package expands to about 16.3 MB plus 2 MB headroom.
- 512 MB RAM or more recommended.
- SSH for the Merlin installer; software center for koolshare.

### Installation

#### Asuswrt-Merlin / gnuton

1. Download `MlClash_<version>_armv7_merlin.tar.gz` from [Releases](https://github.com/fanghengxuan/MlClash/releases).
2. Enable **Administration → System → Enable JFFS custom scripts and configs**.
3. Upload and install:

```sh
cd /tmp
tar -xzf MlClash_<version>_armv7_merlin.tar.gz
sh /tmp/MlClash/install.sh
```

4. Open `http://<router-ip>:9380`.

#### koolshare / SWRT

Download `MlClash_<version>_armv7_koolshare.tar.gz` and install it from the software center. Manual install:

```sh
cd /tmp
tar -xzf MlClash_<version>_armv7_koolshare.tar.gz
sh /tmp/mlclash/install.sh
```

#### Upgrade and migration

The installer upgrades in place. Versions before 3.0 were named `mihomo-merlin`. The installer migrates `/jffs/mihomo-merlin` or `/koolshare/mihomo` data, subscriptions, rules and startup hooks to the new paths. Backups with the old `mihomo-merlin-backup` kind are still accepted.

Uninstall:

```sh
sh /jffs/mlclash/uninstall.sh              # remove program and data
sh /jffs/mlclash/uninstall.sh --keep-data  # keep data
```

### First run

1. Open the panel and add a subscription under **订阅 (Subscriptions)**.
2. Select a node or group under **节点 (Nodes)**.
3. Start the engine from **概览 (Overview)**.
4. Optional: enable **设置 → 国内 IP 规则 → 拦截广告与统计域名**.
5. Optional: set **设置 → 启动与访问 → 面板密码** if the LAN is not fully trusted.

### Ports and data

| Item | Default |
|---|---|
| Panel | `0.0.0.0:9380` |
| Mixed HTTP/SOCKS | `7890` |
| Transparent redirect | `3333` |
| mihomo API | `127.0.0.1:9090` |
| Merlin install / data | `/jffs/mlclash`, `/jffs/mlclash/data` |
| Merlin runtime logs | `/tmp/mlclash/logs` |
| koolshare install / data | `/koolshare/mlclash` |

The panel and API are intended for LAN use. Do not expose ports 9380 or 9090 to the WAN.

### Ad blocking

One switch: **设置 → 国内 IP 规则 → 拦截广告与统计域名**. It is off by default and only applies in `rule` mode.

- Primary list: anti-AD `mihomo.mrs` (~100k domains, ~0.85 MB), downloaded by the panel.
- Fallback/supplement: built-in `category-ads-all` (905 domains), so filtering still works when the enhanced list is unavailable.
- The list follows the rule update schedule (`ruleAutoUpdate` / `ruleIntervalH`, 24 h by default) and can be updated from the rules page.
- Domain filtering cannot remove first-party or in-feed ads, YouTube ads, ads served from the content domain, or requests without a usable SNI/HTTP Host.

### IPv6

LAN IPv4 and IPv6 TCP traffic is redirected to mihomo. The IPv6 chain bypasses `::1`, ULA, link-local, multicast and the LAN `/64`; IPv6 QUIC (UDP/443) is rejected to force TCP. IPv6 coverage requires `ip6tables`, which the tested firmware includes.

### Build from source

Requirements: Go 1.23+, curl, tar, gzip.

```sh
./build.sh                  # fetch the slim kernel and chnroute, build the panel, package koolshare
./build.sh --no-download    # use .cache, no network
./tools/build-merlin-package.sh
```

Options: `--kernel <path>`, `NOKERNEL=1`, `PANEL_UPX=1`.
Kernel source: [fanghengxuan/mihomo](https://github.com/fanghengxuan/mihomo), `slim-*` releases.
The panel is plain HTML/CSS/JS embedded with `go:embed`; there is no Node.js build step.

### Tests

```sh
go test -race ./...
python3 tests/ui_check.py
sh tests/merlin_install_smoke.sh
```

Additional smoke tests require a built `dist/stage` or packages: `tests/install_smoke.sh`, `tests/nokernel_test.sh`, `tests/cleanup_test.sh`. `tests/e2e.sh` requires a local darwin mihomo binary and is optional.

### Project layout

```
api.go        HTTP API and auth
config.go     mihomo config generation
engine.go     process, iptables/ip6tables, reload/watchdog
subs.go       subscription fetch/parse
ruleset.go    cn-ip source and scheduler
geodata.go    geosite/geoip download and validation
antiad.go     anti-AD mrs download and validation
web/          embedded panel
plugin/       Merlin and koolshare installers
tools/        package builders
tests/        smoke and UI checks
docs/API.md   API reference
```

### Documentation

- [API reference](docs/API.md)
- [Kernel notes](docs/kernel.md)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)

### Troubleshooting

- **Engine not running**: `sh /jffs/mlclash/ctl.sh status`; check `/tmp/mlclash/logs/panel.log` and `mihomo.log`.
- **Panel unreachable**: check the panel process and port 9380; restart with `ctl.sh restart`.
- **Ads still visible**: confirm `mode=rule`, ad blocking is on, and anti-AD shows `ready`; restart the app to drop cached ads. First-party, in-feed and YouTube ads are not domain-blockable.
- **Low space**: run storage cleanup in the panel, remove unused kernel backups, or use the `NOKERNEL=1` package.

### Security

- The panel has no password by default. Set one if other LAN devices are not trusted.
- Backups contain subscription URLs and tokens. Store them accordingly.
- The mihomo API binds to `127.0.0.1` and uses a generated secret.

### License

MIT. mihomo is GPL-3.0. Geosite/geoip data comes from MetaCubeX/meta-rules-dat, anti-AD, Loyalsoldier and their upstream sources. Third-party components and data remain under their respective licenses.

---

## 中文

### 项目简介

MlClash 是运行在华硕梅林路由器上的 mihomo 代理插件，内置 Web 面板，提供 Asuswrt-Merlin/gnuton 原生安装包和 koolshare/SWRT 离线安装包。

### 功能

- mihomo 内核，支持 rule / global / direct 三种模式
- 订阅管理：链接或粘贴导入、包含/排除正则、单订阅更新间隔
- 节点分组：手动选择 + URL 测速
- IPv4/IPv6 TCP 透明代理（iptables / ip6tables REDIRECT）
- IPv4/IPv6 QUIC 阻断，迫使回落 TCP
- 国内直连预设：`GEOSITE,cn` + 可选 `GEOIP,CN` + `RULE-SET,cn-ip`；Apple/Google 中国域名直连
- 可选去广告：anti-AD `mihomo.mrs`（约 10 万条）+ 内置 `category-ads-all` 兜底
- 可选 DNS：redir-host / fake-ip，可配置上游
- 面板：概览、节点、订阅、规则、连接、SSE 日志、12 个设置子页
- 备份恢复、存储清理、内核上传/在线更新/回滚
- 开机自启、定时重启、面板密码、局域网访问、日志级别

### 环境要求

- armv7 华硕路由器。已在 TUF-AX3000（axhnd.675x，内核 4.1）的 gnuton Merlin 上验证；koolshare 包通过构建与冒烟测试。
- 启用 JFFS，安装需至少 18MB 可用空间（解压约 16.3MB + 2MB 余量）。
- 建议 512MB 及以上内存。
- Merlin 安装需 SSH；koolshare 通过软件中心安装。

### 安装

#### Asuswrt-Merlin / gnuton

1. 从 [Releases](https://github.com/fanghengxuan/MlClash/releases) 下载 `MlClash_<版本>_armv7_merlin.tar.gz`。
2. 开启 **系统管理 → 系统设置 → Enable JFFS custom scripts and configs**。
3. 上传并安装：

```sh
cd /tmp
tar -xzf MlClash_<版本>_armv7_merlin.tar.gz
sh /tmp/MlClash/install.sh
```

4. 打开 `http://<路由器IP>:9380`。

#### koolshare / SWRT

下载 `MlClash_<版本>_armv7_koolshare.tar.gz`，在软件中心离线安装。手动安装：

```sh
cd /tmp
tar -xzf MlClash_<版本>_armv7_koolshare.tar.gz
sh /tmp/mlclash/install.sh
```

#### 升级与迁移

安装器支持原地升级。3.0 以前的项目名为 `mihomo-merlin`；安装器会把 `/jffs/mihomo-merlin`（或 `/koolshare/mihomo`）下的数据、订阅、规则和启动钩子迁移到新路径，旧 `mihomo-merlin-backup` 备份仍可恢复。

卸载：

```sh
sh /jffs/mlclash/uninstall.sh              # 删除程序和全部数据
sh /jffs/mlclash/uninstall.sh --keep-data  # 保留数据
```

### 首次使用

1. 在「订阅」页添加订阅。
2. 在「节点」页选择分组或节点。
3. 在「概览」页启动引擎。
4. 可选：在「设置 → 国内 IP 规则」开启「拦截广告与统计域名」。
5. 可选：在「设置 → 启动与访问」设置面板密码。

### 端口与数据目录

| 项目 | 默认值 |
|---|---|
| 面板 | `0.0.0.0:9380` |
| HTTP/SOCKS 混合端口 | `7890` |
| 透明代理 redir 端口 | `3333` |
| mihomo API | `127.0.0.1:9090` |
| Merlin 安装/数据 | `/jffs/mlclash`、`/jffs/mlclash/data` |
| Merlin 运行日志 | `/tmp/mlclash/logs` |
| koolshare 安装/数据 | `/koolshare/mlclash` |

面板和 API 面向局域网使用，不要把 9380 或 9090 暴露到 WAN。

### 去广告

只有一个开关：「设置 → 国内 IP 规则 → 拦截广告与统计域名」。默认关闭，仅在 rule 模式生效。

- 主列表：anti-AD `mihomo.mrs`（约 10 万条，约 0.85MB），由面板下载。
- 兜底：内置 `category-ads-all`（905 条），增强列表未就绪时仍能拦截。
- 列表跟随规则更新调度（`ruleAutoUpdate` / `ruleIntervalH`，默认 24 小时），也可在规则页「立即更新」。
- 域名级过滤无法处理第一方/信息流广告、YouTube 广告、与内容同域的广告，以及没有可用 SNI/HTTP Host 的请求。

### IPv6

LAN 的 IPv4 和 IPv6 TCP 流量都会重定向到 mihomo。IPv6 链绕过 `::1`、ULA、链路本地、组播和 LAN `/64`；IPv6 QUIC（UDP/443）被 REJECT，迫使回落 TCP。IPv6 覆盖依赖 `ip6tables`，已验证固件自带。

### 从源码构建

依赖 Go 1.23+、curl、tar、gzip。

```sh
./build.sh                  # 完整构建：下载 slim 内核与 chnroute，编译面板并打 koolshare 包
./build.sh --no-download    # 使用 .cache，不联网
./tools/build-merlin-package.sh
```

常用参数：`--kernel <path>`、`NOKERNEL=1`、`PANEL_UPX=1`。
内核来源：[fanghengxuan/mihomo](https://github.com/fanghengxuan/mihomo) 的 `slim-*` Release。
前端为原生 HTML/CSS/JS，通过 `go:embed` 打包，不需要 Node.js 构建。

### 测试

```sh
go test -race ./...
python3 tests/ui_check.py
sh tests/merlin_install_smoke.sh
```

其他冒烟测试需要先构建 `dist/stage` 或安装包：`tests/install_smoke.sh`、`tests/nokernel_test.sh`、`tests/cleanup_test.sh`。`tests/e2e.sh` 需要本机 darwin mihomo，属可选项。

### 项目结构

```
api.go        HTTP API 与鉴权
config.go     mihomo 配置生成
engine.go     进程、iptables/ip6tables、重载与看门狗
subs.go       订阅拉取与解析
ruleset.go    国内 IP 规则源与调度
geodata.go    geosite/geoip 下载与校验
antiad.go     anti-AD mrs 下载与校验
web/          内嵌面板
plugin/       Merlin 与 koolshare 安装脚本
tools/        打包工具
tests/        冒烟与 UI 检查
docs/API.md   API 文档
```

### 文档

- [API 文档](docs/API.md)
- [内核说明](docs/kernel.md)
- [更新记录](CHANGELOG.md)
- [贡献指南](CONTRIBUTING.md)

### 故障排查

- **引擎未运行**：`sh /jffs/mlclash/ctl.sh status`；查看 `/tmp/mlclash/logs/panel.log` 与 `mihomo.log`。
- **面板打不开**：检查面板进程和 9380 端口；`ctl.sh restart`。
- **仍有广告**：确认 `mode=rule`、开关已开、anti-AD 状态为 `ready`；重启 App 清缓存。第一方/信息流/YouTube 广告无法用域名规则拦截。
- **空间不足**：在面板执行存储清理，删除无用内核备份；必要时使用 `NOKERNEL=1` 包。

### 安全

- 面板默认无密码；局域网不可信时请设置密码。
- 备份包含订阅地址和令牌，请妥善保存。
- mihomo API 仅监听 `127.0.0.1`，并使用随机 secret。

### 许可

MIT。mihomo 为 GPL-3.0；geosite/geoip 数据来自 MetaCubeX/meta-rules-dat、anti-AD、Loyalsoldier 及其上游，第三方组件和数据遵循各自许可。
