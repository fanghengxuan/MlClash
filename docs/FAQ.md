# 常见问题

## 面板打不开

先确认面板进程和 `9380` 端口：

```sh
# Merlin
sh /jffs/mlclash/ctl.sh status

# koolshare
sh /koolshare/scripts/mlclash_ctl.sh status
```

也可以直接请求 `http://<路由器IP>:9380/api/status`。仍然无法访问时，查看 `panel.log`。

## 引擎未运行

确认内核状态并查看日志：

```sh
# Merlin
sh /jffs/mlclash/ctl.sh status
cat /jffs/mlclash/data/logs/panel.log
cat /jffs/mlclash/data/logs/mihomo.log

# koolshare
sh /koolshare/scripts/mlclash_ctl.sh status
cat /koolshare/mlclash/logs/panel.log
cat /koolshare/mlclash/logs/mihomo.log
```

常见原因是订阅无效、配置校验失败或端口被占用。

## 节点质量一直显示「未测」

质量检测在订阅更新完成后触发，并且需要内核正在运行。手动刷新一次订阅即可。检测在后台执行，不阻塞面板。

## 自动选择里的节点变少了

3.2.0 起，被公开库标记为代理 / VPN / Tor 的「有风险」节点会被排除出「自动选择」。它们仍保留在订阅子分组里，可以手动选择。如果所有节点都有风险，分组会保留全部节点，避免组为空。

## 仍有广告

依次确认：

1. 运行模式为 `rule`（去广告只在规则模式生效）。
2. 去广告开关已开启。
3. 规则页里的 anti-AD 状态为 `ready`。

重启应用可清空缓存。域名级过滤无法处理第一方或信息流广告、YouTube 广告，以及没有可用 SNI / HTTP Host 的请求。

## 空间不足

在面板执行存储清理，删除不需要的内核备份。构建时设置 `NOKERNEL=1` 可以生成不带内核的安装包，安装后在面板里下载或上传内核。
