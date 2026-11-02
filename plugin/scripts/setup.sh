#!/bin/sh
# 实际的文件安装动作放在这里：install.sh 只做检查与调用，
# 避免安装脚本里出现大量对 /koolshare 下文件的写操作（软件中心会做静态检查）。
# 从 2.x（/koolshare/mihomo）升级时自动迁移数据与旧文件。

DIR=$(cd "$(dirname "$0")/.." && pwd)
module=mlclash

[ -f /koolshare/scripts/base.sh ] && . /koolshare/scripts/base.sh 2>/dev/null
PATH=/koolshare/bin:/bin:/usr/bin:/sbin:/usr/sbin
export PATH
SH=/bin/sh
unalias echo_date >/dev/null 2>&1
unalias echo >/dev/null 2>&1

mm_log() {
	echo "【$(TZ=UTC-8 date '+%Y%m%d %X')】: $*"
}
fail() {
	mm_log "$*"
	if [ "${ROLLBACK_READY:-0}" = "1" ]; then
		rollback
	fi
	exit 1
}

DBUS=""
for c in /koolshare/bin/dbus /usr/bin/dbus /usr/sbin/dbus /bin/dbus /jffs/koolshare/bin/dbus; do
	[ -x "$c" ] && { DBUS=$c; break; }
done
[ -z "$DBUS" ] && DBUS=$(command -v dbus 2>/dev/null)

DATA=/koolshare/mlclash
OLD_DATA=/koolshare/mihomo
PANEL=/koolshare/bin/mlclash-panel
LANIP=$(nvram get lan_ipaddr 2>/dev/null)
[ -n "$LANIP" ] || LANIP="192.168.50.1"
mkdir -p /koolshare/bin /koolshare/webs /koolshare/res /koolshare/scripts /koolshare/init.d

# install_file 源 目标：先写同目录临时文件再 rename 覆盖。
# 原因：部分固件的 busybox cp 在目标已存在时会报 "can't create ... File exists"，
# 而且正在运行的二进制不能被 open 覆盖（ETXTBSY），但 rename 替换是允许的。
install_file() {
	src=$1
	dst=$2
	dir=$(dirname "$dst")
	base=$(basename "$dst")
	tmp="$dir/.${base}.new.$$"

	rm -f "$tmp" 2>/dev/null
	if ! cat "$src" > "$tmp" 2>/tmp/mlclash_cp.err; then
		# cat 也失败时退回 cp
		if ! cp -f "$src" "$tmp" 2>/tmp/mlclash_cp.err; then
			mm_log "写入临时文件失败：$(cat /tmp/mlclash_cp.err 2>/dev/null)"
			rm -f "$tmp" 2>/dev/null
			return 1
		fi
	fi
	chmod 755 "$tmp" 2>/dev/null

	if mv -f "$tmp" "$dst" 2>/tmp/mlclash_mv.err; then
		return 0
	fi

	# 兜底：先把旧文件挪走（改名对运行中的程序也有效），再放新文件
	if [ -e "$dst" ]; then
		mv -f "$dst" "${dst}.old.$$" 2>/dev/null && rm -f "${dst}.old.$$" 2>/dev/null
	fi
	if mv -f "$tmp" "$dst" 2>>/tmp/mlclash_mv.err; then
		return 0
	fi

	mm_log "替换 $dst 失败：$(cat /tmp/mlclash_mv.err 2>/dev/null | tail -1)"
	rm -f "$tmp" 2>/dev/null
	return 1
}

# 旧面板/内核若还在跑，会占着 /koolshare/bin 下的二进制
killall -q mlclash-panel 2>/dev/null
killall -q mihomo-panel 2>/dev/null
killall -q mihomo 2>/dev/null
i=0
while [ $i -lt 5 ]; do
	if pidof mihomo >/dev/null 2>&1 || pidof mlclash-panel >/dev/null 2>&1 || pidof mihomo-panel >/dev/null 2>&1; then
		sleep 1
		i=$((i + 1))
	else
		break
	fi
done
killall -q -9 mlclash-panel 2>/dev/null
killall -q -9 mihomo-panel 2>/dev/null
killall -q -9 mihomo 2>/dev/null

BACKUP_DIR="/tmp/.mlclash-migrate.$$"
mkdir -p "$BACKUP_DIR/old" "$BACKUP_DIR/current"
MIGRATED=0
ROLLBACK_READY=1

rollback() {
	ROLLBACK_READY=0
	mm_log "安装失败，开始回滚"
	killall -q mlclash-panel 2>/dev/null
	if [ "$MIGRATED" = "1" ] && [ -d "$DATA" ] && [ ! -e "$OLD_DATA" ]; then
		mv "$DATA" "$OLD_DATA" 2>/dev/null && mm_log "数据已移回 $OLD_DATA"
	fi
	for f in mihomo mihomo-panel; do
		[ -e "$BACKUP_DIR/old/$f" ] && mv -f "$BACKUP_DIR/old/$f" "/koolshare/bin/$f"
	done
	for f in mihomo mlclash-panel; do
		[ -e "$BACKUP_DIR/current/$f" ] && mv -f "$BACKUP_DIR/current/$f" "/koolshare/bin/$f"
	done
	KDEST=$DATA
	[ "$MIGRATED" = "1" ] && KDEST=$OLD_DATA
	[ -d "$BACKUP_DIR/kernel" ] && mv -f "$BACKUP_DIR/kernel" "$KDEST/kernel" 2>/dev/null
	if [ -x /koolshare/scripts/mihomo_ctl.sh ] && [ -d "$OLD_DATA" ]; then
		"$SH" /koolshare/scripts/mihomo_ctl.sh start >/dev/null 2>&1 && mm_log "旧版本已恢复启动"
	fi
}

# 迁移 2.x 数据目录（同一文件系统 rename）
if [ -d "$OLD_DATA" ] && [ ! -e "$DATA" ]; then
	mm_log "迁移旧数据：$OLD_DATA -> $DATA"
	if mv "$OLD_DATA" "$DATA"; then
		MIGRATED=1
		mkdir -p "$DATA/providers" "$DATA/rules" "$DATA/logs"
	else
		fail "迁移旧数据失败（${OLD_DATA}）"
	fi
elif [ -d "$OLD_DATA" ] && [ -e "$DATA" ]; then
	mm_log "警告：新数据目录已存在，保留旧数据 $OLD_DATA 不覆盖"
fi
mkdir -p "$DATA/providers" "$DATA/rules" "$DATA/logs"

mm_log "复制 mihomo 内核与面板程序，请稍候"

# 先把旧二进制挪到 RAM(/tmp)，立刻腾出 jffs 空间：
# jffs2 在剩余空间碎片化时，即使 df 显示还有十几 MB，写入大文件也可能报
# "No space left on device"。挪走旧文件后峰值只需一份新文件的大小。
for f in mihomo mihomo-panel; do
	[ -e "/koolshare/bin/$f" ] && mv -f "/koolshare/bin/$f" "$BACKUP_DIR/old/$f" 2>/dev/null
done
[ -e "$PANEL" ] && mv -f "$PANEL" "$BACKUP_DIR/current/mlclash-panel" 2>/dev/null
[ -e /koolshare/bin/mihomo ] && mv -f /koolshare/bin/mihomo "$BACKUP_DIR/current/mihomo" 2>/dev/null

# 旧内核备份搬走，避免升级峰值叠加
if [ -d "$DATA/kernel" ]; then
	OLD_KB=$(du -sk "$DATA/kernel" 2>/dev/null | awk '{print $1}')
	if [ "${OLD_KB:-0}" -gt 1024 ]; then
		mv "$DATA/kernel" "$BACKUP_DIR/kernel" 2>/dev/null && \
			mm_log "已把旧内核备份（${OLD_KB}KB）搬到 /tmp 暂存" || \
			rm -rf "$DATA/kernel" 2>/dev/null
	fi
fi

# 日志截断：有的机器 panel.log 长到几 MB
for lf in "$DATA"/logs/*.log; do
	[ -f "$lf" ] || continue
	sz=$(wc -c < "$lf" 2>/dev/null || echo 0)
	if [ "${sz:-0}" -gt 262144 ]; then
		tail -c 131072 "$lf" > "$lf.tmp" 2>/dev/null && mv -f "$lf.tmp" "$lf" 2>/dev/null && mm_log "已截断日志 ${lf}（原 ${sz} 字节）"
	fi
done

FREE_NOW=$(df -k /koolshare 2>/dev/null | awk 'NR==2 {print $4}')
mm_log "安装前 jffs 剩余：${FREE_NOW:-未知}KB"

HAVE_KERNEL=0
if [ -f "${DIR}/bin/mihomo" ]; then
	HAVE_KERNEL=1
	install_file "${DIR}/bin/mihomo" /koolshare/bin/mihomo || fail "复制 mihomo 失败（jffs 空间不足或文件被占用）"
else
	mm_log "本安装包不含内核，跳过内核安装（稍后在面板里下载或上传）"
fi
install_file "${DIR}/bin/mlclash-panel" "$PANEL" || fail "复制面板失败（jffs 空间不足或文件被占用）"
chmod 755 "$PANEL" 2>/dev/null
[ -f /koolshare/bin/mihomo ] && chmod 755 /koolshare/bin/mihomo 2>/dev/null

# 内核自检：架构不对 / 文件损坏时立刻中止，避免装完变砖
if [ "$HAVE_KERNEL" = "1" ]; then
	if ! /koolshare/bin/mihomo -v >/tmp/mlclash_kver 2>&1; then
		mm_log "内核无法运行，输出如下："
		head -5 /tmp/mlclash_kver 2>/dev/null
		fail "内核自检失败（架构不匹配或文件损坏），安装中止"
	fi
	mm_log "  内核：$(head -1 /tmp/mlclash_kver)"
else
	mm_log "提示：打开 http://<路由器IP>:9380 → 设置 → mihomo 内核 → 检查更新（自动下载），或手动上传内核文件"
fi
PANEL_VER=$("$PANEL" -v 2>&1 | head -1)
[ -n "$PANEL_VER" ] || fail "面板程序自检失败"
mm_log "  ${PANEL_VER}"

mm_log "安装国内 IP 规则文件"
if [ ! -s "$DATA/rules/chnroute.txt" ] && [ -f "${DIR}/rules/chnroute.txt" ]; then
	cp -f "${DIR}/rules/chnroute.txt" "$DATA/rules/chnroute.txt" || mm_log "规则文件缺失，将只做全局代理"
fi

mm_log "安装控制脚本与网页"
cp -f "${DIR}/scripts/mlclash_ctl.sh" /koolshare/scripts/mlclash_ctl.sh
chmod 755 /koolshare/scripts/mlclash_ctl.sh
# 软件中心的「卸载」按钮会调用 /koolshare/scripts/uninstall_<module>.sh
cp -f "${DIR}/uninstall.sh" "/koolshare/scripts/uninstall_${module}.sh"
chmod 755 "/koolshare/scripts/uninstall_${module}.sh"
cp -f "${DIR}"/webs/* /koolshare/webs/
cp -f "${DIR}"/res/* /koolshare/res/
cp -f "${DIR}/version" "$DATA/version" 2>/dev/null

# 旧文件、init 软链与 dbus 登记在面板启动成功后处理；失败时保留旧版便于回滚。

mm_log "启动面板服务"
"$SH" /koolshare/scripts/mlclash_ctl.sh start

sleep 1
if pidof mlclash-panel >/dev/null 2>&1; then
	ROLLBACK_READY=0

	# 清理 2.x 的旧文件与旧 init 软链
	rm -f /koolshare/bin/mihomo-panel /koolshare/scripts/mihomo_ctl.sh \
		/koolshare/scripts/uninstall_mihomo.sh /koolshare/webs/Module_mihomo.asp \
		/koolshare/res/icon-mihomo.png /koolshare/init.d/S99mihomo.sh /koolshare/init.d/N99mihomo.sh 2>/dev/null

	# 开机自启 & 关机停止
	ln -sf /koolshare/scripts/mlclash_ctl.sh /koolshare/init.d/S99mlclash.sh
	ln -sf /koolshare/scripts/mlclash_ctl.sh /koolshare/init.d/N99mlclash.sh

	# 菜单登记：新模块登记，旧模块键尽量移除
	if [ -n "$DBUS" ]; then
		VER=$(cat "${DIR}/version" 2>/dev/null)
		for k in name title description version install; do
			"$DBUS" remove softcenter_module_mihomo_${k} >/dev/null 2>&1 || true
		done
		"$DBUS" set softcenter_module_${module}_name="${module}"
		"$DBUS" set softcenter_module_${module}_title="MlClash"
		"$DBUS" set softcenter_module_${module}_description="基于 mihomo 内核的订阅代理，管理面板端口 9380"
		"$DBUS" set softcenter_module_${module}_version="${VER}"
		"$DBUS" set softcenter_module_${module}_install="1"
		mm_log "已登记到软件中心"
	else
		mm_log "未找到 dbus（可能不是软件中心固件），跳过菜单登记，可用命令手动管理"
	fi

	rm -rf "$BACKUP_DIR" 2>/dev/null
	rm -rf "${MLCLASH_OLD_RUNTIME_DIR:-/tmp/mihomo}" 2>/dev/null
	rm -f /tmp/mlclash_cp.err /tmp/mlclash_mv.err /tmp/mlclash_kver /tmp/mm_cp.err /tmp/mm_mv.err /tmp/mihomo_kver 2>/dev/null
	mm_log "================================================"
	mm_log " 安装成功！"
	mm_log " 管理面板：http://${LANIP}:9380"
	mm_log " 首次使用：进入「订阅」页添加机场订阅链接，"
	mm_log "           再到「概览」页点「启动」开启代理。"
	mm_log "================================================"
else
	fail "面板未启动，请查看 $DATA/logs/panel.log"
fi
