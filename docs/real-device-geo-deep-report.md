# 2.0.6 GEO 规则全面深度测试报告（真机）

- 日期：2026-11-01
- 设备：TUF-AX3000（Asuswrt-Merlin / gnuton 388.11，armv7，512MB RAM，`/jffs` 47MB）
- 面板：2.0.6（Release v2.0.6，面板二进制 `9117922d…`）
- 基线：mode=rule、rulePreset=cn-direct、geoipEnable=true、geosite+geoip 已就绪
- 原始结果：`/tmp/mm_rd/evidence/geo_deep_results.json`、`geo_deep_recheck.json`、`geo_ui_results.json`、`geo_sched_results.json`

## 1. 结论摘要

| 项目 | 结果 |
|---|---|
| 规则/路由正确性 | 通过 |
| GEOIP 开关与持久化 | 通过 |
| mode / rulePreset 回归 | 通过 |
| 手动规则更新 | 通过（IP 3887 + geosite 110177 + geoip 7.65MB，24.8s） |
| 非法配置 / 启动竞态 | 通过 |
| 性能 | 通过（0.83s / 2.09s） |
| 内存/fd | 通过（无持续增长、无 `mihomo -t`/`.tmp` 残留） |
| UI | 通过（7/7） |
| 10 分钟自动调度 | 通过（IP + geosite + geoip 全部自动刷新） |
| 产品缺陷 | **本轮未发现新缺陷** |

补充：主套件自报 45/50，其中 5 个 FAIL 为 2 个探针目标问题 + 3 个测试脚本 `check()` 少传参数；补测脚本 8/9（仅 direct 国外 IP 探针受被墙超时限制，内核日志已证明 `dial DIRECT`）、UI 7/7、调度器实际检查全部通过。

主套件自报 45/50，5 个 FAIL 全部是**测试探针/脚本问题**，已逐条复验：

1. `101.68.142.6` 是路由器 WAN 公网 IP，LAN 访问走 hairpin NAT，mihomo 看不到连接 → 换 `221.15.71.64` / `112.83.140.11` 复验通过。
2. `mode=direct` 下国外裸 IP 连接被墙，连接瞬断抓不到连接行 → 内核日志明确 `dial DIRECT ... i/o timeout`，属探针限制。
3. 三处 `check()` 少传参数导致脚本异常（manual update / restart / final state）→ 补测脚本确认引擎、规则、最终状态全部正常。

## 2. 路由矩阵（连接链）

| 目标 | 规则 | 结果 |
|---|---|---|
| CN IP `123.234.3.167` 裸 TCP | GeoIP cn | DIRECT |
| CN IP `221.15.71.64` 裸 TCP | GeoIP cn | DIRECT |
| CN IP `112.83.140.11` 裸 TCP | GeoIP cn | DIRECT |
| 国外 IP `142.250.1.1` 裸 TCP | Match | 机场节点 |
| SNI `www.google.com` → 国外 IP | Match | 机场节点 |
| SNI `www.bilibili.com` → 境外 DoH IP `103.151.151.133` | GeoSite cn | DIRECT |
| SNI `www.bilibili.com` → 国内 IP `123.234.3.167` | GeoSite cn | DIRECT |
| 境外 DoH IP `103.151.151.133` 裸 TCP | RuleSet cn-ip | DIRECT |

规则命中计数：GeoIP cn hitCount 68+，GeoSite cn hitCount 195+；规则顺序 `GeoSite → GeoIP → RuleSet → IPCIDR → Match`。

## 3. GEOIP 开关

- 关闭：config 移除 `GEOIP,CN,DIRECT`，内核规则无 GeoIP；CN 裸 IP 回落到 `RuleSet cn-ip → DIRECT`；GeoSite 仍生效。
- 重新开启：GeoIP 规则恢复，CN 裸 IP 重新命中 `GeoIP cn → DIRECT`。
- 设置持久化：重启面板/内核后 `geoipEnable=true`、geosite/geoip 均 ready。

## 4. 模式与规则预设

| 模式/预设 | 结果 |
|---|---|
| global | `modeActual=global`、`GLOBAL.now=🚀 节点选择`；CN IP 走代理（链含 GLOBAL） |
| direct | `modeActual=direct`；CN IP DIRECT；国外 IP 内核日志 `dial DIRECT` |
| rule + cn-direct | GeoIP/GeoSite/cn-ip 规则恢复 |
| rule + proxy-all | 仅 `MATCH,🚀 节点选择`；CN IP 走代理；恢复 cn-direct 后规则回来 |

## 5. 更新与容错

- 手动 `/api/rules/update`（preset chnroutes2）：200，24.8s；IP meta 刷新；geosite 110177 条、geoip 7.65MB 均已就绪；无 `geositeError`/`geoipError`；引擎运行、规则顺序正确。
- 非法配置（fake-ip 网段 `not-a-cidr`）：返回 500，但引擎保持运行、旧规则配置还原。
- `ctl.sh restart` 后立即保存设置：等待内核 API 就绪 8.4s 后返回 200，无瞬时 500；geosite/geoip ready，引擎自动恢复。

## 6. 性能 / 内存

- 设置保存：
  - 不影响内核配置（subUA）：**0.83s**
  - 改变配置（logLevel）：**2.09s**
- 10 次配置热重载 + 3 次内核重启 + 静置：
  - 面板 RSS：12.4MB → 峰值 13.3MB → 最终 14.8MB
  - 内核 RSS：60.8MB → 峰值 75.4MB（热重载）→ 重启后 54.7MB → 最终 55.4MB
  - 线程 9（面板）/ 8（内核）稳定；面板 fd 12→13；内核 fd 92→51
  - 无 `mihomo -t` 残留、无 `.tmp` 文件；引擎运行
- 下载/批量操作后面板 RSS 会短时升高，重启面板即回落；未发现内存泄漏。

## 7. 10 分钟自动调度

把 `rules/meta.json`、`geosite.json`、`geoip.json` 的 `updatedAt` 回拨 2 小时，`ruleAutoUpdate=true`、`ruleIntervalH=1`，等待调度 tick。

- 结果（10:55:36 tick，全部通过）：
  - IP meta：`1793494223 → 1793501708`
  - geosite：`1793494223 → 1793501726`
  - geoip：`1793494223 → 1793501736`
  - 规则保持 `GEOSITE,cn → GEOIP,CN → RULE-SET,cn-ip`，引擎运行。
- 说明：调度器脚本最后一行 `check()` 少传参数导致脚本中断，`ruleIntervalH` 由人工恢复为 24；产品侧自动更新本身全部成功。

## 8. UI

Playwright 实测 7/7：

- 规则页有「GEOIP,CN 直连」开关，初始开启；
- 显示 `GEOIP 数据库：已下载 · 7.30 MB` 与更新时间；
- UI 关闭并保存 → `/api/settings.geoipEnable=false`，GeoIP 规则移除；
- UI 重新开启并保存 → API true，geoip ready，开关保持选中。

## 9. 存储

- `/jffs` 47MB，当前（geosite + geoip 均存在）可用约 24.6MB；
- 升级安装需约 17.3MB，余量正常；
- 发布包不包含任何 GEO 文件，安装后在线下载/更新。

## 10. 限制与说明

- `101.68.142.6` 是 WAN 公网 IP，不适合做 LAN 侧透明代理探针（hairpin NAT）。
- `mode=direct` 下被墙的国外 IP 连接会超时，连接行很短；用内核 `dial DIRECT` 日志确认路由。
- 本轮主套件的 3 个 FAIL 是测试脚本 `check()` 参数缺失，不是产品问题；补测已覆盖。
- B 站 Akamai 视频域名仍不属于 `geosite cn`，境外解析时可能仍走代理（与 FlClash 默认 geosite 行为一致）。
