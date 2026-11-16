#!/usr/bin/env bash
# Deploy Lab-AP ke VPS (pengganti Vercel).
#
# Target: paluketok VPS (103.119.237.29:20257)
#   /opt/lab-ap/backend      -> Go API, systemd lab-ap-api, :8082
#   /opt/lab-ap/frontend     -> SvelteKit adapter-node, systemd lab-ap-web, :3002
#   nginx vhost: /etc/nginx/sites-available/lab-algohub
#     algohub.web.id     -> :3002
#     api.algohub.web.id -> :8082
#
# Pakai: bash scripts/deploy-vps.sh
# Secret TIDAK dikirim skrip ini. backend/.env dibuat manual sekali di VPS
# (0600, root). Skrip hanya deploy kode + build.

set -euo pipefail

VPS=paluketok
REMOTE=/opt/lab-ap
LOCAL="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

echo "== 1/6 paket source =="
cd "$LOCAL"
tar --exclude='backend/.env' --exclude='backend/bin' \
    --exclude='frontend/node_modules' --exclude='frontend/.svelte-kit' \
    --exclude='frontend/build' --exclude='backups' --exclude='.git' \
    -czf "$STAGE/src.tgz" backend frontend
scp -q "$STAGE/src.tgz" "$VPS:/tmp/lab-ap-src.tgz"

echo "== 2/6 unpack di VPS (env + build lama dipertahankan) =="
ssh "$VPS" "set -e
  mkdir -p $REMOTE
  tar -xzf /tmp/lab-ap-src.tgz -C $REMOTE
  rm -f /tmp/lab-ap-src.tgz
  # artefak Windows sering CRLF; svelte/vite gagal parse string dengan \r
  find $REMOTE/frontend -path '*/node_modules' -prune -o -type f \
    \( -name '*.ts' -o -name '*.js' -o -name '*.svelte' -o -name '*.json' -o -name '*.css' \) \
    -print0 | xargs -0 sed -i 's/\r\$//'
"

echo "== 3/6 build backend =="
ssh "$VPS" "set -e
  export PATH=/usr/lib/go-1.24/bin:\$PATH
  mkdir -p $REMOTE/bin
  cd $REMOTE/backend && go build -o $REMOTE/bin/lab-ap-server ./cmd/server
"

echo "== 4/6 build frontend (adapter-node, node 22) =="
ssh "$VPS" "set -e
  export PATH=/opt/node22/bin:/root/.bun/bin:\$PATH
  cd $REMOTE/frontend
  bun install
  bunx svelte-kit sync
  bun run build
"

echo "== 5/6 restart service ==="
# adapter-node bisa meninggalkan build/ tanpa index.js kalau build terpotong;
# jangan restart ke keadaan yang bikin lab-ap-web crash-loop MODULE_NOT_FOUND.
ssh "$VPS" "test -f $REMOTE/frontend/build/index.js || {
  echo 'build/index.js hilang, rebuild dulu'; cd $REMOTE/frontend && export PATH=/opt/node22/bin:/root/.bun/bin:\$PATH && bun run build; }
  systemctl restart lab-ap-api lab-ap-web && sleep 3 && systemctl is-active lab-ap-api lab-ap-web"

echo "== 6/6 smoke test =="
ssh "$VPS" "curl -sS -m 20 -o /dev/null -w 'api /api/health -> %{http_code}\n' http://127.0.0.1:8082/api/health
curl -sS -m 20 -o /dev/null -w 'web /praktikum/login -> %{http_code}\n' http://127.0.0.1:3002/praktikum/login"

echo "OK. Log: ssh $VPS 'journalctl -u lab-ap-api -u lab-ap-web -f'"
