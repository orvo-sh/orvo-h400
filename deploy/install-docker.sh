#!/usr/bin/env bash
set -euo pipefail

if command -v docker >/dev/null 2>&1; then
  exit 0
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker

