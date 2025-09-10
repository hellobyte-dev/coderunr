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
  echo "Environment variables:"
  echo "  USE_LOCAL_REPO=true|false    Use local repo (default: true)"
  echo "  CODERUNR_REPO_URL=<url>       Remote repo URL (when USE_LOCAL_REPO=false)"
  echo "  CODERUNR_BASE_IMAGE=<image>   Base API image (default: ghcr.io/hellobyte-dev/coderunr/api:latest)"
  echo
  echo "Examples:"
  echo "  $0 spec.txt my-image:latest                    # Use local repo"
  echo "  USE_LOCAL_REPO=false $0 spec.txt my-image:v1   # Use remote repo"
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
  
  # Check if we should use local repo (default) or allow remote repo
  USE_LOCAL_REPO="${USE_LOCAL_REPO:-true}"
  
  if [ "$USE_LOCAL_REPO" = "true" ]; then
    # Check if local repo is running
    if ! curl -fs http://localhost:8000 >/dev/null 2>&1; then
      echo "Local repo not running on port 8000. Starting it..."
      # Start local repo if not running
      (cd .. && docker compose up coderunr-repo -d) || help_msg "failed to start local repo"
      # Wait a moment for repo to be ready
      sleep 3
    fi
    
    REPO_URL="http://coderunr-repo:8000/index"
    NETWORK_ARG="--network coderunr_coderunr-network"
    echo "Using local repository: $REPO_URL"
  else
    # Use remote repo (default GitHub releases)
    REPO_URL="${CODERUNR_REPO_URL:-https://github.com/hellobyte-dev/coderunr/releases/download/packages/index}"
    NETWORK_ARG=""
    echo "Using remote repository: $REPO_URL"
    echo "Warning: Remote downloads may take longer, consider setting USE_LOCAL_REPO=true for faster builds"
  fi
  
  # Start a coderunr API container
  docker run \
    --privileged \
    -v "$PWD/build":'/coderunr/packages' \
    -e CODERUNR_DISABLE_NETWORKING=false \
    -e CODERUNR_REPO_URL="$REPO_URL" \
    $NETWORK_ARG \
    -dit \
    -p $port:2000 \
    --name builder_coderunr_instance \
    "${CODERUNR_BASE_IMAGE:-ghcr.io/hellobyte-dev/coderunr/api:latest}"

  # Ensure the CLI is built
  if [[ ! -x "$CLI_BIN" ]]; then
    command -v go >/dev/null 2>&1 || help_msg "go is required"
    (cd "$CLI_DIR" && go build -o coderunr-cli .) || help_msg "failed to build coderunr CLI"
  fi

  # Wait for API to be ready with different timeouts based on repo type
  if [ "$USE_LOCAL_REPO" = "true" ]; then
    API_READY_TIMEOUT=30
    echo "Waiting for API to be ready (local repo, ${API_READY_TIMEOUT}s timeout)..."
  else
    API_READY_TIMEOUT=60
    echo "Waiting for API to be ready (remote repo, ${API_READY_TIMEOUT}s timeout)..."
  fi
  
  for i in $(seq 1 $API_READY_TIMEOUT); do
    if curl -fs "http://127.0.0.1:$port/health" >/dev/null 2>&1; then
      echo "API is ready"
      break
    fi
    if [ $i -eq $API_READY_TIMEOUT ]; then
      echo "API readiness timeout after ${API_READY_TIMEOUT}s"
      docker logs builder_coderunr_instance
      help_msg "API failed to start"
    fi
    sleep 1
  done

  # Set package installation timeout based on repo type
  if [ "$USE_LOCAL_REPO" = "true" ]; then
    PACKAGE_TIMEOUT="5m"  # 5 minutes for local repo
  else
    PACKAGE_TIMEOUT="15m" # 15 minutes for remote repo
  fi
  
  echo "Installing packages with ${PACKAGE_TIMEOUT} timeout..."
  
  # Use timeout command to limit package installation time
  if command -v timeout >/dev/null 2>&1; then
    timeout "$PACKAGE_TIMEOUT" "$CLI_BIN" -u "http://127.0.0.1:$port" ppman spec "$specfile" || {
      echo "Package installation failed or timed out after $PACKAGE_TIMEOUT"
      echo "Container logs:"
      docker logs builder_coderunr_instance
      help_msg "ppman spec failed"
    }
  else
    # Fallback without timeout if not available
    "$CLI_BIN" -u "http://127.0.0.1:$port" ppman spec "$specfile" || help_msg "ppman spec failed"
  fi
  
  echo "Package installation completed successfully"
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