#!/usr/bin/env bash
# dev.sh — Levanta backend (Go) + frontend (Vite) + ai-service juntos
# Uso: ./dev.sh
# Para detener: Ctrl+C (mata todos los procesos)

set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
WEB="$ROOT/web"
AI_SERVICE="$ROOT/ai-service"
AI_VENV="$AI_SERVICE/venv"
AI_SERVICE_PORT=8000
BACKEND_PORT=8420
FRONTEND_PORT=5173

# Colores
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RESET='\033[0m'

# Cleanup al salir con Ctrl+C
cleanup() {
  echo -e "\n${YELLOW}[dev] Apagando...${RESET}"
  kill "$BACKEND_PID" "$FRONTEND_PID" "$AI_PID" 2>/dev/null
  wait "$BACKEND_PID" "$FRONTEND_PID" "$AI_PID" 2>/dev/null
  echo -e "${YELLOW}[dev] Listo.${RESET}"
  exit 0
}
trap cleanup INT TERM

echo -e "${CYAN}[dev] Iniciando ReconX...${RESET}"

# ── Bootstrap de dependencias ────────────────────────────
if [ ! -d "$WEB/node_modules" ]; then
  echo -e "${YELLOW}[frontend] node_modules no existe; ejecutando npm install...${RESET}"
  if ! (cd "$WEB" && npm install); then
    echo -e "${YELLOW}[frontend] npm install falló por dependencias; reintentando con --legacy-peer-deps...${RESET}"
    (cd "$WEB" && npm install --legacy-peer-deps)
  fi
fi

if [ -d "$AI_SERVICE" ]; then
  if [ ! -x "$AI_VENV/bin/python" ]; then
    echo -e "${YELLOW}[ai-service] venv no existe; creando entorno virtual...${RESET}"
    (cd "$AI_SERVICE" && python3 -m venv venv)
  fi

  if [ ! -x "$AI_VENV/bin/uvicorn" ]; then
    echo -e "${YELLOW}[ai-service] dependencias faltantes; instalando requirements...${RESET}"
    "$AI_VENV/bin/pip" install -r "$AI_SERVICE/requirements.txt"
  fi
fi

# ── Limpieza previa ───────────────────────────────────────
echo -e "${YELLOW}[dev] Liberando puertos ${AI_SERVICE_PORT}, ${BACKEND_PORT} y ${FRONTEND_PORT}...${RESET}"
lsof -ti :"$AI_SERVICE_PORT" | xargs kill -9 2>/dev/null || true
lsof -ti :"$BACKEND_PORT" | xargs kill -9 2>/dev/null || true
lsof -ti :"$FRONTEND_PORT" | xargs kill -9 2>/dev/null || true
sleep 0.5

# ── AI Service ───────────────────────────────────────────
if [ -d "$AI_SERVICE" ]; then
  echo -e "${GREEN}[ai-service] uvicorn main:app --reload --host 0.0.0.0 --port ${AI_SERVICE_PORT}${RESET}"
  (
    cd "$AI_SERVICE"
    if [ -x "$AI_VENV/bin/uvicorn" ]; then
      "$AI_VENV/bin/uvicorn" main:app --reload --host 0.0.0.0 --port "$AI_SERVICE_PORT" 2>&1 | sed 's/^/[ai-service] /'
    else
      python3 -m uvicorn main:app --reload --host 0.0.0.0 --port "$AI_SERVICE_PORT" 2>&1 | sed 's/^/[ai-service] /'
    fi
  ) &
  AI_PID=$!
else
  echo -e "${YELLOW}[ai-service] Carpeta ai-service no encontrada; se omite.${RESET}"
  AI_PID=""
fi

# ── Backend ──────────────────────────────────────────────
echo -e "${GREEN}[backend] go run ./cmd/api/${RESET}"
go run "$ROOT/cmd/api/" 2>&1 | sed 's/^/[backend] /' &
BACKEND_PID=$!

# ── Frontend ─────────────────────────────────────────────
echo -e "${GREEN}[frontend] npm run dev${RESET}"
(cd "$WEB" && npm run dev 2>&1 | sed 's/^/[frontend] /') &
FRONTEND_PID=$!

echo -e "${CYAN}[dev] PIDs — ai-service: ${AI_PID:-n/a} | backend: $BACKEND_PID | frontend: $FRONTEND_PID${RESET}"
echo -e "${CYAN}[dev] Ctrl+C para detener ambos.${RESET}"

wait
