-- ============================================================
-- 添加 Chrome DevTools MCP Server 配置
-- ============================================================

SET NAMES utf8mb4;
USE auto_test_flow;

-- 插入 Chrome DevTools MCP Server
INSERT INTO `mcp_server` (`name`, `description`, `server_type`, `command`, `args`, `url`, `env_vars`, `status`, `created_at`, `updated_at`)
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
    1,
    NOW(),
    NOW()
) ON DUPLICATE KEY UPDATE 
    `server_type` = 'stdio',
    `command` = 'chrome-devtools-mcp-docker',
    `args` = '[]',
    `description` = 'Chrome DevTools MCP Server - 提供浏览器自动化和网页测试功能',
    `env_vars` = JSON_OBJECT(
        'CHROME_PATH', '/usr/local/bin/chrome',
        'CHROME_DEVTOOLS_MCP_USER_DATA_DIR', '/tmp/auto-test-flow/chrome-profile',
        'CHROME_DISABLE_GPU', 'true',
        'CHROME_NO_SANDBOX', 'true',
        'CHROME_REMOTE_DEBUGGING_PORT', '9222'
    ),
    `status` = 1,
    `updated_at` = NOW();

-- 为默认 Agent 绑定 Chrome DevTools MCP Server
INSERT IGNORE INTO `agent_mcp` (`agent_id`, `mcp_server_id`, `created_at`)
SELECT 
    (SELECT `id` FROM `agent` WHERE `name` = 'default-agent' LIMIT 1),
    (SELECT `id` FROM `mcp_server` WHERE `name` = 'chrome-devtools' LIMIT 1),
    NOW();

-- 验证配置
SELECT 'Chrome DevTools MCP Server 配置完成' as message;
SELECT * FROM `mcp_server` WHERE `name` = 'chrome-devtools';
SELECT * FROM `agent_mcp` WHERE `mcp_server_id` = (SELECT `id` FROM `mcp_server` WHERE `name` = 'chrome-devtools');