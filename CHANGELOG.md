# 更新记录

## 3.1.1（2026-11-03）

- **修复 IPv6 流量完全绕过透明代理**：此前只给 IPv4 加了 REDIRECT，手机走 IPv6 时
  广告拦截和代理都不生效
- 新增 `ip6tables` nat 链 `MLCLASH6` / `MLCLASH6_OUT`：绕过 `::1`、`fc00::/7`、
  `fe80::/10`、`ff00::/8` 和本机 LAN /64，其余 TCP REDIRECT 到 redir 端口
- IPv6 QUIC（UDP/443）REJECT，迫使回落 TCP；IPv6 TCP MSS 钳制
- 引擎启停、`ctl.sh` 与 koolshare/Merlin 卸载脚本同步清理 IPv6 规则
- 真机验证：IPv6 下 `gdt.qq.com` / `doubleclick.net` / `pangle.cn` REJECT，
  baidu / example / Google 正常；IPv4 行为不变

## 3.1.0（2026-11-03）

- **去广告合并为单一开关**：优先使用 anti-AD `mihomo.mrs`（约 10 万条域名，约 0.85MB），
  并保留内置 `category-ads-all` 兜底/补漏；关闭时两条规则一起移除
- anti-AD 列表跟随 `ruleAutoUpdate` / `ruleIntervalH` 自动更新（默认 24h），
  也支持规则页「立即更新」；官方源失败自动回退 raw / jsDelivr
- 规则页显示 anti-AD 列表状态（大小、更新时间）；默认关闭，避免 10 万条列表误杀
- 国内直连新增 `GEOSITE,apple-cn,DIRECT` / `GEOSITE,google-cn,DIRECT`（零额外下载）
- 修复设置页异步加载竞态：设置后到齐时控件可能仍显示旧值，保存时会把旧值写回
- 真机验证：anti-AD 852.5KB，`gdt.qq.com`/`pangle.cn`/`doubleclick.net` REJECT，
  baidu/bilibili/Google 正常；引擎内存约 62MB，jffs 可用 23.7MB

## 3.0.0（2026-11-02）

首个公开版本。

- 原生安装：Asuswrt-Merlin/gnuton（`/jffs/mlclash`）与 koolshare/SWRT（`/koolshare/mlclash`）
- mihomo slim armv7 内核 + Go 面板（内嵌 Web，无需 Node.js 构建）
- 订阅管理、节点分组/测速、rule / global / direct 模式
- IPv4 透明代理（iptables REDIRECT）、QUIC 阻断、国内直连（GEOSITE cn、可选 GEOIP、cn-ip CIDR）
- 可选去广告、可选 DNS（redir-host / fake-ip）
- 备份恢复、存储清理、内核上传/在线更新/回滚
- 开机自启、定时重启、面板密码、日志级别
