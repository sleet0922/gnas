#!/bin/sh
set -e

if [ "$1" = "remove" ] && command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now gnas.service || true
    systemctl disable --now qdrant.service || true
fi
