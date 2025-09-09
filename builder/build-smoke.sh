#!/usr/bin/env bash
set -euo pipefail

# Builder image smoke test (local to builder dir)
# Usage: ./build-smoke.sh <image_tag> [port]

IMAGE_TAG=${1:-}
PORT=${2:-}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC_FILE="${SCRIPT_DIR}/spec.txt"

usage(){
  echo "Usage: $0 <image_tag> [port]" >&2
  exit 1
}

[[ -z "$IMAGE_TAG" ]] && usage
[[ -f "$SPEC_FILE" ]] || { echo "spec file not found: $SPEC_FILE" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }

if [[ -z "${PORT}" ]]; then
  PORT=$((5535 + RANDOM % 60000))
fi

echo "🧪 Builder image smoke test"
echo "Image: $IMAGE_TAG"
echo "Spec : $SPEC_FILE"
echo "Port : $PORT"

CONTAINER=builder_smoke_instance
cleanup(){
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "1️⃣  Starting container..."
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER" \
  --privileged \
  -e CODERUNR_LOG_LEVEL=debug \
  -e SKIP_CHOWN_PACKAGES=1 \
  -p ${PORT}:2000 \
  "$IMAGE_TAG" >/dev/null

echo -n "⏳ Waiting for API ..."
for i in {1..120}; do
  if curl -fsS "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1 \
     || curl -fsS "http://127.0.0.1:${PORT}/api/v2/runtimes" >/dev/null 2>&1 \
     || docker exec "$CONTAINER" sh -lc 'curl -fsS http://localhost:2000/health >/dev/null 2>&1' >/dev/null 2>&1; then
    echo " ready"
    break
  fi
  echo -n "."
  # Show recent logs every ~15s to aid diagnosis while waiting
  if (( i % 15 == 0 )); then
    echo -e "\n--- recent logs ---"
    docker logs --tail=50 "$CONTAINER" || true
    echo "Health: $(docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$CONTAINER" 2>/dev/null || true)"
    echo -n "⏳ Waiting for API ..."
  fi
  sleep 1
  if [[ $i -eq 120 ]]; then
    echo; echo "❌ API not ready in time" >&2
    echo "Container logs:" >&2
    docker logs "$CONTAINER" || true
    echo "Inspect health:" >&2
    docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$CONTAINER" || true
    exit 1
  fi
done

echo "2️⃣  Checking preinstalled runtimes from spec"
PKG_JSON=$(curl -fsS "http://127.0.0.1:${PORT}/api/v2/packages")

check_installed(){
  local lang="$1" ver="$2"
  echo "$PKG_JSON" | jq -e \
    --arg L "$lang" --arg V "$ver" \
    'map(select(.language==$L and .language_version==$V and .installed==true)) | length > 0' >/dev/null \
    && echo "✅ $lang $ver installed" \
    || { echo "❌ $lang $ver not installed"; exit 1; }
}

while IFS= read -r line; do
  line="${line%%#*}"; line="${line#${line%%[![:space:]]*}}"; line="${line%${line##*[![:space:]]}}"
  [[ -z "$line" ]] && continue
  lang=$(awk '{print $1}' <<<"$line")
  ver=$(awk '{print $2}' <<<"$line")
  [[ -z "$lang" || -z "$ver" ]] && continue
  check_installed "$lang" "$ver"
done < "$SPEC_FILE"

echo "3️⃣  Exercising API endpoints"
curl -fsS "http://127.0.0.1:${PORT}/api/v2/runtimes" | jq '.[0:3]' >/dev/null \
  && echo "✅ /api/v2/runtimes ok" || { echo "❌ /api/v2/runtimes failed"; exit 1; }

echo "4️⃣  Quick code runs (best-effort)"
run_quick(){
  local lang="$1" ver="$2"
  case "$lang" in
    python)
      local out
      out=$(curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v2/execute" \
        -H "Content-Type: application/json" \
        -d "{\"language\":\"python\",\"version\":\"$ver\",\"files\":[{\"content\":\"print(\\\"hello from $lang $ver\\\")\"}]}")
      [[ "$(jq -r '.run.stdout // empty' <<<"$out")" == *"hello from $lang $ver"* ]] \
        && echo "✅ $lang run ok" || echo "ℹ️  $lang run not verified"
      ;;
    go)
      local out
      out=$(curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v2/execute" \
        -H "Content-Type: application/json" \
        -d "{\"language\":\"go\",\"version\":\"$ver\",\"files\":[{\"name\":\"main.go\",\"content\":\"package main\\nimport \\\"fmt\\\"\\nfunc main(){fmt.Println(\\\"hello from go $ver\\\")}\"}]}")
      [[ "$(jq -r '.run.stdout // empty' <<<"$out")" == *"hello from go $ver"* ]] \
        && echo "✅ go run ok" || echo "ℹ️  go run not verified"
      ;;
    java)
      local out
      out=$(curl -fsS -X POST "http://127.0.0.1:${PORT}/api/v2/execute" \
        -H "Content-Type: application/json" \
        -d "{\"language\":\"java\",\"version\":\"$ver\",\"files\":[{\"name\":\"Main.java\",\"content\":\"public class Main{public static void main(String[] a){System.out.println(\\\"hello from java $ver\\\");}}\"}]}")
      [[ "$(jq -r '.run.stdout // empty' <<<"$out")" == *"hello from java $ver"* ]] \
        && echo "✅ java run ok" || echo "ℹ️  java run not verified"
      ;;
    *)
      echo "ℹ️  skip quick run for $lang $ver"
      ;;
  esac
}

while IFS= read -r line; do
  line="${line%%#*}"; line="${line#${line%%[![:space:]]*}}"; line="${line%${line##*[![:space:]]}}"
  [[ -z "$line" ]] && continue
  lang=$(awk '{print $1}' <<<"$line")
  ver=$(awk '{print $2}' <<<"$line")
  [[ -z "$lang" || -z "$ver" ]] && continue
  run_quick "$lang" "$ver"
done < "$SPEC_FILE"

echo "\n✅ Builder image smoke test completed"
