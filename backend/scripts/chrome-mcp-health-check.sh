#!/usr/bin/env bash
# Chrome MCP 健康检查和预启动脚本

set -euo pipefail

CHROME_PATH="${CHROME_PATH:-/usr/local/bin/chrome}"
PROFILE_DIR="${CHROME_DEVTOOLS_MCP_USER_DATA_DIR:-/tmp/auto-test-flow/chrome-profile}"

# 创建必要的目录
mkdir -p "${PROFILE_DIR}"
mkdir -p /tmp/chrome-debug /dev/shm/chrome
chmod 777 /dev/shm/chrome

# 检查Chrome是否可执行
if ! [ -x "${CHROME_PATH}" ]; then
    echo "Error: Chrome executable not found at ${CHROME_PATH}"
    exit 1
fi

# 检查Chrome版本
echo "Checking Chrome version:"
"${CHROME_PATH}" --version || true

# 测试Chrome是否能在容器中启动（headless模式）
echo "Testing Chrome startup..."
timeout 10s "${CHROME_PATH}" \
    --headless \
    --no-sandbox \
    --disable-setuid-sandbox \
    --disable-dev-shm-usage \
    --disable-gpu \
    --disable-software-rasterizer \
    --disable-background-networking \
    --disable-default-apps \
    --disable-extensions \
    --disable-sync \
    --disable-translate \
    --metrics-recording-only \
    --no-first-run \
    --safebrowsing-disable-auto-update \
    --disable-features=site-per-process \
    --remote-debugging-port=9222 \
    --disable-background-timer-throttling \
    --disable-backgrounding-occluded-windows \
    --disable-renderer-backgrounding \
    --disable-features=TranslateUI \
    --remote-debugging-address=0.0.0.0 \
    about:blank &
CHROME_PID=$!

# 等待Chrome启动
sleep 3

# 检查Chrome进程是否仍在运行
if ! kill -0 ${CHROME_PID} 2>/dev/null; then
    echo "Error: Chrome failed to start properly"
    exit 1
fi

echo "Chrome started successfully (PID: ${CHROME_PID})"

# 清理测试进程
kill ${CHROME_PID} 2>/dev/null || true
wait ${CHROME_PID} 2>/dev/null || true

# 检查Chrome DevTools MCP是否可用
if command -v chrome-devtools-mcp >/dev/null 2>&1; then
    echo "Chrome DevTools MCP is available:"
    chrome-devtools-mcp --version
elif [ -x /usr/local/bin/chrome-devtools-mcp-docker ]; then
    echo "Chrome DevTools MCP (Docker) is available"
    /usr/local/bin/chrome-devtools-mcp-docker --help | head -5
else
    echo "Warning: Chrome DevTools MCP not found"
    exit 1
fi

echo "Chrome MCP health check completed successfully"
exit 0