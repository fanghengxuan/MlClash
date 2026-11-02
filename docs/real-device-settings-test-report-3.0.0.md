# MlClash 3.0.0 设置项真机深度测试报告

- 日期：2026-11-02
- 设备：TUF-AX3000（Asuswrt-Merlin / gnuton 388.11，armv7，内核 4.1.52）
- 面板：MlClash **3.0.0**（提交 `ea409db`），`http://192.168.50.1:9380`
- 方法：每个设置项按「请求成功 → 持久化 → 生成配置 → 运行时/网络行为」四层验证；
  每项测试后恢复基线。需要等待的定时项单独拉起，缩短到最短合法周期。
- 对照：2.0.5 设置项报告（8 类缺陷已修复）与 FlClash 行为。

## 1. 结论摘要

| 类别 | 结果 |
|---|---|
| 设置项 / 动作 | **通过 120+ 项**；未发现 3.0.0 新增功能缺陷 |
| 2.0.5 历史缺陷回归 | 8 类全部通过（global、DNS fallback、lanIf、ctl restart、routerSelf、subInsecure、密码、lanIf API） |
| 3.0.0 改名相关 | 页面/接口/路径/iptables 链/备份兼容全部通过 |
| UI 12 个子页 | 渲染 12/12、取值 8/8、保存路径 2/2、外观 8/8，共 **32/32** |
| 定时器 | 每日定时重启、restartEveryHours、logLevel SSE、sniffer 通过；订阅/规则调度见 §5 |
| 缺陷 | 产品缺陷 **0**；测试脚本自身问题 4 处（已修正，不影响产品结论） |

## 2. 基线

- 设置：rule / cn-direct / 3333 / 7890 / allowLan=true / blockQuic=true / sniffer=true /
  routerSelf=false / lanIf=br0 / subInterval=6 / subUA=default / subInsecure=true /
  subProxyMode=auto / ruleSource=loyalsoldier / ruleAutoUpdate=true / ruleIntervalH=24 /
  geoipEnable=true / adBlock=true / autoRestart=true / restartAt=05:00 /
  autoIntervalSec=300 / autoToleranceMs=500 / dnsEnable=false / logLevel=error /
  autoStart=true / 面板无密码。
- 订阅：2 个，75 个节点；规则：Loyalsoldier 10012 条；GEOIP：country.mmdb 已下载。
- 备份：导出到 `/tmp/mlclash_test_backup.json`（37KB，2 订阅 / 2 provider），测试结束删除。
- 基线 `settings.json`、`subs.json`、`config.json`、`rules/meta.json`、`restart.json` 均已留存。

## 3. 逐项结果

### 3.1 代理与端口

| 设置项 | 结果 | 证据 |
|---|---|---|
| `mode=rule/global/direct` | 通过 | direct 出口 124.90.60.83；global `GLOBAL.now=🚀 节点选择`，出口 54.251.129.162；rule 恢复 |
| `rulePreset=cn-direct/proxy-all` | 通过 | proxy-all 规则 `GEOSITE,category-ads-all,REJECT` + `MATCH,🚀 节点选择`，Baidu 200、ipify 出口 42.200.173.7；cn-direct 恢复 GEOSITE,cn/GEOIP/CN/RULE-SET |
| `redirPort 3333→3334` | 通过 | config `redir-port=3334`、iptables `--to-ports 3334`、LAN 透明代理 Google 200 |
| `mixedPort 7890→7891` | 通过 | 7891 HTTP 代理 200；旧 7890 拒绝（000） |
| `allowLan=false/true` | 通过 | false 时 LAN 7890 拒绝（000），true 恢复 200 |
| `blockQuic=false/true` | 通过 | `FORWARD -p udp --dport 443 -j REJECT` 随开关增删 |
| `sniffer=false/true` | 通过 | 开：日志 `--> www.google.com:443`；关：日志 `--> 104.16.132.229:443` |
| `routerSelf=false/true` | 通过 | 开：`OUTPUT -j MLCLASH_OUT`，路由器 ipify 出口 209.9.201.135（代理）；关：无 OUTPUT 跳转，路由器 domestic 出口 124.90.60.83 |
| `lanIf=br0→br1→br0` | 通过 | br1 时 PREROUTING 仅 `-i br1 -j MLCLASH`，LAN 透明代理 000；br0 恢复且 Google 200 |
| `logLevel` | 通过 | 配置 `log-level` 同步；SSE error 模式 info 行 0 条，info 模式 3 条 |
| `adBlock=true/false/true` | 通过 | 关闭后首条规则变为 GEOSITE,cn；开启后首条为 `GEOSITE,category-ads-all,REJECT`，`ad-srv.net` 被 REJECT |
| `geoipEnable=true/false/true` | 通过 | 关闭时无 `GEOIP,CN`；开启恢复 |

### 3.2 DNS

| 设置项 | 结果 | 证据 |
|---|---|---|
| `dnsEnable=true` + `dnsFallback=1.1.1.1` | 通过 | PUT 200、引擎存活（2.0.5 D2 回归通过）、`fallback-filter.geoip=false` |
| `dnsMode=redir-host` | 通过 | `dig @192.168.50.1 -p 1053 example.com` → 104.20.23.154 |
| `dnsMode=fake-ip` | 通过 | 返回 198.19.0.4 |
| `dnsListen=0.0.0.0:1053` | 通过 | 端口可查询；留空后查询失败 |
| `dnsNameserver` / `dnsFallback` | 通过 | 配置数组与输入一致 |
| `dnsFakeIpRange=198.19.0.1/16` | 通过 | fake-ip 返回该网段 |
| 非法 fake-ip 网段 | 通过 | PUT 500，但引擎继续运行、旧配置保留 |

### 3.3 订阅拉取

| 设置项 | 结果 | 证据 |
|---|---|---|
| `subUA` default/curl/v2rayn/v2rayng/shadowrocket/custom | 通过 | 本机 HTTP 服务器记录：`clash.meta/1.19.33`、`curl/8.5.0`、`v2rayN/6.45`、`v2rayNG/1.9.16`、`Shadowrocket/2.2.9`、`TestUA/9.9` |
| `subProxyMode=direct` | 通过 | httpbin origin = 124.90.60.83（WAN） |
| `subProxyMode=proxy` | 通过 | httpbin origin = 13.215.177.122（代理出口） |
| `subProxyMode=auto` | 通过 | 直连成功，origin = 124.90.60.83 |
| `subInsecure=true` | 通过 | 本机自签名 HTTPS 订阅 status=ok |
| `subInsecure=false` | 通过 | 证书校验失败，status=error（不再自动降级） |
| `subInterval=1/0/6` | 通过 | 设置持久化；定时刷新见 §5 |
| 临时订阅清理 | 通过 | 删除后真实订阅仍为 2 个 |

### 3.4 国内规则

| 设置项 | 结果 | 证据 |
|---|---|---|
| `ruleSourcePreset=chnroutes2` | 通过 | 3887 条，来源 jsDelivr chnroutes2 |
| `ruleSourcePreset=17mon` | 通过 | 7456 条 |
| `ruleSourcePreset=loyalsoldier` | 通过 | 10012 条 |
| `ruleSourceURL` 自定义 | 通过 | 本机 600 条 CIDR 被采用，meta.count=600 |
| `ruleAutoUpdate` / `ruleIntervalH` | 通过 | 设置持久化；调度见 §5 |
| 规则恢复 | 通过 | 恢复 loyalsoldier 10012 条 |

### 3.5 自动选择 / 定时重启 / 启动与访问

| 设置项 | 结果 | 证据 |
|---|---|---|
| `autoIntervalSec=301` | 通过 | config url-test `interval=301` |
| `autoToleranceMs=123` | 通过 | config `tolerance=123`；UI 保存 501/500 往返也通过 |
| `autoRestart=false` + 到点 | 通过 | 70s 内 PID 不变（6122→6122） |
| `autoRestart=true` + 到点 | 通过 | 重置 2.x 继承的 `lastDailyDate` 后：PID 21564→21843，日志「每天 13:37 定时重启」「定时重启完成」，restart.json 更新 |
| `restartEveryHours=1` | 通过 | `nextRestartAt = lastRestartAt + 3600` |
| `autoStart=false/true` | 通过 | false：ctl restart 后引擎 stopped；true：启动引擎后 ctl restart 自动恢复 |
| `panelPassword` | 通过 | 未登录/错误密码 401；旧 cookie 改密后 401；`panelPasswordSet=true` 且无明文；清除后 200 |
| `rulePreset` / `mode` / `lanIf` 回归 | 通过 | 见 §3.1 |

### 3.6 内核 / 备份 / 存储 / 诊断

| 动作 | 结果 | 证据 |
|---|---|---|
| 内核检查更新 | 通过 | 最新 slim-v1.19.33，`hasUpdate=false` |
| 内核上传（同版本） | 通过 | 校验通过、引擎重启、版本/SHA 不变 |
| 内核回滚 | 通过 | 回滚成功、引擎运行、SHA `d9299808…` |
| 备份恢复 | 通过 | 改 autoTolerance=123 后恢复，值回到 500，2 订阅保留 |
| 旧版备份兼容 | 通过 | `kind=mihomo-merlin-backup` + format1 恢复 200 |
| 存储报告 | 通过 | `report.dataDir=/jffs/mlclash/data`，items/total/free/logBytes/cacheBytes 正常 |
| 存储清理 | 通过 | 制造 orphan provider/cache/.tmp，清理后全部删除，订阅仍为 2 |
| 诊断 | 通过 | service/intranet/ipquality/netinfo/kernel/connections 均正常 |

### 3.7 边界校验

19 个非法值 + `redirPort == mixedPort` 全部返回 400，且设置不变；覆盖 mode、logLevel、
rulePreset、端口 0/65536、subInterval 169/-1、dnsMode、subUA、subProxyMode、
ruleIntervalH -1、autoIntervalSec 29/3601、autoToleranceMs -1/1001、
restartEveryHours 0/169、restartAt 25:00。引擎始终存活。

### 3.8 UI

- 12 个设置子页全部渲染，标题均为「… · MlClash」，无 console/page error。
- 关键值回显：redirPort/mixedPort/allowLan/adBlock/geoip/autoTolerance/autoRestart/autoStart 8/8。
- UI 保存路径：autoTolerance 500→501→500；adBlock true→false 保存后规则首条变为 GEOSITE,cn。
- 外观：浅色/深色/主色青瓷/纯黑/低端模式开/关/刷新持久化 8/8；测试后恢复系统/海湾蓝。

## 4. 2.0.5 历史缺陷回归

| 缺陷 | 3.0.0 结果 |
|---|---|
| D1 global 模式无效 | 通过：GLOBAL.now=🚀 节点选择，ipify 出口 54.251.129.162 |
| D2 DNS fallback 停引擎 | 通过：PUT 200，引擎运行，fallback-filter.geoip=false |
| D3 lanIf 不生效 | 通过：br1 仅 br1 跳转且 LAN 代理断开；br0 恢复 |
| D4 ctl.sh restart 断代理 | 通过：autoStart=true 时面板重启后引擎自动恢复 |
| D5 routerSelf 回环/bypass 解析 | 通过：OUTPUT 跳转 + 路由器 ipify 出口代理；关闭恢复直连 |
| D6 subInsecure 被架空 | 通过：false 时自签名证书失败 |
| D7 密码明文/旧会话 | 通过：无明文、改密旧 cookie 401 |
| D8 lanIf 未回传 | 通过：`/api/settings` 含 lanIf |

## 5. 定时器

- 每日固定时刻：重置 2.x 继承的 `lastDailyDate` 后，13:37 到点重启成功（PID 变化、
  restart.json 更新、日志两行 info）。
- `restartEveryHours=1`：`nextRestartAt` 精确等于 `lastRestartAt + 3600`。
- logLevel：error 模式 SSE 无 info 行，info 模式有 info 行。
- 订阅 / 规则 10 分钟维护周期：见下方「调度器实测」小节（通过）。

### 5.1 订阅 / 规则 10 分钟维护周期（实测）

- 面板 13:48:40 重启；第一个维护 tick 约 13:58:40。
- 真实订阅 机场A 13:59:58 自动更新（updatedAt 1793599198）；机场B 14:00:36 更新
  （updatedAt 1793599236）；临时订阅 SCHED 14:00:37 更新成功（status=ok，nodeCount=1）。
- 本机 HTTP 服务器在 14:00:36 收到 `GET /sub`，证明订阅自动更新真实发生。
- `ruleAutoUpdate=false` 时，同一 tick 后 `rules/meta.json` 的 `updatedAt` 仍为 0，
  没有被自动更新；恢复 `ruleAutoUpdate=true` 并手动更新后，meta 变为
  `updatedAt=1793599249, source=Loyalsoldier, count=10012`。
- 注意：维护循环串行处理订阅，两个真实订阅各耗数十秒，因此 10 分钟 tick 的
  完成时间会略晚于 tick 起点（本次约 2 分钟），不是调度失效。

## 6. 测试脚本自身问题（非产品缺陷）

1. core 的 proxy-all 用境内 IP 回显判断是否代理；该回显经代理返回空，改为
   「config MATCH + Baidu 200 + ipify 代理出口」判定后通过。
2. routerSelf 的 SSH 出口探测最初缺少 `SSHPASS` 环境，导致 shell 打印 usage；
   修正后用 `api.ipify.org` 得到 209.9.201.135，关闭时 domestic 得到 WAN。
3. storage 报告实际结构是 `{"ok":true,"report":{...}}`，脚本按顶层字段读取才报错；
   实际报告字段完整，清理与订阅保留均正常。
4. 定时重启脚本没有先清理 2.x 继承的 `lastDailyDate`（当天 05:00 已执行过），
   导致 13:32/13:34 的到点测试被正确跳过；重置该字段后 13:37 重启通过。
   这是产品正确行为，不是缺陷。

## 7. 最终状态与清理

- 恢复基线：rule / cn-direct / 3333 / 7890 / allowLan=true / blockQuic=true / sniffer=true /
  routerSelf=false / lanIf=br0 / subInterval=6 / subUA=default / subInsecure=true /
  subProxyMode=auto / ruleSource=loyalsoldier / ruleAutoUpdate=true / ruleIntervalH=24 /
  geoipEnable=true / adBlock=true / autoRestart=true / restartAt=05:00 /
  autoIntervalSec=300 / autoToleranceMs=500 / dnsEnable=false / logLevel=error /
  autoStart=true / 面板无密码。
- 引擎运行、iptables 已应用；真实订阅 2 个（42 + 33 = 75 节点），临时订阅/测试规则/
  本地 HTTP/HTTPS 服务已删除；存储清理 `freed=0`（确认无测试残留）。
- 规则恢复 Loyalsoldier 10012 条；GEOIP country.mmdb 保留；adblock 首条规则恢复。
- 路由器 `/tmp` 仅保留 `/tmp/mlclash` 运行时目录；`/tmp/mihomo` 已不存在。
- 本机测试脚本、备份、截图与调度器 fixture 清理。

## 8. 未覆盖 / 限制

- koolshare 3.0.0 仅做假根目录冒烟，未在真实 koolshare 固件上跑设置项。
- 内核「在线更新」只验证检查更新（当前已是最新 v1.19.33），未下载替换；
  本地上传同版本 + 回滚已实测。
- 订阅调度器实测一个 tick（真实订阅 + 临时订阅均更新）；`ruleAutoUpdate=false` 的
  跳过行为通过；`ruleAutoUpdate=true` 的自动 tick 依赖手动更新与上一版调度器测试。
- 定时重启测试为每日固定时刻；`restartEveryHours` 只验算了 nextRestartAt，未等待 1 小时。
- 未做真实断电/重启路由器（autoStart 用 `ctl.sh restart` 验证）。
- `dnsFallback` 验证了配置与引擎稳定性，未强制构造 fallback 触发场景；
  `allowLan=false` 只验证 mixed 端口，透明代理链路仍保留。
