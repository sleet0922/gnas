#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
    legacy_unit=/etc/systemd/system/gnas.service
    if [ -f "$legacy_unit" ] \
        && grep -q '^Description=GNAS Service$' "$legacy_unit" \
        && ! grep -q '^ExecStart=/usr/local/bin/gnas$' "$legacy_unit"; then
        rm -f "$legacy_unit"
    fi
    # 清理旧版代码运行时写入的 qdrant.service
    legacy_qdrant=/etc/systemd/system/qdrant.service
    if [ -f "$legacy_qdrant" ]; then
        rm -f "$legacy_qdrant"
    fi
    systemctl daemon-reload || true
    systemctl enable gnas.service || true
    systemctl enable qdrant.service || true
    if [ -d /run/systemd/system ]; then
        systemctl restart gnas.service || true
    fi
fi
