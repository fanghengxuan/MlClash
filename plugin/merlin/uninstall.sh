#!/bin/sh
# MlClash 卸载脚本（Asuswrt-Merlin / gnuton，无软件中心）
#
# 用法：uninstall.sh [--keep-data]
#   默认删除 /jffs/mlclash 全部内容（含订阅/节点/规则/日志）
#   --keep-data 只删程序，保留 data/ 下的订阅与设置
#   同时兼容清理 2.x 的 /jffs/mihomo-merlin。

ROOT=${MLCLASH_ROOT:-}
JFFS=$ROOT/jffs
BASE=$JFFS/mlclash
OLD_BASE=$JFFS/mihomo-merlin
SCRIPTS=$JFFS/scripts
KEEP=0
[ "${1:-}" = "--keep-data" ] && KEEP=1

mm_log() { echo "【$(date '+%Y-%m-%d %H:%M:%S')】: $*"; }

for d in "$BASE" "$OLD_BASE"; do
	if [ -x "$d/ctl.sh" ]; then
		"$d/ctl.sh" stop >/dev/null 2>&1
	fi
done
killall -q mlclash-panel 2>/dev/null
killall -q mihomo-panel 2>/dev/null

remove_hook() {
	file=$1
	[ -f "$file" ] || return 0
	tmp="$file.mlclash.$$"
	awk '
		/^# >>> (mihomo-merlin|mlclash) >>>/{skip=1; next}
		/^# <<< (mihomo-merlin|mlclash) <<</{skip=0; next}
		skip!=1{print}
	' "$file" >"$tmp" || { rm -f "$tmp"; mm_log "更新 $file 失败"; return 1; }
	cat "$tmp" >"$file"
	rm -f "$tmp"
}

mm_log "清理开机自启与防火墙钩子"
remove_hook "$SCRIPTS/services-start"
remove_hook "$SCRIPTS/services-stop"
remove_hook "$SCRIPTS/nat-start"

# 兜底清理 iptables（正常情况下 ctl stop / 面板退出时已经清过）
if command -v iptables >/dev/null 2>&1; then
	lanif=$(nvram get lan_ifname 2>/dev/null)
	[ -n "$lanif" ] || lanif=br0
	for chain in MLCLASH MIHOMO; do
		i=0
		while [ "$i" -lt 5 ]; do
			iptables -t nat -D PREROUTING -i "$lanif" -j "$chain" 2>/dev/null || break
			i=$((i + 1))
		done
		iptables -t nat -F "$chain" 2>/dev/null
		iptables -t nat -X "$chain" 2>/dev/null
	done
	for chain in MLCLASH_OUT MIHOMO_OUT; do
		i=0
		while [ "$i" -lt 5 ]; do
			iptables -t nat -D OUTPUT -j "$chain" 2>/dev/null || break
			i=$((i + 1))
		done
		iptables -t nat -F "$chain" 2>/dev/null
		iptables -t nat -X "$chain" 2>/dev/null
	done
	iptables -D FORWARD -p udp --dport 443 -j REJECT 2>/dev/null
	i=0
	while [ "$i" -lt 3 ]; do
		iptables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || break
		i=$((i + 1))
	done
fi

if command -v ip6tables >/dev/null 2>&1; then
	for chain in MLCLASH6 MLCLASH6_OUT; do
		i=0
		while [ "$i" -lt 5 ]; do
			if [ "$chain" = "MLCLASH6" ]; then
				ip6tables -t nat -D PREROUTING -i "$lanif" -j "$chain" 2>/dev/null || break
			else
				ip6tables -t nat -D OUTPUT -j "$chain" 2>/dev/null || break
			fi
			i=$((i + 1))
		done
		ip6tables -t nat -F "$chain" 2>/dev/null
		ip6tables -t nat -X "$chain" 2>/dev/null
	done
	i=0
	while [ "$i" -lt 5 ]; do
		ip6tables -D FORWARD -p udp --dport 443 -j REJECT 2>/dev/null || break
		i=$((i + 1))
	done
	i=0
	while [ "$i" -lt 3 ]; do
		ip6tables -t mangle -D FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || break
		i=$((i + 1))
	done
fi

if [ "$KEEP" = "1" ]; then
	for d in "$BASE" "$OLD_BASE"; do
		[ -d "$d" ] || continue
		mm_log "保留数据目录：$d/data"
		rm -rf "$d/bin" "$d/scripts" "$d/ctl.sh" "$d/uninstall.sh" "$d/version" 2>/dev/null
	done
else
	for d in "$BASE" "$OLD_BASE"; do
		[ -d "$d" ] || continue
		mm_log "删除 $d"
		rm -rf "$d"
	done
fi
rm -rf "${MLCLASH_RUNTIME_DIR:-/tmp/mlclash}" "${MLCLASH_OLD_RUNTIME_DIR:-/tmp/mihomo}" 2>/dev/null

mm_log "卸载完成"
