#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
# Wails 3 provides native DEB packaging.
wails3 task linux:create:deb VERSION="${VERSION:-dev}"
