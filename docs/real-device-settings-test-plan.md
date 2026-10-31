# mihomo-merlin 设置项真机验证计划

- 日期：2026-11-01
- 目标路由器：TUF-AX3000（Asuswrt-Merlin / gnuton 388.11），LAN `192.168.50.1`
- 面板：v2.0.4（提交 `1285672`），`http://192.168.50.1:9380`
- 验证原则：每个设置项都必须同时证明 **①请求成功 ②设置持久化 ③生成配置变化 ④运行时/网络行为真实变化**；只看到「保存成功」不算通过。
- 对照实现：FlClash（`chen08209/FlClash`，本地浅克隆 `/tmp/flclash`）。遇到行为异常先查 FlClash 同名能力。

## 0. 安全与回滚

1. 测试前抓取完整基线：`settings.json`、`config.json`、`subs.json`、`rules/meta.json`、`restart.json`、iptables nat/mangle、监听端口、进程状态。
2. 每项测试只改一个变量；测试后立即恢复基线值并复核。
3. 高风险项（会断网/重启内核）集中放在独立阶段：定时重启、autoStart、内核上传/回滚、备份恢复。
4. 定时重启、订阅/规则定时器等需要等待的项，先缩短到最短合法周期；不修改系统时间。
5. 任何测试导致面板/内核不可达，先通过 SSH `ctl.sh restart` 恢复，再继续。
6. 最终恢复基线设置并重启内核，确认代理恢复。

## 1. 验证矩阵

| # | 子页 | 设置项 | 真机验证方法 | 通过标准 |
|---|---|---|---|---|
| 1 | 代理与端口 | `mode` | rule/global/direct 三态切换；读 `/api/status.engine.modeActual`；从 LAN 客户端访问 IP 回显，比较出口 IP | actual 跟随；direct 出口=WAN，global/rule 出口=代理（取决于规则） |
| 2 | 代理与端口 | `rulePreset` | cn-direct / proxy-all；读生成的 `rules` 与 mihomo `/providers/rules`；访问国内 IP 站点 | cn-direct 含 `RULE-SET,cn-ip,DIRECT` 且国内 IP 直连；proxy-all 无该规则 |
| 3 | 代理与端口 | `redirPort` | 改端口后看 iptables REDIRECT、`/api/status`、LAN 访问 | 规则指向新端口；LAN 流量仍可代理 |
| 4 | 代理与端口 | `mixedPort` | 改端口后从 LAN 用 HTTP/SOCKS 代理访问回显 | 新端口可代理；旧端口不再监听 |
| 5 | 代理与端口 | `allowLan` | 关闭后从 LAN 连 mixed 端口；开启后重试 | 关闭=拒绝/不可达；开启=可用 |
| 6 | 代理与端口 | `blockQuic` | 开关后查 `iptables FORWARD udp/443 REJECT`；LAN 发 UDP/443 测试 | 规则存在/不存在；阻断时 UDP 443 不通 |
| 7 | 代理与端口 | `sniffer` | 开关后查 `config.json.sniffer`；内核重载成功；抓日志 | 配置段出现/消失，内核接受 |
| 8 | 代理与端口 | `routerSelf` | 开关后查 nat `OUTPUT/MIHOMO_OUT`；SSH 到路由器 curl 出口 IP | 开启有 OUTPUT 跳转且路由器出口变代理；关闭无跳转 |
| 9 | 代理与端口 | `lanIf` | 设为错误接口再改回；查 PREROUTING 规则；LAN 访问回显 | 错误接口时 LAN 不被重定向；br0 时恢复 |
| 10 | DNS | `dnsEnable` | 开关后查 `config.json.dns` 是否存在 | 开关与配置一致，内核接受 |
| 11 | DNS | `dnsMode` | redir-host / fake-ip；查配置；从 LAN 向路由器 DNS 监听端口查询域名 | redir-host 返回真实 IP；fake-ip 返回 198.18.x.x |
| 12 | DNS | `dnsListen` | 设置 `0.0.0.0:1053`，从 LAN `dig @192.168.50.1 -p 1053` | 端口可查询；留空时不监听 |
| 13 | DNS | `dnsNameserver` | 设为可控 DNS（如 223.5.5.5），查配置；查询结果 | 配置值一致且解析成功 |
| 14 | DNS | `dnsFallback` | 查配置 `fallback` 数组 | 值与输入一致 |
| 15 | DNS | `dnsFakeIpRange` | 设自定义网段，fake-ip 模式查询 | 返回自定义网段内地址 |
| 16 | 订阅拉取 | `subUA` | 本机起 HTTP 服务器记录 UA；用临时订阅分别请求 | 每种预设/自定义 UA 与设置一致 |
| 17 | 订阅拉取 | `subProxyMode` | 临时订阅指向 IP 回显；direct 与 proxy 各刷新一次，读原始响应 | direct=家庭 WAN IP；proxy=代理出口 IP |
| 18 | 订阅拉取 | `subInsecure` | 本机起自签名 HTTPS；分别开/关 | 开启成功；关闭失败（若关闭仍成功=缺陷） |
| 19 | 订阅拉取 | `subInterval` | 设 1 小时；修订单条 `updatedAt` 后重启面板触发调度；看日志/更新时间 | 到期自动更新，未到期不更新；0=关闭 |
| 20 | 国内 IP 规则 | `ruleSourcePreset` | 三个预设各更新一次；读 `rules/meta.json` 与规则条数 | 来源/条数变化，下载成功 |
| 21 | 国内 IP 规则 | `ruleSourceURL` | 本机 HTTP 提供小型 CIDR；保存 custom 后立即更新 | 使用自定义 URL，meta.source=自定义 |
| 22 | 国内 IP 规则 | `ruleAutoUpdate` | 关闭/开启；改 meta 时间戳后等调度检查 | 关闭时到期也不更新；开启时更新 |
| 23 | 国内 IP 规则 | `ruleIntervalH` | 设 1/24；按 meta.updatedAt 计算是否到期 | 到期判断符合设置 |
| 24 | 自动选择 | `autoIntervalSec` | 改值后查 `config.json` 自动选择组 `interval` | 配置值一致，内核接受 |
| 25 | 自动选择 | `autoToleranceMs` | 改值后查 `tolerance` | 配置值一致 |
| 26 | 定时重启 | `autoRestart` | 关闭/开启；配合下两项验证 | 行为与开关一致 |
| 27 | 定时重启 | `restartEveryHours` | 写入过期 `restart.json` + 重启面板，等调度器 | 到期后内核 PID 变化，日志记录 |
| 28 | 定时重启 | `restartAt` | 设为下一分钟，等调度器 | 到点内核 PID 变化，`lastDailyDate` 写入 |
| 29 | 启动与访问 | `autoStart` | 控制 `engineWanted` 标记后重启面板 | 开=引擎自动起；关=不起 |
| 30 | 启动与访问 | `panelPassword` | 设密码后不带 cookie 请求、错误密码、正确密码；再清空 | 未登录 401；错误 401；正确登录后可访问 |
| 31 | 内核 | 检查更新/上传/回滚 | 检查版本；上传当前同版本二进制；回滚 | 校验通过、引擎重启、版本/哈希正确 |
| 32 | 备份与恢复 | 导出/恢复 | 导出 JSON 检查字段；用自身备份恢复 | 恢复后设置/订阅/节点一致，引擎重载 |
| 33 | 存储 | 占用/清理 | 读取报告；制造临时无主文件后清理 | 报告字段正确；清理释放且不影响订阅 |
| 34 | 外观 | 主题/主色/纯黑/低端模式 | Playwright 操作并读 DOM/localStorage | 立即生效、刷新后保持 |
| 35 | 诊断 | 服务/网络/IP 质量/复制 | 调接口并与 SSH/独立探测对照 | 数据真实、无假绿 |
| 36 | 日志级别 | `logLevel` | 五档切换，查内核配置、SSE 日志过滤、持久化 | 配置与过滤一致 |

## 2. 执行顺序

1. 基线抓取与只读巡检（状态、服务、诊断、存储、内核信息）。
2. 低风险配置项：mode、rulePreset、端口、allowLan、blockQuic、sniffer、routerSelf、lanIf。
3. DNS 全组。
4. 订阅拉取全组（本机 HTTP/HTTPS 测试服务）。
5. 规则源全组。
6. 自动选择、日志级别。
7. 定时重启、autoStart（会重启内核/面板）。
8. 面板密码、外观。
9. 内核、备份、存储。
10. 恢复基线并复核。

## 3. 证据与报告

- 原始证据：`/tmp/mm_rd/`（设置快照、配置、iptables、HTTP 抓包、日志片段）。
- 最终报告：`docs/real-device-settings-test-report.md`，逐项结论：通过 / 缺陷（含复现与修复建议）。
- 发现缺陷后先对照 FlClash 实现，记录差异与修复方案。
