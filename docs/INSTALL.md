# 安装与卸载

## 支持范围

| 项目 | 要求 |
|---|---|
| CPU | armv7（`GOARM=7`）；其他架构需自行交叉编译 |
| 固件 | Asuswrt-Merlin / gnuton，或带软件中心的 koolshare / SWRT |
| JFFS | 启用自定义脚本；至少 **18 MB** 可用空间 |
| 内存 | 建议 512 MB 及以上 |
| 访问方式 | Merlin 原生安装需要 SSH；koolshare 可在软件中心操作 |

已验证设备为 **TUF-AX3000**（axhnd.675x / armv7 / 内核 4.1），已验证固件为 **gnuton Asuswrt-Merlin**。其他华硕 armv7 机型未逐台验证。

## Asuswrt-Merlin / gnuton

1. 从 [Releases](https://github.com/fanghengxuan/MlClash/releases) 下载 `MlClash_<版本>_armv7_merlin.tar.gz`。
2. 开启 **系统管理 → 系统设置 → Enable JFFS custom scripts and configs**。
3. 上传到路由器并安装：

```sh
cd /tmp
tar -xzf MlClash_<版本>_armv7_merlin.tar.gz
sh /tmp/MlClash/install.sh
```

4. 浏览器打开 `http://<路由器IP>:9380`。

安装位置为 `/jffs/mlclash`，控制脚本为 `/jffs/mlclash/ctl.sh`：

```sh
sh /jffs/mlclash/ctl.sh status
sh /jffs/mlclash/ctl.sh restart
```

## koolshare / SWRT

下载 `MlClash_<版本>_armv7_koolshare.tar.gz`，在软件中心「离线安装」中选择该文件。

手动安装：

```sh
cd /tmp
tar -xzf MlClash_<版本>_armv7_koolshare.tar.gz
sh /tmp/mlclash/install.sh
```

安装位置为 `/koolshare/mlclash`，控制脚本为 `/koolshare/scripts/mlclash_ctl.sh`：

```sh
sh /koolshare/scripts/mlclash_ctl.sh status
sh /koolshare/scripts/mlclash_ctl.sh restart
```

## 升级

直接在现有安装上覆盖安装新版本。安装器会停止服务、更新面板与内核，保留数据、订阅和规则，然后重新启动面板。

- Merlin：重新执行新包里的 `install.sh`。
- koolshare：在软件中心重新离线安装新版本。

## 卸载

Merlin：

```sh
sh /jffs/mlclash/uninstall.sh              # 删除程序和全部数据
sh /jffs/mlclash/uninstall.sh --keep-data  # 只删程序，保留数据
```

koolshare：使用软件中心的卸载按钮，或执行 `/koolshare/scripts/uninstall_mlclash.sh`。koolshare 卸载会删除程序和全部数据；需要保留时，先在面板「设置 → 备份与恢复」导出 JSON 备份。

## 首次使用

1. 打开面板，在 **订阅** 页添加订阅。
2. 在 **节点** 页选择分组或节点；订阅更新完成后会自动检测节点质量。
3. 在 **概览** 页启动引擎。
4. 可选：在 **设置 → 国内 IP 规则** 开启「拦截广告与统计域名」。
5. 可选：在 **设置 → 启动与访问** 设置面板密码。

## 端口与数据目录

| 项目 | 默认值 |
|---|---|
| 面板 | `0.0.0.0:9380` |
| HTTP/SOCKS 混合端口 | `7890` |
| 透明代理 redir 端口 | `3333` |
| mihomo API | `127.0.0.1:9090` |
| Merlin 安装 / 数据 | `/jffs/mlclash`、`/jffs/mlclash/data` |
| Merlin 日志 | `/jffs/mlclash/data/logs`（`panel.log`、`mihomo.log`；实际软链到 `/tmp/mlclash/logs`） |
| koolshare 安装 / 数据 | `/koolshare/mlclash` |
| koolshare 日志 | `/koolshare/mlclash/logs` |

## 安全提醒

面板与 mihomo API 面向局域网使用，不要把 `9380` 或 `9090` 暴露到 WAN。建议设置面板密码，并只在需要时开启「允许局域网访问代理端口」。
