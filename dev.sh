#!/usr/bin/env bash
# dev.sh — Levanta backend (Go) + frontend (Vite) juntos
# Uso: ./dev.sh
# Para detener: Ctrl+C (mata ambos procesos)

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
WEB="$ROOT/web"

# Colores
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RESET='\033[0m'

# Cleanup al salir con Ctrl+C
cleanup() {
  echo -e "\n${YELLOW}[dev] Apagando...${RESET}"
  kill "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null
  wait "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null
  echo -e "${YELLOW}[dev] Listo.${RESET}"
  exit 0
}
trap cleanup INT TERM

echo -e "${CYAN}[dev] Iniciando ReconX...${RESET}"

# ── Limpieza previa ───────────────────────────────────────
echo -e "${YELLOW}[dev] Liberando puertos 8420 y 5173...${RESET}"
lsof -ti :8420 | xargs kill -9 2>/dev/null || true
lsof -ti :5173 | xargs kill -9 2>/dev/null || true
sleep 0.5

# ── Backend ──────────────────────────────────────────────
echo -e "${GREEN}[backend] go run ./cmd/api/${RESET}"
go run "$ROOT/cmd/api/" 2>&1 | sed 's/^/[backend] /' &
BACKEND_PID=$!

# ── Frontend ─────────────────────────────────────────────
echo -e "${GREEN}[frontend] npm run dev${RESET}"
(cd "$WEB" && npm run dev 2>&1 | sed 's/^/[frontend] /') &
FRONTEND_PID=$!

echo -e "${CYAN}[dev] PIDs — backend: $BACKEND_PID | frontend: $FRONTEND_PID${RESET}"
echo -e "${CYAN}[dev] Ctrl+C para detener ambos.${RESET}"

wait
