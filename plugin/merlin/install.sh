#!/bin/sh
# MlClash 安装脚本（Asuswrt-Merlin / gnuton，无软件中心）
# 安装位置：/jffs/mlclash
# 从 2.x（/jffs/mihomo-merlin）升级时，自动迁移数据、/tmp 运行目录与开机钩子。
# 卸载：sh /jffs/mlclash/uninstall.sh [--keep-data]
#
# MLCLASH_ROOT 仅用于测试（假根目录），路由器上不要设置。

DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=${MLCLASH_ROOT:-}
JFFS=$ROOT/jffs
BASE=$JFFS/mlclash
OLD_BASE=$JFFS/mihomo-merlin
BIN=$BASE/bin
DATA=$BASE/data
SCRIPTS=$JFFS/scripts
RT=${MLCLASH_RUNTIME_DIR:-/tmp/mlclash}
OLD_RT=${MLCLASH_OLD_RUNTIME_DIR:-/tmp/mihomo}
PLVER=$(cat "$DIR/version" 2>/dev/null || echo unknown)

mm_log() { echo "【$(date '+%Y-%m-%d %H:%M:%S')】: $*"; }

fail() {
	mm_log "$*"
	if [ "${ROLLBACK_READY:-0}" = "1" ]; then
		rollback
	fi
	exit 1
}

mm_log "================================================"
mm_log " MlClash v${PLVER}（Asuswrt-Merlin 安装程序）"
mm_log " 机型：$(nvram get productid 2>/dev/null || echo 未知)  架构：$(uname -m)  内核：$(uname -r)"
mm_log "================================================"

[ -d "$JFFS" ] || fail "未检测到 /jffs。Merlin 固件需要在「系统管理 → 系统设置」里启用 JFFS。"
[ -x "$DIR/bin/mlclash-panel" ] || fail "安装包缺少 bin/mlclash-panel"

ARCH=$(uname -m)
PKG_ARCH=$(cat "$DIR/arch" 2>/dev/null)
[ -n "$PKG_ARCH" ] || PKG_ARCH=armv7l
[ "$ARCH" = "$PKG_ARCH" ] || fail "本安装包是 ${PKG_ARCH} 架构，当前设备是 ${ARCH}。请换对应架构的安装包。"

SCRIPTS_ON=$(nvram get jffs2_scripts 2>/dev/null || echo "")
if [ "$SCRIPTS_ON" != "1" ]; then
	if [ "${MLCLASH_ALLOW_NO_SCRIPTS:-0}" = "1" ]; then
		mm_log "警告：JFFS 自定义脚本未开启，开机自启不会生效（继续安装）"
	else
		fail "请先在「系统管理 → 系统设置」勾选 Enable JFFS custom scripts and configs（或执行 nvram set jffs2_scripts=1 && nvram commit），再重新安装。"
	fi
fi

PAYLOAD_KB=$(du -sk "$DIR" 2>/dev/null | awk '{print $1}')
[ -n "$PAYLOAD_KB" ] || PAYLOAD_KB=0
NEED_KB=$((PAYLOAD_KB + 1024))
[ "$NEED_KB" -ge 2048 ] || NEED_KB=2048
# 自检：在写入前确认二进制能在这台机器上跑
"$DIR/bin/mlclash-panel" -v >/dev/null 2>&1 || fail "面板程序无法运行（架构不匹配或文件损坏）"
if [ -f "$DIR/bin/mihomo" ]; then
	"$DIR/bin/mihomo" -v >/dev/null 2>&1 || fail "内核无法运行（架构不匹配或文件损坏）"
fi

# 停止旧版 / 当前版本
for d in "$OLD_BASE" "$BASE"; do
	if [ -x "$d/ctl.sh" ]; then
		mm_log "停止 $(basename "$d")"
		"$d/ctl.sh" stop >/dev/null 2>&1
	fi
done
# 兜底：旧版 ctl 缺失时直接回收进程
killall -q mlclash-panel 2>/dev/null
killall -q mihomo-panel 2>/dev/null
killall -q mihomo 2>/dev/null

mkdir -p "$BIN" "$SCRIPTS"

BACKUP_DIR="/tmp/.mlclash-migrate.$$"
HOOK_BACKUP="$BACKUP_DIR/hooks"
BIN_BACKUP="$BACKUP_DIR/bin"
mkdir -p "$HOOK_BACKUP" "$BIN_BACKUP/old" "$BIN_BACKUP/current"
MIGRATED=0
ROLLBACK_READY=1

# 备份钩子，失败时恢复
for f in services-start services-stop nat-start; do
	[ -f "$SCRIPTS/$f" ] && cp -f "$SCRIPTS/$f" "$HOOK_BACKUP/$f" 2>/dev/null
done

rollback() {
	ROLLBACK_READY=0
	mm_log "安装失败，开始回滚"
	[ -x "$BASE/ctl.sh" ] && "$BASE/ctl.sh" stop >/dev/null 2>&1
	killall -q mlclash-panel 2>/dev/null
	for f in services-start services-stop nat-start; do
		[ -f "$HOOK_BACKUP/$f" ] && cp -f "$HOOK_BACKUP/$f" "$SCRIPTS/$f" 2>/dev/null
	done
	if [ "$MIGRATED" = "1" ] && [ -d "$DATA" ] && [ ! -e "$OLD_BASE/data" ]; then
		mv "$DATA" "$OLD_BASE/data" 2>/dev/null && mm_log "数据已移回 $OLD_BASE/data"
	fi
	for f in mihomo mihomo-panel; do
		[ -e "$BIN_BACKUP/old/$f" ] && { mkdir -p "$OLD_BASE/bin"; mv -f "$BIN_BACKUP/old/$f" "$OLD_BASE/bin/$f"; }
	done
	for f in mihomo mlclash-panel; do
		[ -e "$BIN_BACKUP/current/$f" ] && mv -f "$BIN_BACKUP/current/$f" "$BIN/$f"
	done
	if [ -x "$OLD_BASE/ctl.sh" ]; then
		"$OLD_BASE/ctl.sh" start >/dev/null 2>&1 && mm_log "旧版本已恢复启动"
	fi
}

# 迁移 2.x 数据目录（同一文件系统 rename，不复制、不占额外空间）
if [ -d "$OLD_BASE/data" ] && [ ! -e "$DATA" ]; then
	mm_log "迁移旧数据：$OLD_BASE/data -> $DATA"
	if mv "$OLD_BASE/data" "$DATA"; then
		MIGRATED=1
	else
		fail "迁移旧数据失败（$OLD_BASE/data）"
	fi
elif [ -d "$OLD_BASE/data" ] && [ -e "$DATA" ]; then
	mm_log "警告：新数据目录已存在，保留旧数据 $OLD_BASE/data 不覆盖"
fi
mkdir -p "$DATA/rules" "$DATA/providers"

# 迁移 /tmp 运行目录（旧版 logs/kernel 软链指向 /tmp/mihomo）
mkdir -p "$RT"
for d in logs kernel; do
	if [ -d "$OLD_RT/$d" ] && [ ! -e "$RT/$d" ]; then
		if mv "$OLD_RT/$d" "$RT/$d" 2>/dev/null; then
			mm_log "迁移运行目录 $OLD_RT/$d -> $RT/$d"
		elif cp -R "$OLD_RT/$d" "$RT/$d" 2>/dev/null; then
			rm -rf "$OLD_RT/$d"
			mm_log "复制运行目录 $OLD_RT/$d -> $RT/$d"
		else
			mm_log "警告：$OLD_RT/$d 迁移失败，继续安装"
		fi
	fi
done
rmdir "$OLD_RT" 2>/dev/null

# 先更新控制脚本，再用它把 logs/kernel 软链指向新运行目录
cp -f "$DIR/scripts/ctl.sh" "$BASE/ctl.sh" || fail "复制控制脚本失败"
chmod 755 "$BASE/ctl.sh"
"$BASE/ctl.sh" prepare >/dev/null 2>&1

# 把旧二进制挪到 /tmp 暂存：jffs2 碎片化时，峰值只保留一份新文件
for f in mihomo mihomo-panel; do
	[ -e "$OLD_BASE/bin/$f" ] && mv -f "$OLD_BASE/bin/$f" "$BIN_BACKUP/old/$f" 2>/dev/null
done
for f in mihomo mlclash-panel; do
	[ -e "$BIN/$f" ] && mv -f "$BIN/$f" "$BIN_BACKUP/current/$f" 2>/dev/null
done

FREE_KB=$(df -k "$JFFS" 2>/dev/null | awk 'NR==2 {print $4}')
if [ -n "$FREE_KB" ] && [ "$FREE_KB" -lt "$NEED_KB" ] 2>/dev/null; then
	fail "剩余空间不足：本包需要 ${NEED_KB}KB，当前只有 ${FREE_KB}KB。请清理 /jffs 后重试。"
fi
mm_log "安装前剩余空间：${FREE_KB:-未知}KB（本包需要 ${NEED_KB}KB）"

install_file() {
	src=$1
	dst=$2
	if ! cp -f "$src" "$dst.tmp"; then
		rm -f "$dst.tmp" 2>/dev/null
		fail "复制 $src 失败（空间不足或文件被占用）"
	fi
	chmod 755 "$dst.tmp" 2>/dev/null
	if ! mv -f "$dst.tmp" "$dst"; then
		rm -f "$dst.tmp" 2>/dev/null
		fail "写入 $dst 失败"
	fi
}

mm_log "安装面板与内核"
install_file "$DIR/bin/mlclash-panel" "$BIN/mlclash-panel"
if [ -f "$DIR/bin/mihomo" ]; then
	install_file "$DIR/bin/mihomo" "$BIN/mihomo"
else
	mm_log "本包不含内核，稍后可在面板「设置 → 内核 → 检查更新」下载，或手动上传"
fi

# 规则只在缺失时补，升级不覆盖用户已更新的 chnroute.txt
if [ ! -s "$DATA/rules/chnroute.txt" ] && [ -f "$DIR/rules/chnroute.txt" ]; then
	cp -f "$DIR/rules/chnroute.txt" "$DATA/rules/chnroute.txt" 2>/dev/null
fi
cp -f "$DIR/uninstall.sh" "$BASE/uninstall.sh" || fail "复制卸载脚本失败"
cp -f "$DIR/version" "$BASE/version" 2>/dev/null
chmod 755 "$BASE/uninstall.sh"

# 在 services-start / services-stop / nat-start 中维护托管块（同时清掉旧版标记）
install_hook() {
	file=$1
	action=$2
	[ -f "$file" ] || printf '#!/bin/sh\n' >"$file"
	tmp="$file.mlclash.$$"
	awk '
	/^# >>> (mihomo-merlin|mlclash) >>>/{skip=1; next}
	/^# <<< (mihomo-merlin|mlclash) <<</{skip=0; next}
	skip!=1{print}
	' "$file" >"$tmp" || { rm -f "$tmp"; fail "更新 $file 失败"; }
	cat "$tmp" >"$file"
	rm -f "$tmp"
	{
		echo ""
		echo "# >>> mlclash >>>"
		echo "$BASE/ctl.sh $action"
		echo "# <<< mlclash <<<"
	} >>"$file"
	chmod a+rx "$file"
}

mm_log "配置开机自启与防火墙钩子"
install_hook "$SCRIPTS/services-start" start
install_hook "$SCRIPTS/services-stop" stop
install_hook "$SCRIPTS/nat-start" reapply

mm_log "启动面板"
if ! "$BASE/ctl.sh" start; then
	fail "面板启动失败，查看 $DATA/logs/panel.log"
fi

# 启动成功：清理旧版本目录、旧二进制暂存与旧运行目录
ROLLBACK_READY=0
if [ -d "$OLD_BASE" ]; then
	if [ -d "$OLD_BASE/data" ]; then
		mm_log "旧目录仍保留 data，未删除 $OLD_BASE"
	else
		rm -rf "$OLD_BASE" 2>/dev/null && mm_log "已移除旧版本目录 $OLD_BASE"
	fi
fi
rm -rf "$BACKUP_DIR" 2>/dev/null
rm -rf "$OLD_RT" 2>/dev/null

LANIP=$(nvram get lan_ipaddr 2>/dev/null)
[ -n "$LANIP" ] || LANIP="路由器IP"
mm_log "================================================"
mm_log " 安装完成"
mm_log " 管理面板：http://${LANIP}:9380"
mm_log " 卸载：sh $BASE/uninstall.sh [--keep-data]"
mm_log " 注意：首次使用请在「概览」点一次启动，之后开机会自动拉起内核"
mm_log "================================================"
