# 开发与构建

## 环境

- Go 1.23（`go.mod` 与 CI 均使用 1.23）
- `sh`、`curl`、`tar`、`gzip`
- `python3`（前端静态检查）
- 不需要 Node.js：前端是原生 HTML/CSS/JS，通过 `go:embed` 打进面板

## 构建

```sh
./build.sh                        # armv7 koolshare 安装包
./build.sh --no-download          # 只用 .cache 里的内核与规则
./build.sh --dev                  # 额外编译本机可运行的 panel，便于调试
./build.sh --kernel /path/mihomo  # 使用指定的内核文件
PANEL_UPX=1 ./build.sh            # 面板二进制用 UPX 压缩
NOKERNEL=1 ./build.sh             # 不打包内核
./tools/build-merlin-package.sh   # Merlin 原生安装包
```

产物在 `dist/`，同时生成 `.sha256` 校验文件。`build.sh` 会先运行 `python3 tests/ui_check.py`，检查不通过时停止打包。

## 测试

```sh
go vet ./...
go test ./...
python3 tests/ui_check.py
sh tests/merlin_install_smoke.sh
sh tests/install_smoke.sh         # 需要先构建 dist/stage
sh tests/e2e.sh                   # 需要本机可运行的 mihomo 内核
```

`.github/workflows/ci.yml` 执行 `go vet`、armv7 交叉编译、前端静态检查、Merlin 安装冒烟与 `go test`。`tests/e2e.sh` 依赖脚本内设定的本机内核路径，按需调整。

## 项目结构

```
cmd/mlclash-panel/   Go 面板与内嵌前端（web/）
plugin/              两种固件的安装/卸载脚本、控制脚本、版本号
tools/               Merlin 打包脚本
tests/               单元、冒烟与端到端测试
docs/                安装、FAQ、API、内核与开发文档
```

## 版本号

同一个版本号需要同步四处：

- `cmd/mlclash-panel/main.go` 的 `panelVersion`
- `plugin/version`
- `plugin/webs/Module_mlclash.asp` 的 `PKG_VERSION`
- `cmd/mlclash-panel/web/index.html` 的资源版本号（用于打破浏览器缓存）

## 内核

默认使用 [fanghengxuan/mihomo](https://github.com/fanghengxuan/mihomo) 的 `slim-*` Release（默认 `MIHOMO_VERSION=v1.19.33`）。内核体积、构建标签与能力边界见 [kernel.md](kernel.md)。
