#!/bin/sh
# 安装脚本冒烟测试：用假根目录 + 桩命令模拟 koolshare 路由器，
# 验证 install.sh / uninstall.sh 的 2.x 迁移、落盘位置、dbus 登记、开机自启软链、卸载清理。
#
# 用法：sh tests/install_smoke.sh
# 依赖：sh、sed、tar、mktemp；不需要 ARM 环境

set -eu

PROJ=$(cd "$(dirname "$0")/.." && pwd)
PKG_SRC="$PROJ/dist/stage/mlclash"
[ -d "$PKG_SRC" ] || { echo "先跑 ./build.sh 生成 dist/stage/mlclash"; exit 1; }

ROOT=$(mktemp -d /tmp/mlclash-smoke.XXXXXX)
STUB="$ROOT/stub"
KS="$ROOT/koolshare"
mkdir -p "$STUB" "$KS"/bin "$KS"/webs "$KS"/res "$KS"/scripts "$KS"/init.d
USE_BASH_ALIAS=""
if bash -c 'true' 2>/dev/null && bash -O expand_aliases -c 'true' 2>/dev/null; then
	USE_BASH_ALIAS=1
fi
run_sh() {
	# 桩命令放前面：卸载脚本不自己设 PATH，需要能解析到 iptables/pidof 等桩
	export PATH="$STUB:$KS/bin:$PATH"
	if [ -n "$USE_BASH_ALIAS" ]; then
		bash -O expand_aliases "$@"
	else
		sh "$@"
	fi
}

fail() { echo "❌ $*"; exit 1; }
ok() { echo "✅ $*"; }

# ---------- 桩命令 ----------
cat > "$STUB/uname" <<EOF
#!/bin/sh
case "\$1" in
	-m) echo armv7l ;;
	-r) echo 4.1.52 ;;
	*) echo Linux ;;
esac
EOF

cat > "$STUB/nvram" <<'EOF'
#!/bin/sh
case "$2" in
	productid) echo TUF-AX3000 ;;
	lan_ipaddr) echo 192.168.50.1 ;;
	lan_ifname) echo br0 ;;
	*) echo "" ;;
esac
EOF

cat > "$STUB/dbus" <<EOF
#!/bin/sh
echo "dbus \$*" >> "$ROOT/dbus.log"
case "\$2" in
	softcenter_module_mlclash_install) echo 4 ;;
esac
EOF

cat > "$STUB/iptables" <<EOF
#!/bin/sh
echo "iptables \$*" >> "$ROOT/iptables.log"
exit 0
EOF

# df：模拟 jffs 剩余空间（默认 17548KB，和只有 17MB 的机型一致）
cat > "$STUB/df" <<EOF
#!/bin/sh
echo "Filesystem 1024-blocks Used Available Capacity Mounted on"
echo "/dev/mtdblock 65536 0 \${MLCLASH_FAKE_FREE_KB:-17548} 1% /jffs"
EOF
chmod +x "$STUB/df"

# killall：macOS 没有该命令（BusyBox 有），给个桩避免卸载脚本报错
cat > "$STUB/killall" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$STUB/killall"

# pidof：从 pid 文件读，模拟「进程存在」
cat > "$STUB/pidof" <<EOF
#!/bin/sh
[ -f "$ROOT/panel.pid" ] || exit 1
pid=\$(cat "$ROOT/panel.pid" 2>/dev/null)
[ -n "\$pid" ] || exit 1
kill -0 "\$pid" 2>/dev/null || exit 1
echo "\$pid"
EOF

# 内核桩：-v 打印版本，-t 校验通过
cat > "$STUB/mihomo" <<'EOF'
#!/bin/sh
case "$1" in
	-v) echo "Mihomo Meta v1.19.33 linux arm (stub)"; exit 0 ;;
	-t) exit 0 ;;
	*) exit 0 ;;
esac
EOF

# 面板桩：-v 立刻返回；正常启动时写 pid 后挂起（模拟常驻进程）
cat > "$STUB/mlclash-panel" <<EOF
#!/bin/sh
case "\$1" in
	-v) echo "MlClash panel 3.1.1-stub"; exit 0 ;;
esac
echo \$\$ > "$ROOT/panel.pid"
echo "MlClash panel 3.1.1-stub started"
exec sleep 300
EOF

chmod +x "$STUB"/*
# 模拟 koolshare 的 base.sh：里面正是把 echo_date 定义成别名
cat > "$KS/scripts/base.sh" <<BASEEOF
#!/bin/sh
alias echo_date='echo -n ""'
dbus() {
	printf 'dbus %s\n' "\$*" >> "$ROOT/dbus.log"
}
BASEEOF

# ---------- 复制并改路径 ----------
mkdir -p "$ROOT/pkg"
cp -R "$PKG_SRC" "$ROOT/pkg/mlclash"
cp -f "$STUB/mihomo" "$ROOT/pkg/mlclash/bin/mihomo"
cp -f "$STUB/mlclash-panel" "$ROOT/pkg/mlclash/bin/mlclash-panel"
chmod +x "$ROOT/pkg/mlclash/bin/"*
for f in "$ROOT/pkg/mlclash/install.sh" "$ROOT/pkg/mlclash/uninstall.sh" \
	"$ROOT/pkg/mlclash/scripts/mlclash_ctl.sh" "$ROOT/pkg/mlclash/scripts/setup.sh"; do
	sed -i '' -e "s#/koolshare#$KS#g" "$f" 2>/dev/null || sed -i -e "s#/koolshare#$KS#g" "$f"
done

# ---------- 模拟已装 2.x：目标文件已存在，安装必须迁移/覆盖 ----------
echo "=== 安装（从 2.x 覆盖升级）==="
echo "OLD-KERNEL" > "$KS/bin/mihomo"
echo "OLD-PANEL" > "$KS/bin/mihomo-panel"
mkdir -p "$KS/mihomo" && echo '{"mode":"rule","adBlock":true}' > "$KS/mihomo/settings.json"
cp -f "$STUB"/* "$KS/bin/" 2>/dev/null || true
MLCLASH_PAYLOAD_KB=14644 run_sh "$ROOT/pkg/mlclash/install.sh" > "$ROOT/install.log" 2>&1 || {
	cat "$ROOT/install.log"
}
tail -6 "$ROOT/install.log"

# ---------- 断言 ----------
if grep -v '^[[:space:]]*#' "$ROOT/pkg/mlclash/install.sh" | grep -qE 'echo_date[[:space:]]*\(\)'; then
	fail "install.sh 仍定义 echo_date()，会被 base.sh 的别名展开成语法错误"
fi
grep -q 'mm_log()' "$ROOT/pkg/mlclash/install.sh" || fail "install.sh 缺少 mm_log() 日志函数"
ok "日志函数为 mm_log（避开 base.sh 的 echo_date 别名）"

grep -q 'base\.sh' "$ROOT/pkg/mlclash/install.sh" || fail "install.sh 没有 source base.sh"
ok "install.sh 会 source base.sh（取 dbus 等固件函数）"

[ -x "$KS/bin/mihomo" ] || fail "内核未安装"
ok "内核已安装到 $KS/bin/mihomo（覆盖旧文件成功）"
grep -q "OLD-KERNEL" "$KS/bin/mihomo" && fail "旧内核未被覆盖（busybox cp 覆盖问题回归）"

[ -x "$KS/bin/mlclash-panel" ] || fail "面板未安装"
[ ! -e "$KS/bin/mihomo-panel" ] || fail "旧面板未清理"
ok "面板已安装，旧面板已清理"

[ -x "$KS/scripts/mlclash_ctl.sh" ] || fail "控制脚本未安装"
[ ! -e "$KS/scripts/mihomo_ctl.sh" ] || fail "旧控制脚本未清理"
ok "控制脚本已安装，旧脚本已清理"

[ -x "$KS/scripts/uninstall_mlclash.sh" ] || fail "软件中心卸载钩子未安装（按钮会失效）"
[ ! -e "$KS/scripts/uninstall_mihomo.sh" ] || fail "旧卸载钩子未清理"
ok "卸载钩子 /scripts/uninstall_mlclash.sh 已安装"

[ -L "$KS/init.d/S99mlclash.sh" ] || fail "缺少开机自启软链 S99mlclash"
[ -L "$KS/init.d/N99mlclash.sh" ] || fail "缺少关机停止软链 N99mlclash"
[ ! -e "$KS/init.d/S99mihomo.sh" ] || fail "旧 S99 软链未清理"
ok "init.d S99/N99 软链已更新"

[ -f "$KS/webs/Module_mlclash.asp" ] || fail "菜单页未安装"
[ -f "$KS/res/icon-mlclash.png" ] || fail "图标未安装"
[ ! -e "$KS/webs/Module_mihomo.asp" ] || fail "旧菜单页未清理"
ok "菜单页与图标已安装，旧文件已清理"

[ -f "$ROOT/pkg/mlclash/.valid" ] || fail "安装包缺少 .valid 平台标记（软件中心会拒绝安装）"
[ "$(cat "$ROOT/pkg/mlclash/.valid" | tr -d '\n')" = "hnd" ] || fail ".valid 内容不是 hnd"
ok ".valid 平台标记正确：hnd"

grep -q 'PKG_ARCH = "hnd"' "$ROOT/pkg/mlclash/webs/Module_mlclash.asp" || fail "ASP 缺少 PKG_ARCH=hnd 元信息"
grep -q 'PKG_NAME = "mlclash"' "$ROOT/pkg/mlclash/webs/Module_mlclash.asp" || fail "ASP 缺少 PKG_NAME=mlclash"
ok "ASP 元信息正确"

[ -f "$KS/mlclash/rules/chnroute.txt" ] || fail "规则文件未安装"
[ -d "$KS/mlclash/providers" ] || fail "数据目录未创建"
[ -f "$KS/mlclash/settings.json" ] || fail "旧设置未迁移"
grep -q '"adBlock":true' "$KS/mlclash/settings.json" || fail "旧设置内容不对"
[ ! -d "$KS/mihomo" ] || fail "旧数据目录未迁移/清理"
ok "规则与数据目录已就位，旧设置已迁移"

for k in name title description version install; do
	grep -q "softcenter_module_mlclash_${k}=" "$ROOT/dbus.log" || fail "缺少 dbus 键 softcenter_module_mlclash_${k}"
done
grep -q 'softcenter_module_mlclash_install=1' "$ROOT/dbus.log" || fail "install 标记不是 1"
grep -q 'dbus remove softcenter_module_mihomo_' "$ROOT/dbus.log" || fail "旧 dbus 键未清理"
ok "软件中心菜单登记完整，旧键已清理"

grep -q "剩余空间：17548KB" "$ROOT/install.log" || fail "安装脚本应打印剩余空间"
grep -q "本包需要 15668KB" "$ROOT/install.log" || fail "安装脚本应按实际大小报告需要的空间（14644+1024）"
ok "空间检查按实际大小：$(grep -o '剩余空间：[^】]*' "$ROOT/install.log" | head -1)"

[ "$(cat "$KS/mlclash/version" 2>/dev/null | tr -d '\n')" = "$(cat "$PROJ/plugin/version" | tr -d '\n')" ] \
	|| fail "版本号未写入数据目录"
ok "版本号已写入：$(cat "$KS/mlclash/version")"

# ---------- 空间不足时应拒绝安装 ----------
echo
echo "=== 空间不足场景 ==="
ROOT2="$ROOT/pkg2"
cp -R "$ROOT/pkg" "$ROOT2"
MLCLASH_FAKE_FREE_KB=5000 MLCLASH_PAYLOAD_KB=14644 run_sh "$ROOT2/mlclash/install.sh" > "$ROOT/install-low.log" 2>&1 && fail "空间不足时不该安装成功"
grep -q "剩余空间不足" "$ROOT/install-low.log" || fail "空间不足时缺少明确提示"
grep -q "本包需要" "$ROOT/install-low.log" || fail "空间不足提示应说明需要多少"
ok "空间不足时明确拒绝：$(grep -o '剩余空间不足[^】]*' "$ROOT/install-low.log" | head -1)"
rm -rf "$ROOT2"

# ---------- 跑卸载 ----------
echo
echo "=== 卸载 ==="
run_sh "$KS/scripts/uninstall_mlclash.sh" > "$ROOT/uninstall.log" 2>&1 || fail "卸载脚本执行失败"
cat "$ROOT/uninstall.log"

[ ! -e "$KS/bin/mihomo" ] || fail "内核未删除"
[ ! -e "$KS/bin/mlclash-panel" ] || fail "面板未删除"
[ ! -e "$KS/scripts/uninstall_mlclash.sh" ] || fail "卸载脚本未自清理"
[ ! -e "$KS/webs/Module_mlclash.asp" ] || fail "菜单页未删除"
[ ! -L "$KS/init.d/S99mlclash.sh" ] || fail "自启软链未删除"
ok "程序文件、菜单页、自启软链均已清理"

[ ! -d "$KS/mlclash" ] || fail "数据目录未删除（卸载应释放空间）"
[ ! -d "$KS/mihomo" ] || fail "旧数据目录未删除"
ok "数据目录已彻底删除（含订阅/节点/规则/内核备份/日志）"

grep -q "softcenter_module_mlclash_install" "$ROOT/dbus.log" && grep -q "dbus remove softcenter_module_mlclash_install" "$ROOT/dbus.log" \
	|| fail "dbus 登记未清理"
ok "dbus 菜单登记已清理"
