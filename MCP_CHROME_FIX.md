# MCP Chrome DevTools 错误修复说明

## 问题描述

在CI容器化部署后，点击生成测试时调用MCP工具出现以下错误：

```
mcp.chrome-devtools.list_pages
Protocol error (Target.setDiscoverTargets): Target closed
```

## 根本原因

1. **容器环境限制**：Chrome在容器环境中需要特殊配置才能正常运行
2. **资源不足**：共享内存(shm)不足导致Chrome进程无法启动
3. **Chrome参数不完整**：缺少容器环境所需的启动参数
4. **连接超时**：MCP连接Chrome的超时时间设置过短
5. **无重试机制**：Chrome启动失败时没有重试机制

## 修复方案

### 1. Docker Compose 配置优化 (docker-compose.prod.yml)

#### 增加容器资源限制
- 将`shm_size`从512m增加到2g
- 添加`tmpfs`挂载：`/tmp`和`/dev/shm`各2g
- 配置CPU和内存限制：2核4GB（上限），1核2GB（保留）

#### 新增Chrome环境变量
```yaml
environment:
  CHROME_ARGS: --no-sandbox --disable-setuid-sandbox --disable-dev-shm-usage --disable-gpu --disable-software-rasterizer --disable-background-networking --disable-default-apps --disable-extensions --disable-sync --disable-translate --metrics-recording-only --no-first-run --safebrowsing-disable-auto-update --disable-features=site-per-process --disable-background-timer-throttling --disable-backgrounding-occluded-windows --disable-renderer-backgrounding
  CHROME_USER_DATA_DIR: /tmp/auto-test-flow/chrome-profile
  CHROME_DISABLE_GPU: "true"
  CHROME_NO_SANDBOX: "true"
  CHROME_REMOTE_DEBUGGING_PORT: "9222"
  CHROME_PATH: /usr/local/bin/chrome
  CHROME_DEVTOOLS_MCP_USER_DATA_DIR: /tmp/auto-test-flow/chrome-profile
```

#### 端口映射
- 新增9222端口映射用于Chrome远程调试

#### 自定义启动命令
- 添加Chrome MCP健康检查
- 在启动服务前验证Chrome环境

### 2. Docker Runtime 镜像优化 (Dockerfile.runtime-base)

#### 增强Chrome启动参数
在`chrome-devtools-mcp-docker`脚本中添加更多容器环境参数：
- `--disable-software-rasterizer`：禁用软件光栅化
- `--disable-background-networking`：禁用后台网络
- `--disable-default-apps`：禁用默认应用
- `--disable-extensions`：禁用扩展
- `--disable-sync`：禁用同步
- `--disable-translate`：禁用翻译
- `--metrics-recording-only`：仅记录指标
- `--no-first-run`：跳过首次运行设置
- `--safebrowsing-disable-auto-update`：禁用安全浏览自动更新
- `--disable-features=site-per-process`：禁用站点隔离
- `--remote-debugging-port=9222`：远程调试端口
- `--disable-background-timer-throttling`：禁用后台定时器节流
- `--disable-backgrounding-occluded-windows`：禁用遮挡窗口后台运行
- `--disable-renderer-backgrounding`：禁用渲染器后台运行

#### 添加健康检查脚本
- 创建`chrome-mcp-health-check.sh`脚本
- 验证Chrome可执行文件
- 测试Chrome启动能力
- 检查MCP工具可用性

### 3. MCP 运行时代码优化 (mcp_runtime.go)

#### 增强连接重试机制
- 连接超时从20秒增加到60秒
- 添加3次重试机制
- 重试间隔递增（5秒、10秒、15秒）
- 详细的错误日志记录

#### 增强工具调用重试机制
- 工具调用超时从30秒增加到60秒
- Chrome MCP工具使用3次重试，其他工具2次
- 重试间隔递增（3秒、6秒、9秒）
- 更好的错误处理和日志记录

### 4. 数据库配置 (migrations/003_add_chrome_mcp.sql)

#### 添加Chrome MCP Server配置
```sql
INSERT INTO `mcp_server` (`name`, `description`, `server_type`, `command`, `args`, `url`, `env_vars`, `status`)
VALUES (
    'chrome-devtools',
    'Chrome DevTools MCP Server - 提供浏览器自动化和网页测试功能',
    'stdio',
    'chrome-devtools-mcp-docker',
    '[]',
    '',
    JSON_OBJECT(
        'CHROME_PATH', '/usr/local/bin/chrome',
        'CHROME_DEVTOOLS_MCP_USER_DATA_DIR', '/tmp/auto-test-flow/chrome-profile',
        'CHROME_DISABLE_GPU', 'true',
        'CHROME_NO_SANDBOX', 'true',
        'CHROME_REMOTE_DEBUGGING_PORT', '9222'
    ),
    1
);
```

#### 为默认Agent绑定Chrome MCP Server
```sql
INSERT IGNORE INTO `agent_mcp` (`agent_id`, `mcp_server_id`)
SELECT 
    (SELECT `id` FROM `agent` WHERE `name` = 'default-agent' LIMIT 1),
    (SELECT `id` FROM `mcp_server` WHERE `name` = 'chrome-devtools' LIMIT 1);
```

## 部署步骤

### 1. 重新构建和推送镜像
```bash
# 构建backend镜像
docker build -t harbor.inspur.local/atf/auto-test-flow-backend:${COMMIT_SHA} backend
docker push harbor.inspur.local/atf/auto-test-flow-backend:${COMMIT_SHA}
```

### 2. 应用数据库迁移
```bash
# 在服务器上执行
mysql -h 192.168.53.185 -u root -p auto_test_flow < /opt/AutoTestFlow/app/backend/migrations/003_add_chrome_mcp.sql
```

### 3. 重新部署服务
```bash
# 使用CI/CD自动部署，或手动执行
cd /opt/AutoTestFlow/app
sudo docker-compose -p auto-test-flow --env-file .env.deploy -f docker-compose.prod.yml pull
sudo docker-compose -p auto-test-flow --env-file .env.deploy -f docker-compose.prod.yml up -d --remove-orphans
```

### 4. 验证修复
```bash
# 检查服务状态
sudo docker-compose -p auto-test-flow ps

# 查看backend日志
sudo docker-compose -p auto-test-flow logs -f backend

# 检查Chrome进程
sudo docker exec -it atf-backend chrome --version
sudo docker exec -it atf-backend chrome-devtools-mcp-docker --version
```

## 验证步骤

### 1. 检查Chrome MCP健康状态
```bash
sudo docker exec -it atf-backend chrome-mcp-health-check.sh
```

### 2. 测试Chrome启动
```bash
sudo docker exec -it atf-backend chrome \
  --headless \
  --no-sandbox \
  --disable-setuid-sandbox \
  --disable-dev-shm-usage \
  --disable-gpu \
  about:blank
```

### 3. 在AutoTestFlow界面中
1. 访问 http://192.168.53.185
2. 进入Agent管理页面
3. 确认Chrome MCP Server已配置
4. 创建测试任务
5. 检查是否还有"Target closed"错误

## 预期效果

修复后，Chrome MCP应该能够：
1. 在容器环境中正常启动
2. 成响应MCP工具调用
3. 提供网页测试和自动化功能
4. 连接失败时有重试机制
5. 详细的错误日志便于排查问题

## 故障排查

如果问题仍然存在，请检查：

1. **容器资源**：确认服务器有足够资源（建议4核8GB以上）
2. **Chrome日志**：查看详细的Chrome启动日志
3. **网络连接**：确认容器网络配置正确
4. **权限问题**：确认Docker socket和文件系统权限
5. **代理配置**：检查Proxifier等代理软件是否影响连接

## 相关文件

- `docker-compose.prod.yml` - Docker Compose生产环境配置
- `backend/Dockerfile.runtime-base` - 运行时基础镜像
- `backend/internal/service/mcp_runtime.go` - MCP运行时实现
- `backend/migrations/003_add_chrome_mcp.sql` - Chrome MCP数据库配置
- `backend/scripts/chrome-mcp-health-check.sh` - Chrome健康检查脚本