<p align="center">
  <img src="docs/brand/hero.jpg" alt="MlClash — Make Mihomo Easier on Your Router." width="100%">
</p>

<p align="center">
  <a href="https://github.com/fanghengxuan/MlClash/actions/workflows/ci.yml"><img src="https://github.com/fanghengxuan/MlClash/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/fanghengxuan/MlClash/releases"><img src="https://img.shields.io/github/v/release/fanghengxuan/MlClash" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <img src="https://img.shields.io/badge/platform-ASUS%20Merlin%20%2F%20armv7-4F8CFF" alt="Platform">
</p>

<p align="center"><a href="#中文">中文</a> · <a href="#english">English</a></p>

---

<a id="中文"></a>

## 中文

### 项目简介

**MlClash** 是运行在华硕梅林（Asuswrt-Merlin / gnuton）路由器上的 mihomo 代理插件。它把 mihomo 内核、IPv4/IPv6 透明代理和一套完整的 Web 面板打包成一个安装包，直接装在路由器上：

- 不再需要旁路由、Docker 或额外的控制器；
- 局域网设备无需配置，连上 Wi-Fi 即走代理；
- 订阅、节点、规则、DNS、去广告、备份和内核更新都在浏览器里完成。

> **Make Mihomo Easier on Your Router.** — 让 mihomo 在路由器上开箱即用。

### 核心功能

| 能力 | 说明 |
|---|---|
| **路由器级透明代理** | IPv4 与 IPv6 TCP 流量经 `iptables` / `ip6tables` REDIRECT 到 mihomo，LAN 设备零配置 |
| **订阅与节点管理** | 链接导入或粘贴配置，支持包含/排除正则、单订阅更新间隔、手动选择与自动选择 |
| **节点出口质量** | 订阅更新后后台检测每个节点的出口 IP，节点页显示「良好 / 一般 / 有风险」，自动选择自动排除风险节点 |
| **规则与分流** | `rule` / `global` / `direct` 三种模式；国内直连预设（GeoSite / GeoIP / chnroute） |
| **去广告** | anti-AD `mihomo.mrs`（约 10 万条）为主，内置 `category-ads-all` 兜底 |
| **内核运维** | 内核在线更新、上传、回滚；配置热重载；定时重启与看门狗 |
| **Web 面板** | 概览、节点、订阅、规则、连接、日志、设置；SSE 实时日志与流量 |
| **数据与安全** | JSON 备份恢复、存储清理、面板密码、局域网访问控制 |

### 界面预览

<p align="center">
  <img src="docs/screenshots/overview.png" width="49%" alt="概览">
  <img src="docs/screenshots/nodes.png" width="49%" alt="节点">
</p>
<p align="center">
  <img src="docs/screenshots/subs.png" width="49%" alt="订阅">
  <img src="docs/screenshots/settings.png" width="49%" alt="设置">
</p>

### 功能与特性

**代理内核**

- mihomo slim armv7 构建，`rule` / `global` / `direct` 三种模式
- 配置由面板生成并热重载；配置不变时跳过内核重新解析
- 内核上传、在线更新、失败回滚

**订阅与节点**

- 订阅链接或直接粘贴 Clash 配置；支持 Base64 / URI 列表等常见格式
- 单订阅包含/排除正则、独立更新间隔
- 节点分组：手动选择、URL 测速自动选择
- **节点出口质量**：订阅更新完成后逐个探测出口 IP，记录「良好 / 一般 / 有风险 / 未测」；「自动选择」会排除被公开库标记为代理/VPN/Tor 的节点，组内仍按延迟选择

**规则与分流**

- 国内直连预设：`GEOSITE,cn` → 可选 `GEOIP,CN` → `RULE-SET,cn-ip`
- Apple / Google 中国域名单独直连
- 规则页可查看命中情况，支持规则集手动更新

**DNS**

- `redir-host` 或 `fake-ip`
- 可配置上游 DNS 与 fallback；fake-ip 网段可改

**IPv6**

- LAN IPv4 与 IPv6 TCP 流量都重定向到 mihomo
- IPv6 链绕过 `::1`、ULA、链路本地、组播与 LAN `/64`
- IPv6 QUIC（UDP/443）REJECT，迫使回落 TCP

**去广告**

- 一个开关，默认关闭，仅在 `rule` 模式生效
- 主列表 anti-AD `mihomo.mrs`，内置 `category-ads-all` 兜底
- 跟随规则更新调度（默认 24 小时），也可在规则页立即更新
- 域名级过滤无法处理第一方/信息流广告、YouTube 广告和没有可用 SNI/HTTP Host 的请求

**面板**

- 概览：连接状态、实时流量、模式切换、订阅用量、服务连通性
- 节点：搜索、排序、单节点测速、整组测速、节点质量
- 订阅：自动更新状态、流量/到期、导入与原始内容
- 规则、连接、日志、设置；支持深浅色主题、强调色、纯黑模式和低端设备模式

**运维**

- 开机自启；面板守护与内核看门狗
- 定时重启（按小时间隔或每天固定时刻）
- 面板密码；局域网访问控制
- 日志级别可调，SSE 实时日志
- JSON 备份恢复、存储清理

### 支持环境

| 项目 | 状态 |
|---|---|
| 已验证设备 | **TUF-AX3000**（axhnd.675x / armv7 / 内核 4.1） |
| 已验证固件 | **gnuton Asuswrt-Merlin** |
| koolshare / SWRT | 安装包通过构建与冒烟测试，**未在真机验证** |
| CPU 架构 | armv7（`GOARM=7`）；其他架构需自行交叉编译 |
| JFFS | 需启用自定义脚本；安装至少需要 **18 MB** 可用空间（解压约 16.3 MB + 2 MB 余量） |
| 内存 | 建议 512 MB 及以上 |
| 安装通道 | Merlin 原生安装需 SSH；koolshare 通过软件中心离线安装 |

> 真实性说明：上表只标注实际验证过的设备与固件。其他华硕 armv7 机型理论可用，但未逐台验证。

### 安装指南

#### Asuswrt-Merlin / gnuton

1. 从 [Releases](https://github.com/fanghengxuan/MlClash/releases) 下载 `MlClash_<版本>_armv7_merlin.tar.gz`。
2. 在路由器开启 **系统管理 → 系统设置 → Enable JFFS custom scripts and configs**。
3. 上传并安装：

```sh
cd /tmp
tar -xzf MlClash_<版本>_armv7_merlin.tar.gz
sh /tmp/MlClash/install.sh
```

4. 浏览器打开 `http://<路由器IP>:9380`。

#### koolshare / SWRT

下载 `MlClash_<版本>_armv7_koolshare.tar.gz`，在软件中心离线安装。手动安装：

```sh
cd /tmp
tar -xzf MlClash_<版本>_armv7_koolshare.tar.gz
sh /tmp/mlclash/install.sh
```

#### 升级

直接在现有安装上覆盖安装新版本。安装器会停止服务、更新面板与内核、保留数据/订阅/规则，然后重新启动面板。

#### 卸载

```sh
sh /jffs/mlclash/uninstall.sh              # 删除程序和全部数据
sh /jffs/mlclash/uninstall.sh --keep-data  # 保留数据
```

### 首次使用

1. 打开面板，在 **订阅** 页添加订阅。
2. 在 **节点** 页选择分组或节点；订阅更新完成后会自动检测节点质量。
3. 在 **概览** 页启动引擎。
4. 可选：在 **设置 → 国内 IP 规则** 开启「拦截广告与统计域名」。
5. 可选：在 **设置 → 启动与访问** 设置面板密码。

### 端口与数据目录

| 项目 | 默认值 |
|---|---|
| 面板 | `0.0.0.0:9380` |
| HTTP/SOCKS 混合端口 | `7890` |
| 透明代理 redir 端口 | `3333` |
| mihomo API | `127.0.0.1:9090` |
| Merlin 安装 / 数据 | `/jffs/mlclash`、`/jffs/mlclash/data` |
| Merlin 运行日志 | `/tmp/mlclash/logs` |
| koolshare 安装 / 数据 | `/koolshare/mlclash` |

面板和 API 面向局域网使用，不要把 `9380` 或 `9090` 暴露到 WAN。

### 更新日志

| 版本 | 主要内容 |
|---|---|
| **3.2.0** | 节点出口质量展示；自动选择排除风险节点；修复 `exclude-filter` 子串误排除 |
| 3.1.2 | 代码布局重构，53 个 `.go` 与前端移入 `cmd/mlclash-panel/`，无功能变更 |
| 3.1.1 | IPv6 透明代理（`ip6tables`），修复 IPv6 流量绕过代理 |
| 3.1.0 | 去广告合并为单一开关，anti-AD 主列表 + 内置兜底 |

完整记录见 [CHANGELOG.md](CHANGELOG.md)。

### 常见问题

**面板打不开？**
检查面板进程与 `9380` 端口，执行 `sh /jffs/mlclash/ctl.sh restart`。

**引擎未运行？**
执行 `sh /jffs/mlclash/ctl.sh status`，查看 `/tmp/mlclash/logs/panel.log` 与 `mihomo.log`。

**节点质量一直显示「未测」？**
质量检测在订阅更新完成后触发，且需要内核正在运行。手动刷新一次订阅即可。

**自动选择的节点变少了？**
3.2.0 起，被公开库标记为代理/VPN/Tor 的「有风险」节点会被排除出自动选择。它们仍保留在订阅子分组里，可手动选择。

**仍有广告？**
确认 `mode=rule`、去广告开关已开、anti-AD 状态为 `ready`；重启应用清缓存。第一方、信息流和 YouTube 广告无法用域名规则拦截。

**空间不足？**
在面板执行存储清理，删除无用内核备份；必要时使用 `NOKERNEL=1` 构建不带内核的包。

### 参与项目

- **反馈问题**：[GitHub Issues](https://github.com/fanghengxuan/MlClash/issues)
- **贡献代码**：[CONTRIBUTING.md](CONTRIBUTING.md)
- **接口文档**：[docs/API.md](docs/API.md)
- **内核说明**：[docs/kernel.md](docs/kernel.md)

从源码构建：

```sh
./build.sh                  # 下载 slim 内核与 chnroute，编译面板并打 koolshare 包
./build.sh --no-download    # 使用 .cache，不联网
./tools/build-merlin-package.sh
```

运行测试：

```sh
go test -race ./...
python3 tests/ui_check.py
sh tests/merlin_install_smoke.sh
```

内核来源：[fanghengxuan/mihomo](https://github.com/fanghengxuan/mihomo) 的 `slim-*` Release。前端为原生 HTML/CSS/JS，通过 `go:embed` 打包，不需要 Node.js 构建。

### 开源协议

[MIT](LICENSE)。mihomo 为 GPL-3.0；GeoSite/GeoIP 数据来自 MetaCubeX/meta-rules-dat、anti-AD、Loyalsoldier 及其上游，第三方组件与数据遵循各自许可。

---

<a id="english"></a>

## English

### Overview

**MlClash** is a mihomo-based proxy plugin for ASUS Merlin routers. It ships the mihomo core, transparent IPv4/IPv6 forwarding and a complete web panel in one installable package. No side router, Docker container or external controller is required — LAN devices go through the proxy without any client configuration.

> **Make Mihomo Easier on Your Router.**

### Highlights

| Capability | Details |
|---|---|
| **Router-level transparent proxy** | IPv4 and IPv6 TCP traffic redirected to mihomo via `iptables` / `ip6tables`; zero client setup |
| **Subscriptions & nodes** | URL import or pasted config, include/exclude regex, per-subscription intervals, manual and URL-test groups |
| **Exit IP quality** | After each subscription refresh, every node's exit IP is checked; the nodes page shows Good / Normal / Risky, and the URL-test group excludes risky nodes |
| **Rules & routing** | `rule` / `global` / `direct` modes; China-direct preset (GeoSite / GeoIP / chnroute) |
| **Ad blocking** | anti-AD `mihomo.mrs` (~100k domains) with a built-in `category-ads-all` fallback |
| **Kernel operations** | Online update, upload, rollback, hot reload, scheduled restart and watchdog |
| **Web panel** | Overview, nodes, subscriptions, rules, connections, SSE logs and settings |
| **Data & security** | JSON backup/restore, storage cleanup, panel password, LAN access control |

### Screenshots

<p align="center">
  <img src="docs/screenshots/overview.png" width="49%" alt="Overview">
  <img src="docs/screenshots/nodes.png" width="49%" alt="Nodes">
</p>
<p align="center">
  <img src="docs/screenshots/subs.png" width="49%" alt="Subscriptions">
  <img src="docs/screenshots/settings.png" width="49%" alt="Settings">
</p>

### Features

- mihomo slim armv7 core with `rule`, `global` and `direct` modes
- Subscription import from URL or pasted config, with include/exclude regex and per-subscription intervals
- Node groups: manual select and URL test
- Exit IP quality checks after subscription refresh; risky nodes are excluded from the URL-test group
- Transparent TCP proxy for IPv4 and IPv6 via `iptables` / `ip6tables` REDIRECT
- IPv4/IPv6 QUIC blocking to force TCP fallback
- China direct preset: `GEOSITE,cn` + optional `GEOIP,CN` + `RULE-SET,cn-ip`; Apple/Google China domains direct
- Optional ad blocking: anti-AD `mihomo.mrs` with the built-in `category-ads-all` fallback
- Optional DNS: `redir-host` or `fake-ip`, configurable upstreams
- Panel: overview, nodes, subscriptions, rules, connections, SSE logs, settings
- Backup/restore (JSON), storage cleanup, kernel upload, online update and rollback
- Auto-start, scheduled restart, panel password, LAN access control, log levels
- Light/dark themes, accent colors, pure-black and low-end device modes

### Requirements

| Item | Status |
|---|---|
| Verified device | **TUF-AX3000** (axhnd.675x / armv7 / kernel 4.1) |
| Verified firmware | **gnuton Asuswrt-Merlin** |
| koolshare / SWRT | Build and smoke tested, **not verified on hardware** |
| CPU | armv7 (`GOARM=7`); other architectures require cross-compiling |
| JFFS | Custom scripts enabled; at least **18 MB** free (about 16.3 MB expanded + 2 MB headroom) |
| RAM | 512 MB or more recommended |
| Install path | SSH for the Merlin installer; software center for koolshare |

Only the hardware and firmware above are marked as verified. Other ASUS armv7 models should work in theory but have not been tested individually.

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

#### Upgrade and uninstall

Install a newer package over the existing installation. The installer stops the service, updates the panel and kernel, keeps data, subscriptions and rules, then restarts the panel.

```sh
sh /jffs/mlclash/uninstall.sh              # remove program and data
sh /jffs/mlclash/uninstall.sh --keep-data  # keep data
```

### First run

1. Open the panel and add a subscription under **Subscriptions**.
2. Select a group or node under **Nodes**; quality checks run after the subscription refreshes.
3. Start the engine from **Overview**.
4. Optional: enable **Settings → China IP rules → Block ads and tracking domains**.
5. Optional: set a panel password under **Settings → Startup & access**.

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

The panel and API are intended for LAN use. Do not expose ports `9380` or `9090` to the WAN.

### Changelog

| Version | Highlights |
|---|---|
| **3.2.0** | Exit IP quality display; risky nodes excluded from URL-test; anchored `exclude-filter` fix |
| 3.1.2 | Code layout refactor; no functional changes |
| 3.1.1 | IPv6 transparent proxy (`ip6tables`) |
| 3.1.0 | Single ad-blocking switch with anti-AD primary list |

See [CHANGELOG.md](CHANGELOG.md) for the full history.

### FAQ

**Panel unreachable?** Check the panel process and port `9380`, then run `sh /jffs/mlclash/ctl.sh restart`.

**Engine not running?** Run `sh /jffs/mlclash/ctl.sh status` and check `/tmp/mlclash/logs/panel.log` and `mihomo.log`.

**Quality still shows "untested"?** Checks run after a subscription refresh and require the engine to be running. Refresh a subscription manually.

**Fewer nodes in the auto group?** Since 3.2.0, nodes marked as proxy/VPN/Tor are excluded from the URL-test group. They remain available in the subscription groups for manual selection.

**Ads still visible?** Confirm `mode=rule`, ad blocking is on and anti-AD is `ready`; restart the app to clear caches. First-party, in-feed and YouTube ads cannot be blocked by domain rules.

**Low space?** Run storage cleanup in the panel, remove unused kernel backups, or build with `NOKERNEL=1`.

### Contributing

- [Issues](https://github.com/fanghengxuan/MlClash/issues)
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [API reference](docs/API.md)
- [Kernel notes](docs/kernel.md)

```sh
./build.sh                  # fetch the slim kernel and chnroute, build the panel, package koolshare
./build.sh --no-download    # use .cache, no network
./tools/build-merlin-package.sh
```

```sh
go test -race ./...
python3 tests/ui_check.py
sh tests/merlin_install_smoke.sh
```

The kernel comes from [fanghengxuan/mihomo](https://github.com/fanghengxuan/mihomo) `slim-*` releases. The panel is plain HTML/CSS/JS embedded with `go:embed`; there is no Node.js build step.

### License

[MIT](LICENSE). mihomo is GPL-3.0. GeoSite/GeoIP data comes from MetaCubeX/meta-rules-dat, anti-AD, Loyalsoldier and their upstream sources. Third-party components and data remain under their respective licenses.
