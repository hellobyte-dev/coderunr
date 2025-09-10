#!/bin/bash
# 测试容器启动性能的脚本

set -e

IMAGE_TAG="coderunr-test:startup-fix"
CONTAINER_NAME="coderunr-startup-test"

echo "🔨 构建测试镜像..."
cd builder
./build.sh spec.txt "$IMAGE_TAG"

echo ""
echo "📊 性能测试开始..."

cleanup() {
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}

test_startup() {
    local env_var="$1"
    local test_name="$2"
    
    cleanup
    
    echo "⏱️  测试 $test_name..."
    
    start_time=$(date +%s)
    
    if [ -n "$env_var" ]; then
        docker run -d --name "$CONTAINER_NAME" --privileged \
            -e "$env_var" \
            -p 2000:2000 \
            "$IMAGE_TAG" >/dev/null
    else
        docker run -d --name "$CONTAINER_NAME" --privileged \
            -p 2000:2000 \
            "$IMAGE_TAG" >/dev/null
    fi
    
    # 等待API可用
    echo -n "   等待API启动"
    for i in {1..180}; do
        if curl -fs http://localhost:2000/health >/dev/null 2>&1; then
            end_time=$(date +%s)
            startup_time=$((end_time - start_time))
            echo ""
            echo "   ✅ $test_name 启动时间: ${startup_time}秒"
            cleanup
            return
        fi
        echo -n "."
        sleep 1
    done
    
    echo ""
    echo "   ❌ $test_name 启动超时"
    cleanup
}

trap cleanup EXIT

# 测试标准启动（完整chown）
test_startup "" "标准启动 (完整权限检查)"

# 测试快速启动（跳过chown）
test_startup "SKIP_CHOWN_PACKAGES=true" "快速启动 (跳过包权限检查)"

echo ""
echo "🎉 测试完成！"
