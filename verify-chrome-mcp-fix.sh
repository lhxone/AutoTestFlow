#!/bin/bash
# Chrome MCP 修复验证脚本

set -e

echo "=== Chrome MCP 修复验证脚本 ==="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查函数
check_success() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $1"
        return 0
    else
        echo -e "${RED}✗${NC} $1"
        return 1
    fi
}

check_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

echo "1. 检查Docker Compose配置..."
if [ -f "docker-compose.prod.yml" ]; then
    check_success "docker-compose.prod.yml 文件存在"

    # 检查shm_size配置
    if grep -q "shm_size: '2g'" docker-compose.prod.yml; then
        check_success "共享内存配置为2GB"
    else
        check_warning "共享内存配置可能不是2GB"
    fi

    # 检查tmpfs配置
    if grep -q "tmpfs:" docker-compose.prod.yml; then
        check_success "tmpfs配置已添加"
    else
        check_warning "tmpfs配置可能缺失"
    fi

    # 检查Chrome环境变量
    if grep -q "CHROME_ARGS:" docker-compose.prod.yml; then
        check_success "Chrome环境变量已配置"
    else
        check_warning "Chrome环境变量可能缺失"
    fi

    # 检查端口映射
    if grep -q "9222:9222" docker-compose.prod.yml; then
        check_success "Chrome调试端口已映射"
    else
        check_warning "Chrome调试端口映射可能缺失"
    fi
else
    check_warning "docker-compose.prod.yml 文件不存在"
fi

echo ""
echo "2. 检查Docker Runtime镜像..."
if [ -f "backend/Dockerfile.runtime-base" ]; then
    check_success "Dockerfile.runtime-base 文件存在"

    # 检查Chrome启动参数
    if grep -q "disable-software-rasterizer" backend/Dockerfile.runtime-base; then
        check_success "增强的Chrome启动参数已添加"
    else
        check_warning "增强的Chrome启动参数可能缺失"
    fi

    # 检查健康检查脚本
    if grep -q "chrome-mcp-health-check" backend/Dockerfile.runtime-base; then
        check_success "Chrome健康检查脚本已添加"
    else
        check_warning "Chrome健康检查脚本可能缺失"
    fi
else
    check_warning "Dockerfile.runtime-base 文件不存在"
fi

echo ""
echo "3. 检查MCP运行时代码..."
if [ -f "backend/internal/service/mcp_runtime.go" ]; then
    check_success "mcp_runtime.go 文件存在"

    # 检查重试机制
    if grep -q "maxRetries := 3" backend/internal/service/mcp_runtime.go; then
        check_success "连接重试机制已添加"
    else
        check_warning "连接重试机制可能缺失"
    fi

    # 检查超时配置
    if grep -q "60\*time.Second" backend/internal/service/mcp_runtime.go; then
        check_success "超时时间已增加"
    else
        check_warning "超时时间配置可能不够"
    fi
else
    check_warning "mcp_runtime.go 文件不存在"
fi

echo ""
echo "4. 检查数据库迁移..."
if [ -f "backend/migrations/003_add_chrome_mcp.sql" ]; then
    check_success "Chrome MCP数据库迁移文件存在"

    # 检查MCP Server配置
    if grep -q "chrome-devtools" backend/migrations/003_add_chrome_mcp.sql; then
        check_success "Chrome MCP Server配置已添加"
    else
        check_warning "Chrome MCP Server配置可能缺失"
    fi
else
    check_warning "Chrome MCP数据库迁移文件不存在"
fi

echo ""
echo "5. 检查健康检查脚本..."
if [ -f "backend/scripts/chrome-mcp-health-check.sh" ]; then
    check_success "Chrome健康检查脚本文件存在"
    if [ -x "backend/scripts/chrome-mcp-health-check.sh" ]; then
        check_success "健康检查脚本具有执行权限"
    else
        check_warning "健康检查脚本可能没有执行权限"
    fi
else
    check_warning "Chrome健康检查脚本文件不存在"
fi

echo ""
echo "=== 检查完成 ==="
echo ""
echo "请按照以下步骤部署修复："
echo ""
echo "1. 重新构建并推送backend镜像"
echo "2. 在服务器上应用数据库迁移: mysql -u root -p auto_test_flow < backend/migrations/003_add_chrome_mcp.sql"
echo "3. 重新部署Docker服务: sudo docker-compose -p auto-test-flow --env-file .env.deploy -f docker-compose.prod.yml up -d --remove-orphans"
echo "4. 验证Chrome MCP: sudo docker exec -it atf-backend chrome-mcp-health-check.sh"
echo ""
echo "详细修复说明请查看: MCP_CHROME_FIX.md"
