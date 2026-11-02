#!/bin/sh
# MlClash 插件卸载脚本：停进程、清 iptables、删程序与全部数据（彻底释放空间）
#
# 软件中心的「卸载」按钮会以无参方式调用本脚本；这里默认做「彻底卸载」：
# 连同 /koolshare/mlclash 下的订阅、节点文件、规则、内核备份、日志一起删除。
# 同时兼容清理 2.x 的 /koolshare/mihomo 与旧文件。
# 想保留数据再装回来，请先在面板「设置 → 备份与恢复」导出备份。

module=mlclash
DATA_DIR="/koolshare/mlclash"
OLD_DATA_DIR="/koolshare/mihomo"

if [ -x /koolshare/bin/dbus ]; then
	DBUS=/koolshare/bin/dbus
elif command -v dbus >/dev/null 2>&1; then
	DBUS=dbus
else
	DBUS=""
fi

unalias mm_log >/dev/null 2>&1
unalias echo >/dev/null 2>&1

mm_log() {
	echo "【$(TZ=UTC-8 date '+%Y%m%d %X')】: $*"
}

# 目录大小（KB），用于告诉用户释放了多少
dir_kb() {
	[ -d "$1" ] || { echo 0; return; }
	du -sk "$1" 2>/dev/null | awk '{print $1}' | head -1
}

BEFORE_KB=$(( $(dir_kb "$DATA_DIR") + $(dir_kb "$OLD_DATA_DIR") ))

mm_log "停止面板与内核"
for ctl in /koolshare/scripts/mlclash_ctl.sh /koolshare/scripts/mihomo_ctl.sh; do
	[ -x "$ctl" ] && sh "$ctl" stop >/dev/null 2>&1
done
# 兜底：ctl 脚本不在或没杀干净时直接按进程名收尾
killall -q mlclash-panel 2>/dev/null
killall -q mihomo-panel 2>/dev/null
killall -q mihomo 2>/dev/null
sleep 1
killall -q -9 mlclash-panel 2>/dev/null
killall -q -9 mihomo-panel 2>/dev/null
killall -q -9 mihomo 2>/dev/null

mm_log "清理 iptables 规则（含 DNS/QUIC/MSS）"
LANIF=$(nvram get lan_ifname 2>/dev/null)
[ -n "$LANIF" ] || LANIF=br0
# nat：透明代理链（多次删除，防止重复插入的残留），含 2.x 旧链
for chain in MLCLASH MIHOMO; do
	i=0
	while [ $i -lt 5 ]; do
		iptables -t nat -D PREROUTING -i "$LANIF" -j "$chain" 2>/dev/null || break
		i=$((i + 1))
	done
	iptables -t nat -F "$chain" 2>/dev/null
	iptables -t nat -X "$chain" 2>/dev/null
done
for chain in MLCLASH_OUT MIHOMO_OUT; do
	i=0
	while [ $i -lt 5 ]; do
		iptables -t nat -D OUTPUT -j "$chain" 2>/dev/null || break
		i=$((i + 1))
	done
	iptables -t nat -F "$chain" 2>/dev/null
	iptables -t nat -X "$chain" 2>/dev/null
done

# filter：阻断 QUIC 的规则
i=0
while [ $i -lt 5 ]; do
	iptables -D FORWARD -p udp --dport 443 -j REJECT 2>/dev/null || break
	i=$((i + 1))
done

# mangle：1.7.2 起加的 TCP MSS 钳制
i=0
while [ $i -lt 5 ]; do
	iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || break
	i=$((i + 1))
done

mm_log "删除程序文件"
rm -f /koolshare/bin/mihomo
rm -f /koolshare/bin/mlclash-panel /koolshare/bin/mihomo-panel
rm -f /koolshare/scripts/mlclash_ctl.sh /koolshare/scripts/mihomo_ctl.sh
rm -f /koolshare/webs/Module_mlclash.asp /koolshare/webs/Module_mihomo.asp
rm -f /koolshare/res/icon-mlclash.png /koolshare/res/icon-mihomo.png
rm -f /koolshare/init.d/S99mlclash.sh /koolshare/init.d/N99mlclash.sh
rm -f /koolshare/init.d/S99mihomo.sh /koolshare/init.d/N99mihomo.sh
# 卸载脚本自身
rm -f /koolshare/scripts/uninstall_mlclash.sh /koolshare/scripts/uninstall_mihomo.sh

mm_log "删除数据目录（订阅、节点文件、规则、内核备份、日志）"
# 只删本插件自己的目录，路径写死，避免误删
for d in "$DATA_DIR" "$OLD_DATA_DIR"; do
	[ -d "$d" ] && rm -rf "$d"
done

# 老版本可能把日志写到 /tmp，一并清掉
rm -f /tmp/mlclash-panel.log /tmp/mihomo-panel.log /tmp/mlclash.log /tmp/mihomo.log
rm -rf "${MLCLASH_RUNTIME_DIR:-/tmp/mlclash}" "${MLCLASH_OLD_RUNTIME_DIR:-/tmp/mihomo}"

if [ -n "$DBUS" ]; then
	for m in mlclash mihomo; do
		for k in name title description version install; do
			"$DBUS" remove "softcenter_module_${m}_${k}" 2>/dev/null
		done
	done
fi

AFTER_KB=$(( $(dir_kb "$DATA_DIR") + $(dir_kb "$OLD_DATA_DIR") ))
if [ "${BEFORE_KB:-0}" -gt 0 ]; then
	if [ "$BEFORE_KB" -ge 1024 ]; then
		mm_log "已释放约 $((BEFORE_KB / 1024)) MB（数据目录 ${BEFORE_KB} KB，剩余 ${AFTER_KB} KB）"
	else
		mm_log "已释放 ${BEFORE_KB} KB（数据目录已清空，剩余 ${AFTER_KB} KB）"
	fi
fi

if [ -d "$DATA_DIR" ] || [ -d "$OLD_DATA_DIR" ]; then
	mm_log "警告：数据目录仍在：$DATA_DIR / $OLD_DATA_DIR"
else
	mm_log "卸载完成，程序与数据已全部删除"
fi
