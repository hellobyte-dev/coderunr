#!/usr/bin/env bash
set -euo pipefail

# Smoke test for a builder-produced CodeRunr image.
# Usage: ./builder-smoke.sh <image_tag> [port]

IMAGE_TAG=${1:-}
PORT=${2:-}

usage() {
  echo "Usage: $0 <image_tag> [port]" >&2
  exit 1
}

[[ -z "$IMAGE_TAG" ]] && usage

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }

if [[ -z "${PORT}" ]]; then
  PORT=$((5535 + RANDOM % 60000))
fi

echo "🧪 Builder image smoke test"
echo "Image: $IMAGE_TAG"
echo "Port:  $PORT"

cleanup() {
  docker rm -f builder_smoke_instance >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "1️⃣  Starting container..."
docker rm -f builder_smoke_instance >/dev/null 2>&1 || true
docker run -d --name builder_smoke_instance \
  --privileged \
  -p ${PORT}:2000 \
  "$IMAGE_TAG" >/dev/null

echo -n "⏳ Waiting for API ..."
for i in {1..60}; do
  if curl -fsS "http://127.0.0.1:${PORT}/api/v2/packages" >/dev/null; then
    echo " ready"
    break
  fi
  echo -n "."
  sleep 1
  if [[ $i -eq 60 ]]; then
    echo ""; echo "❌ API not ready in time" >&2; exit 1
  fi
done

echo "2️⃣  Checking preinstalled runtimes (spec: python 3.12.0, java 15.0.2, go 1.16.2)"
PKG_JSON=$(curl -fsS "http://127.0.0.1:${PORT}/api/v2/packages")

check_installed() {
  local lang="$1"; local ver="$2"
  echo "$PKG_JSON" | jq -e \
    --arg L "$lang" --arg V "$ver" \
    'map(select(.language==$L and .language_version==$V and .installed==true)) | length > 0' >/dev/null \
    && echo "✅ $lang $ver installed" \
    || { echo "❌ $lang $ver not installed"; exit 1; }
}

check_installed python 3.12.0
check_installed java 15.0.2
check_installed go 1.16.2

echo "3️⃣  Exercising API endpoints"
curl -fsS "http://127.0.0.1:${PORT}/api/v2/runtimes" | jq '.[0:3]' >/dev/null \
  && echo "✅ /api/v2/runtimes ok" || { echo "❌ /api/v2/runtimes failed"; exit 1; }

echo "4️⃣  Running a quick Python program"
PY_OUT=$(curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v2/execute" \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python",
    "version": "3.12.0",
    "files": [{"content": "print(\"Hello from builder Python!\")"}]
  }' | jq -r '.run.stdout // empty')

if [[ "$PY_OUT" == *"Hello from builder Python!"* ]]; then
  echo "✅ Python execution ok"
else
  echo "❌ Python execution failed"; exit 1
fi

echo "5️⃣  Optional: try Go and Java quick runs"
set +e
GO_OUT=$(curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v2/execute" \
  -H "Content-Type: application/json" \
  -d '{
    "language": "go",
    "version": "1.16.2",
    "files": [{"name":"main.go","content": "package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"Hello from builder Go!\")}"}]
  }' | jq -r '.run.stdout // empty')
[[ "$GO_OUT" == *"Hello from builder Go!"* ]] && echo "✅ Go execution ok" || echo "ℹ️  Go execution skipped/failed"

JAVA_OUT=$(curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v2/execute" \
  -H "Content-Type: application/json" \
  -d '{
    "language": "java",
    "version": "15.0.2",
    "files": [{"name":"Main.java","content": "public class Main{public static void main(String[] args){System.out.println(\"Hello from builder Java!\");}}"}]
  }' | jq -r '.run.stdout // empty')
[[ "$JAVA_OUT" == *"Hello from builder Java!"* ]] && echo "✅ Java execution ok" || echo "ℹ️  Java execution skipped/failed"
set -e

echo "\n✅ Builder image smoke test completed successfully"