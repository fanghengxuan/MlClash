# 2.0.6 国内域名/IP 直连（GEOSITE + 可选 GEOIP）真机报告

- 日期：2026-11-01
- 设备：TUF-AX3000（Asuswrt-Merlin / gnuton 388.11，armv7，512MB RAM，`/jffs` 47MB）
- 面板：2.0.6，内核 slim v1.19.33
- 目的：解决「国内站点（如 bilibili）走到机场代理」；做法对齐 FlClash 的 geodata + GEOSITE，并补一个可选的 GEOIP,CN

## 1. 问题与复现

原规则只有 IP 匹配：

```text
RULE-SET,cn-ip,DIRECT
IP-CIDR,192.168.0.0/16,DIRECT
...
MATCH,🚀 节点选择
```

客户端使用路由器 DNS 时，bilibili 解析到国内 IP，`cn-ip` 命中 DIRECT；但用境外 DoH/ECS 时解析结果不同：

| 解析方式 | bilibili 解析结果 | 旧行为 |
|---|---|---|
| 路由器本地 DNS | `123.234.x / 221.15.x / 112.83.x` | RuleSet cn-ip → DIRECT |
| Cloudflare / Google DoH | `103.151.151.x / 164.52.1.x`（不在列表） | **Match → 机场节点** |
| B 站 Akamai 视频边缘 | `23.55.179.10`（不在列表） | **Match → 机场节点** |

根因：规则模式只有 IP-CIDR，没有域名匹配。国内域名只要解析到列表外 IP，就会被 `MATCH` 判给代理。

## 2. FlClash 的 geo 资源与我们的差异

FlClash 内置/管理四类 MetaCubeX geodata：

| 资源 | FlClash 内置大小 | 用途 |
|---|---|---|
| `GEOSITE.dat` | 4.4MB | 域名规则 `GEOSITE,cn` |
| `GEOIP.dat` | 20.5MB | IP 规则 `GEOIP,CN` |
| `GEOIP.metadb` | 9.6MB | MMDB 形式 GeoIP |
| `ASN.mmdb` | 10.8MB | `IP-ASN` 等 |

FlClash 是「内置全部 geo 文件 + 核心在线更新」；它的规则编辑器支持 `GEOSITE / GEOIP / IP-ASN / SRC-GEOIP / SRC-IP-ASN`。它**不使用** Loyalsoldier/17mon 这类 CIDR 列表作为默认 IP 源。

我们的 2.0.6（受 47MB jffs 限制）：

- **默认**：在线下载 `GEOSITE.dat`（4.35MB，110,177 条 cn 域名），生成 `GEOSITE,cn,DIRECT`；IP 继续用 `cn-ip` CIDR（3887 条）兜底。
- **可选开关（默认关闭）**：下载 `country.mmdb`（7.65MB，存为 `geoip.metadb`），生成 `GEOIP,CN,DIRECT`。
- 不下载 20.5MB `GEOIP.dat`、10.8MB ASN；不打包任何 geo 文件进安装包。

## 3. 2.0.6 实现

- 规则顺序（开启 GEOIP 时）：
  ```text
  GEOSITE,cn,DIRECT
  GEOIP,CN,DIRECT       # 可选，开启后存在
  RULE-SET,cn-ip,DIRECT
  IP-CIDR,192.168.0.0/16,DIRECT
  ...
  MATCH,🚀 节点选择
  ```
- 配置加入 FlClash 同款 geodata 键：
  - `geodata-loader: memconservative`
  - `geox-url`（jsdelivr 镜像）
  - `geo-auto-update: false`（由面板调度更新）
- 面板下载、校验、更新：
  - `geosite.dat`：`mihomo -t` 真加载 `GEOSITE,cn` 校验，写 `.geosite.stamp`；失败保留旧文件
  - `geoip.metadb`：校验文件大小与 MaxMind DB 元数据标记，写 `.geoip.stamp`；失败保留旧文件
  - 首次安装后台补齐；与 IP 规则共用 `ruleAutoUpdate` / `ruleIntervalH` 调度
- 规则页显示域名规则/GEOIP 数据库状态；`/api/rules/set`、`/api/rules/update` 返回 `geosite`/`geoip`
- 性能/稳定性：
  - Reload 不再重复跑 `mihomo -t`；按「内核上次成功加载的配置哈希」判断是否跳过内核重载
  - 设置保存：不改变配置的项约 0.6–0.9s；配置变化项约 2.7–3.1s
  - 启动窗口内改设置会等待内核 API 就绪，不再瞬时 500

## 4. 真机验证

| 场景 | 结果 |
|---|---|
| 境外 DoH IP `103.151.151.133` + SNI `www.bilibili.com` | **GeoSite cn → DIRECT**（修复前 Match → 机场） |
| 裸国内 IP `123.234.3.167`（无 SNI） | RuleSet cn-ip → DIRECT |
| 开启 GEOIP 后，裸 CN IP | **GeoIP cn → DIRECT** |
| 裸国外 IP `142.250.1.1` / `www.google.com` | Match → 机场节点 |
| 首次安装 | 自动下载 geosite，`ready=true, count=110177` |
| GEOIP 开关开启 | 自动下载 `country.mmdb` 7,651,117 字节，`ready=true`，规则加入 GEOIP |
| GEOIP 开关关闭 | 规则移除，数据库文件保留复用（本次测试后已清理以节省 jffs） |
| 手动「立即更新」 | IP 3887 条 + 域名 110177 条（开启时含 GEOIP）同时更新 |
| 非法配置 | 500，但引擎保持运行、旧配置还原 |
| 启动窗口内保存设置 | 等待内核 API 就绪后 200 |

## 5. 存储

| 项目 | 大小 |
|---|---|
| `/jffs` 总量 / 可用（仅 GEOSITE） | 47MB / **28.3MB** |
| `geosite.dat` | 4.35MB |
| `geoip.metadb`（可选） | 7.65MB |
| 两者都下载后可用空间 | **24.6MB** |
| 升级安装要求 | 约 17.3MB |
| FlClash 全套 geo | ≈45MB（**放不下**） |

结论：安装包不包含 geo 文件；默认只在线下载 4.35MB geosite；GEOIP 是可选项，开启后仍有 24.6MB 可用，不下载 20.5MB 的 GEOIP.dat / 10.8MB ASN。

## 6. 内存

- 仅 GEOSITE：内核 RSS 约 55–60MB（无 geosite 时约 48.5MB）
- 开启 GEOIP 并命中后：约 59.5MB；关闭后规则移除，文件不占内存
- 干净重启后 4 分钟静置（无业务、无浏览器）：
  - 面板 RSS 9.6 → 14.2MB 后趋平，fd 13 → 20，socket 7 → 14（平台化）
  - 内核 RSS 57.1 → 58.3MB，fd 65–85 波动
- 下载/大批量 UI 操作后面板 RSS 会短时升高，重启面板即回落到 10MB 左右；未发现持续泄漏。

## 7. 限制

- B 站视频的 `upos-hz-mirrorakam.akamaized.net` 是 Akamai 域名/海外 IP，不属于 `geosite cn`；用境外 DNS 时仍可能被代理（FlClash 默认 geosite 同样覆盖不到）。
- 纯 IP、无 SNI 的连接只能靠 `GEOIP,CN`（可选）或 `cn-ip` CIDR 兜底；两者覆盖范围有差异。
- 不提供 `GEOIP.dat`、ASN、IP-ASN；路由器 jffs 与升级峰值放不下 FlClash 的 45MB 全套。
- GEOIP 数据库默认不保留：曾开启并测试后已删除，保持默认关闭时 jffs 可用 28.3MB。
