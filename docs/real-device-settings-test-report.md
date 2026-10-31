# mihomo-merlin 设置项真机测试报告

- 日期：2026-11-01
- 设备：TUF-AX3000（Asuswrt-Merlin / gnuton 388.11，armv7）
- 面板：v2.0.4（提交 `1285672`），`http://192.168.50.1:9380`
- 方法：每个设置项单独修改，验证「请求成功 → 持久化 → 生成配置 → 运行时/网络行为」四层；需要等待的项缩短到最短合法周期。原始证据在 `/tmp/mm_rd/`。
- 对照：FlClash（`chen08209/FlClash`，本地稀疏克隆 `/tmp/flclash-git`）。

## 1. 结论摘要

| 类别 | 结果 |
|---|---|
| 设置项/动作逐项测试 | 通过 50+ 项，发现缺陷 8 类 + 1 项观察 |
| 严重缺陷 | 4（global 模式无效、DNS fallback 会停引擎、lanIf 不生效、ctl.sh restart 断代理） |
| 高危缺陷 | 1（代理服务器 bypass 解析失败，routerSelf 会回环） |
| 中危缺陷 | 2（改密旧会话不失效 + 密码明文回传、subInsecure 开关被自动降级架空） |
| 轻微缺陷 | 1（`lanIf` 未出现在 `/api/settings`，UI 输入框为空） |
| 修复与复验 | 8 类缺陷已全部修复；`go test -race` 全绿，真机复验 37/37、UI 26/26、调度器 6/6（§5） |

## 2. 逐项结果

### 2.1 代理与端口

| 设置项 | 结果 | 证据 |
|---|---|---|
| `mode=rule/global/direct` | **global 失效** | rule/direct 的 `modeActual` 与连接链正确；global 下 GLOBAL 选择器为 `DIRECT`，出口仍直连 |
| `rulePreset=cn-direct/proxy-all` | 通过 | cn-direct 生成 `RULE-SET,cn-ip,DIRECT`，国内站点连接链 `DIRECT`；proxy-all 下国内站点连接链为「节点选择→自动选择→机场节点」 |
| `redirPort` 3333→3334 | 通过 | config、iptables `--to-ports 3334`、监听端口、LAN 透明代理全部切换 |
| `mixedPort` 7890→7891 | 通过 | 新端口 HTTP 代理可用，旧端口拒绝；热重载无需重启 |
| `allowLan` false/true | 通过 | false 时仅监听 127.0.0.1，LAN 代理端口拒绝；true 恢复 |
| `blockQuic` false/true | 通过 | `FORWARD -p udp --dport 443 -j REJECT` 随开关增删 |
| `sniffer` false/true | 通过 | 开：连接 Host=`speed.cloudflare.com`；关：Host=`172.66.0.218:443` |
| `routerSelf` | **待修复后复验** | 基线 iptables 中没有任何代理服务器 IP 的 RETURN，开启会回环 |
| `lanIf` | **失效** | 切到 `br1` 后旧 `-A PREROUTING -i br0 -j MIHOMO` 仍在，LAN 流量继续走代理 |

### 2.2 DNS

| 设置项 | 结果 | 证据 |
|---|---|---|
| `dnsEnable` | 通过（fallback 留空时） | 配置出现/移除 `dns` 段，内核重载成功 |
| `dnsMode=redir-host/fake-ip` | 通过 | redir-host 返回真实 IP；fake-ip 返回 `198.19.0.4` |
| `dnsListen` | 通过 | `0.0.0.0:1053` 监听并可查询；留空后端口关闭 |
| `dnsNameserver` | 通过 | 单值/多值写入 `nameserver` 数组 |
| `dnsFallback` | **严重缺陷** | 填 `1.1.1.1,8.8.8.8` 时 mihomo 尝试下载 GeoIP MMDB 失败，配置校验失败，内核被停止 |
| `dnsFakeIpRange` | 通过 | 自定义 `198.19.0.1/16` 返回该网段 fake-ip |

### 2.3 订阅拉取

| 设置项 | 结果 | 证据 |
|---|---|---|
| `subUA` 六种 | 通过 | 本机 HTTP 服务器实测 UA：`clash.meta/1.19.33` / `curl/8.5.0` / `v2rayN/6.45` / `v2rayNG/1.9.16` / `Shadowrocket/2.2.9` / 自定义原样 |
| `subProxyMode=direct/proxy/auto` | 通过 | direct 得 `101.68.142.6`；proxy 得 `54.251.129.162`；auto 在直连失败后改走代理并保存原始响应 |
| `subInsecure` | **开关被架空** | 关闭时对自签名证书仍自动降级重试并成功；与 FlClash 默认校验、不降级的行为不一致 |
| `subInterval` | 见 §4 定时器 | 10 分钟维护周期 |

### 2.4 国内 IP 规则

| 设置项 | 结果 | 证据 |
|---|---|---|
| `ruleSourcePreset` 三预设 | 通过 | chnroutes2 3887 条、17mon 7456 条、Loyalsoldier 10012 条，来源/条数/配置均更新 |
| `ruleSourceURL` 自定义 | 通过 | 本机 600 条 CIDR 源被采用，meta.count=600，`chnroute.yaml` 600 条，RULE-SET 生效 |
| `ruleAutoUpdate` / `ruleIntervalH` | 见 §4 定时器 | 10 分钟规则调度周期 |

### 2.5 自动选择 / 日志 / 外观

| 设置项 | 结果 | 证据 |
|---|---|---|
| `autoIntervalSec` / `autoToleranceMs` | 通过 | 配置 url-test 组 `interval/tolerance` 精确一致，非法值 400 |
| `logLevel` 五档 | 通过 | 内核 `log-level` 同步；SSE 实际过滤：info/debug 可见 info，warning/error/silent 不可见 |
| 外观（主题/主色/纯黑/低端） | 通过 | Playwright 实测 DOM 属性、CSS 变量、localStorage 与刷新持久化，10/10 |

### 2.6 运行与安全

| 设置项 | 结果 | 证据 |
|---|---|---|
| `autoStart` off/on | 通过 | 用内容为 `1` 的 enabled 标记复验：off 不启动、on 自动启动 |
| `panelPassword` | **部分缺陷** | 未登录/错误密码 401、正确登录 200；但改密后旧 cookie 仍有效，且 API 明文返回密码 |
| `autoRestart` | **部分缺陷** | 周期重启正常；关闭总开关后「每天固定时刻」仍会重启 |
| `restartEveryHours` | 通过 | 过期状态 + 面板重启后 1 小时周期到点重启，PID 与 restart.json 均更新 |
| `restartAt` | 通过（开启时） | 下一分钟到点重启，日志「定时重启完成」，`lastDailyDate` 正确 |
| 内核检查/上传/回滚 | 通过 | 检查返回最新 `v1.19.33`（无更新）；上传同版本后 SHA 不变、引擎重启、生成备份；回滚成功 |
| 备份/恢复 | 通过 | 导出含 settings/2 订阅/2 provider/rulesMeta；改设置后恢复即回到基线；外来 JSON 400 |
| 存储清理 | 通过 | 制造 1MB+256KB+64KB 孤儿文件，清理释放 1,376,256 字节，真实订阅完好 |
| 诊断 | 通过 | service/intranet/ipquality 与 SSH、出口 IP 一致；内核 SHA256/大小与路由器文件一致 |

### 2.7 边界校验

`mode`、`logLevel`、`rulePreset`、`redirPort`(0/65536)、`mixedPort`(0)、`subInterval`(169/-1)、`dnsMode`、`subUA`、`subProxyMode`、redir=mixed 同端口，均返回 400 且不改变现有设置（13/13）。

## 3. 缺陷清单

### D1（严重）全局模式实际仍直连
- 现象：`mode=global` 后 `/api/status` 的 `modeActual=global`，但 mihomo `GLOBAL` 选择器 `now=DIRECT`，出口 IP 仍为直连。
- 原因：配置里只改 `mode`，从未设置 mihomo 内置 `GLOBAL` 选择器；它默认是 DIRECT。面板也没有 GLOBAL 组入口。
- FlClash：`changeMode(Mode.global)` 会把当前组切到 `GroupName.GLOBAL`（`lib/providers/actions/setup.dart:373`），用户可明确选择全局出口。
- 修复：内核启动/热重载后，若 `mode=global`，把 `GLOBAL` 选到主组 `🚀 节点选择`（`Engine.ensureGlobalSelector`）。

### D2（严重）配置 DNS fallback 会导致内核被停掉
- 现象：设置 `dnsEnable=true` 且 `dnsFallback` 非空时，PUT 返回 500，内核 stopped。
- 原因：`fallback` 存在但未写 `fallback-filter`，mihomo 默认 `fallback-filter.geoip=true`，启动校验时下载 GeoIP MMDB；路由器直连 GitHub 超时 → 校验失败。旧 `Reload` 失败后转 `Restart`，`Restart` 校验再失败，内核直接停掉。
- 修复：① `buildDNSSection` 在配置 fallback 时写入 `fallback-filter:{geoip:false,ipcidr:[240.0.0.0/4]}`；② `Reload` 先 `mihomo -t`，失败则还原旧配置并返回错误，不再停止运行中的内核。

### D3（严重）修改 lanIf 不生效
- 现象：`lanIf=br1` 后 iptables 同时存在 `-i br0` 与 `-i br1` 的 MIHOMO 跳转，LAN 流量仍被 br0 规则代理。
- 原因：`ApplyIptables` 只追加当前接口，不清理旧接口；`ClearIptables` 也只删当前接口。
- 修复：按 `iptables -t nat -S PREROUTING` 实际输出清理所有非当前接口的 MIHOMO 跳转；清理时删除所有接口跳转。

### D4（严重）`ctl.sh restart` 后引擎不再自动启动
- 现象：按 README 执行 `ctl.sh restart`，面板起来但 `core: stopped`、`enabled: no`。
- 原因：面板收到 SIGTERM 调用 `engine.Stop()`，`stopLocked(false)` 删除了 enabled 标记；新面板启动时 `autoStart && engineWanted` 不成立。
- 修复：新增 `Engine.Shutdown()`（`stopLocked(true)`），退出信号保留运行意图。

### D5（高危）代理服务器 bypass 解析不到，routerSelf 会回环
- 现象：基线 `MIHOMO` 链只有私网 RETURN，没有任何代理服务器 IP；`proxyServerIPs()` 返回空。
- 原因：订阅 provider 是压缩单行 YAML（`- { name:..., server: 1.2.3.4, ... }`），旧正则只匹配行首或 JSON `"server":`。
- 修复：正则支持行内 `server:`；只收 IPv4；上限从 64 提到 256。

### D6（中危）关闭「跳过证书校验」仍会放行自签名
- 现象：`subInsecure=false` 时自签名 HTTPS 订阅仍成功。
- 原因：`doFetch` 在证书错误时自动把 `insecure` 置 true 重试。
- FlClash：`checkCertificate` 默认 true；`lib/common/http.dart:77-91` 的 `badCertificateCallback` 返回 `!checkCertificate`，即严格按设置执行，不会自动降级。
- 修复：移除自动降级，证书错误按设置如实失败。

### D7（中危）面板密码相关问题
- 现象：修改密码后旧 cookie 仍有效；`GET /api/settings` 明文返回 `panelPassword`。
- 修复：改密时清空 session；API 改为只返回 `panelPasswordSet`；前端改为「设置新密码（留空不改）」+ 显式「清除密码」。

### D8（轻微）`lanIf` 不在 `/api/settings` 返回
- 现象：前端「局域网接口」输入框永远为空（只有 placeholder br0）。
- 修复：`publicSettings` 加入 `lanIf`。

### D9（观察）`iptables.blockQuic` 状态字段只是回显设置值
- 不是功能性错误，但 `/api/status` 的 `iptables.blockQuic` 表达式等价于 `s.BlockQuic`，不反映规则是否真的存在。建议后续按实测规则返回。

## 4. 定时器测试（10 分钟周期）

- 全局 `subInterval=1` + 临时订阅 `updatedAt=0`：第一个维护 tick（面板启动后 10 分钟）到达后，临时订阅 `updatedAt` 从 0 变为当前时间，本机测试服务器收到 1 次新请求；真实订阅（临时 pin 到 24h）的 `updatedAt` 完全未变。**通过**。
- `ruleAutoUpdate=false` + meta 时间戳回拨 2 小时：同一 tick 内规则 meta `updatedAt` 保持回拨值不变。**通过**。
- `ruleAutoUpdate=true` 后下一个 tick（04:38:32）：规则从本地自定义源重新下载，meta.updatedAt 从回拨值更新为当前时间，来源保持本地 URL。**通过**。
- 结论：订阅全局间隔、规则自动更新开关与调度周期均在真机按预期工作。

## 5. 修复与真机复验

修复内容已随 **2.0.5** 发布：先用本地构建（`sha256=37bd49ec…`）覆盖安装到路由器完成复验，随后用正式的
`mihomo-merlin_2.0.5_armv7_merlin.tar.gz`（`sha256=77b8e1d8…`）在真机执行升级安装，确认版本显示 2.0.5、
设置/订阅/规则全部保留、引擎自动恢复。

### 5.1 修复清单

| 缺陷 | 修复文件 | 关键改动 |
|---|---|---|
| D1 global 模式无效 | `engine.go` | 启动/热重载后按 `mode=global` 把 GLOBAL 选择器切到 `🚀 节点选择`；最小配置无主组时跳过 |
| D2 DNS fallback 停内核 | `config.go`、`engine.go` | fallback 配 `fallback-filter.geoip=false`；Reload 先 `mihomo -t`，失败还原旧配置且不再 Restart |
| D3 lanIf 不生效 | `engine.go` | 按 `iptables -S PREROUTING` 实际结果清理所有非当前接口跳转；Clear 时清全部 |
| D4 ctl.sh restart 断代理 | `engine.go`、`main.go` | 新增 `Engine.Shutdown()`（保留 enabled 标记），退出信号使用它 |
| D5 服务器 bypass 解析失败 | `engine.go` | 正则支持压缩 YAML 行内 `server:`；仅 IPv4；上限 256 |
| D6 subInsecure 被架空 | `subs.go` | 删除证书错误自动降级，严格按开关执行（与 FlClash `badCertificateCallback` 一致） |
| D7 密码明文/旧会话 | `api.go`、`backup.go`、`web/js/views/settings.js` | API 只回 `panelPasswordSet`；改密/恢复备份清 session；前端「设置新密码 + 清除密码」 |
| D8 lanIf 不回传 | `api.go` | `publicSettings` 增加 `lanIf` |
| 错误信息重复前缀 | `engine.go` | Reload 直接返回 validateConfig 的错误，避免「配置校验失败：配置校验失败：」 |

配套：`conns_test.go` 更新调度语义测试，`iptables_apply_test.go` 新增跳转解析与 server 正则测试，`docs/API.md` 更新设置字段；`go test -race ./...` 全绿。

### 5.2 复验结果

| 复验脚本 | 结果 | 覆盖 |
|---|---|---|
| `/tmp/mm_rd/verify_fixes.py` | **37 / 37 通过** | publicSettings、global、DNS fallback、非法配置不停引擎、lanIf、autoRestart 总开关、ctl.sh restart、proxyServerIPs + routerSelf、subInsecure、面板密码 |
| `/tmp/mm_rd/ui_sweep.js`（Playwright） | **26 / 26 通过** | 12 个设置子页渲染/取值/控件状态、UI 保存写回 API、安全页新文案 |
| `/tmp/mm_rd/sched_test.py` | **6 / 6 通过** | 全局 subInterval、ruleAutoUpdate、调度 tick |

关键复验证据：
- global：`modeActual=global`、`GLOBAL.now=🚀 节点选择`、Mac 出口 `54.251.129.162`（代理）。
- DNS fallback：PUT 成功、引擎存活、`fallback-filter.geoip=false`、1053 端口解析正常。
- 非法配置（fake-ip 网段 `not-a-cidr`）：返回 500，但引擎继续运行、旧配置还原。
- lanIf：只有 `-i br1` 跳转时 LAN 不被代理；恢复后只有 `-i br0` 且 LAN 代理恢复。
- autoRestart=false + 固定时刻下分钟：PID 不变，`nextRestartAt=0`。
- `ctl.sh restart`：引擎自动恢复，`enabled` 标记保留。
- proxyServerIPs：`MIHOMO` 链出现 37 条代理服务器 `/32` RETURN；routerSelf 开：路由器自身走代理（ipify `209.9.201.135`、Google `204`），关：OUTPUT 跳转移除、恢复直连。
- 密码：改密后旧 cookie 401；`panelPasswordSet=true` 且无 `panelPassword` 字段。

### 5.3 最终状态

- 路由器面板版本 **2.0.5**（Merlin 正式安装包验证通过），引擎运行、`enabled` 标记保留。
- 设置恢复基线（rule / cn-direct / DNS 关闭 / br0 / routerSelf 关 / autoStart 开 / 定时重启 05:00）。
- 订阅：机场A 41 节点、机场B 33 节点，interval=0（跟随全局 6h）。
- 规则：chnroutes2 3888 条；内核 slim v1.19.33，无残留备份。
- 引擎运行、iptables 已应用、Mac 出口为代理。
- 测试用临时订阅、本地 HTTP/HTTPS 服务、内核备份均已清理。

## 6. 未覆盖 / 限制

- 内核「在线更新」只验证了检查更新（当前已是最新 `v1.19.33`，没有新版本可下载），未实际执行下载替换。
- 直连侧 `api.ipify.org` 在本网络被阻断，出口判断改用 `cip.cc` / `ip.3322.net` / 连接链。
- 定时类测试使用 10 分钟调度周期；没有修改路由器时间。
