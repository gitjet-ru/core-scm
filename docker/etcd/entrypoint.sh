#!/bin/sh
# etcd 3.4 помечает arm64 как «unsupported» без этого флага (политика апстрима).
case "$(uname -m)" in
aarch64 | arm64) export ETCD_UNSUPPORTED_ARCH=arm64 ;;
esac
if [ "${1:-}" = etcd ]; then
	shift
fi
exec /usr/local/bin/etcd "$@"
