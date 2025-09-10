#!/usr/bin/env bash

# Build a container using the spec file provided (aligned with piston)

START_DIR=$PWD
cd "$(dirname "${BASH_SOURCE[0]}")"
SCRIPT_DIR=$PWD
CLI_DIR="$SCRIPT_DIR/../cli"
CLI_BIN="$CLI_DIR/coderunr-cli"

help_msg(){
  echo "Usage: $0 [specfile] [tag]"
  echo
  echo "$1"

  exit 1
}

cleanup(){
  echo "Exiting..."
  docker stop builder_coderunr_instance >/dev/null 2>&1 || true
  docker rm builder_coderunr_instance >/dev/null 2>&1 || true
}

fetch_packages(){
  local specfile="$1"
  local port=$((5535 + RANDOM % 60000))
  rm -rf build
  mkdir build
  
  # Check if local repo is running
  if ! curl -fs http://localhost:8000 >/dev/null 2>&1; then
    echo "Local repo not running on port 8000. Starting it..."
    # Start local repo if not running
    (cd .. && docker compose up coderunr-repo -d) || help_msg "failed to start local repo"
    # Wait a moment for repo to be ready
    sleep 3
  fi
  
  # Start a coderunr API container with local repo configuration
  docker run \
    --privileged \
    -v "$PWD/build":'/coderunr/packages' \
    -e CODERUNR_DISABLE_NETWORKING=false \
    -e CODERUNR_REPO_URL=http://coderunr-repo:8000/index \
    --network coderunr_coderunr-network \
    -dit \
    -p $port:2000 \
    --name builder_coderunr_instance \
    "${CODERUNR_BASE_IMAGE:-ghcr.io/hellobyte-dev/coderunr/api:latest}"

  # Ensure the CLI is built
  if [[ ! -x "$CLI_BIN" ]]; then
    command -v go >/dev/null 2>&1 || help_msg "go is required"
    (cd "$CLI_DIR" && go build -o coderunr-cli .) || help_msg "failed to build coderunr CLI"
  fi

  # Wait for API to be ready
  echo "Waiting for API to be ready..."
  for i in {1..30}; do
    if curl -fs "http://127.0.0.1:$port/health" >/dev/null 2>&1; then
      echo "API is ready"
      break
    fi
    sleep 1
  done

  # Evaluate the specfile (no fallback, aligned with piston)
  "$CLI_BIN" -u "http://127.0.0.1:$port" ppman spec "$specfile" || help_msg "ppman spec failed"
}

build_container(){
  docker build -t $1 -f "Dockerfile" "$START_DIR/build"
}


SPEC_FILE=$START_DIR/$1
TAG=$2

[ -z "$1" ] && help_msg "specfile is required"
[ -z "$TAG" ] && help_msg "tag is required"

[ -f "$SPEC_FILE" ] || help_msg "specfile does not exist"

command -v docker >/dev/null 2>&1 || help_msg "docker is required"

trap cleanup EXIT

fetch_packages $SPEC_FILE
build_container $TAG

echo "Start your custom coderunr container with"
echo "$ docker run --privileged -dit -p 2000:2000 $TAG"