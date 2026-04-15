-- ============================================================
-- AutoTestFlow 数据库初始化脚本
-- 数据库: MySQL 8.0+
-- 字符集: utf8mb4
-- ============================================================

SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

CREATE DATABASE IF NOT EXISTS auto_test_flow
    DEFAULT CHARACTER SET utf8mb4
    DEFAULT COLLATE utf8mb4_unicode_ci;

USE auto_test_flow;

-- ============================================================
-- 1. 用户与权限模块
-- ============================================================

-- 用户表
CREATE TABLE `user` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户ID',
    `username`      VARCHAR(64)     NOT NULL COMMENT '用户名(登录名)',
    `password_hash` VARCHAR(255)    NOT NULL COMMENT '密码哈希(bcrypt)',
    `real_name`     VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '真实姓名',
    `email`         VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '邮箱',
    `phone`         VARCHAR(20)     NOT NULL DEFAULT '' COMMENT '手机号',
    `avatar`        VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '头像URL',
    `status`        TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 0=禁用',
    `last_login_at` DATETIME        NULL COMMENT '最后登录时间',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at`    DATETIME        NULL COMMENT '软删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_username` (`username`),
    KEY `idx_email` (`email`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

-- 角色表
CREATE TABLE `role` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色ID',
    `code`        VARCHAR(32)     NOT NULL COMMENT '角色编码: admin/test_lead/tester/dev_lead/viewer',
    `name`        VARCHAR(64)     NOT NULL COMMENT '角色名称',
    `description` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '角色描述',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色表';

-- 权限表
CREATE TABLE `permission` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '权限ID',
    `code`        VARCHAR(64)     NOT NULL COMMENT '权限编码: project:create, review:approve 等',
    `name`        VARCHAR(64)     NOT NULL COMMENT '权限名称',
    `module`      VARCHAR(32)     NOT NULL COMMENT '所属模块: project/user/issue/agent/review/test/report',
    `description` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '权限描述',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_code` (`code`),
    KEY `idx_module` (`module`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='权限表';

-- 角色-权限关联表
CREATE TABLE `role_permission` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `role_id`       BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
    `permission_id` BIGINT UNSIGNED NOT NULL COMMENT '权限ID',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_role_perm` (`role_id`, `permission_id`),
    KEY `idx_permission_id` (`permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色权限关联表';

-- 用户-角色关联表
CREATE TABLE `user_role` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `role_id`    BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user_role` (`user_id`, `role_id`),
    KEY `idx_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户角色关联表';

-- ============================================================
-- 2. 项目管理模块
-- ============================================================

CREATE TABLE `project` (
    `id`                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '项目ID',
    `name`                 VARCHAR(128)    NOT NULL COMMENT '项目名称',
    `description`          TEXT            NULL COMMENT '项目描述',
    `func_doc_path`        VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '功能文档MD路径',
    `design_doc_path`      VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '设计文档MD路径',
    `db_doc_path`          VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '数据库文档MD路径',
    `test_doc_path`        VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '测试文档MD路径',
    `extra_files_path`     VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '其他文件路径(JSON数组)',
    `git_repo_url`         VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '测试Git仓库地址',
    `git_branch`           VARCHAR(128)    NOT NULL DEFAULT 'main' COMMENT '测试Git主分支',
    `zentao_project_id`    INT             NULL COMMENT '禅道项目集ID',
    `zentao_project_name`  VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '禅道项目集名称',
    `zentao_branch`        VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '禅道分支',
    `status`               TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 0=禁用',
    `owner_id`             BIGINT UNSIGNED NULL COMMENT '项目负责人ID',
    `created_at`           DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`           DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at`           DATETIME        NULL,
    PRIMARY KEY (`id`),
    KEY `idx_name` (`name`),
    KEY `idx_zentao_project` (`zentao_project_id`),
    KEY `idx_owner` (`owner_id`),
    KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目表';

-- 项目成员关联表
CREATE TABLE `project_member` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `user_id`    BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    `role`       VARCHAR(32)     NOT NULL DEFAULT 'member' COMMENT '项目角色: owner/test_lead/tester/dev_lead/member',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_project_user` (`project_id`, `user_id`),
    KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目成员表';

-- ============================================================
-- 3. 禅道问题单模块
-- ============================================================

CREATE TABLE `issue` (
    `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '本地ID',
    `zentao_id`        INT             NOT NULL COMMENT '禅道问题单ID',
    `project_id`       BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `title`            VARCHAR(512)    NOT NULL COMMENT '问题标题',
    `description`      TEXT            NULL COMMENT '问题描述',
    `issue_type`       VARCHAR(32)     NOT NULL DEFAULT 'bug' COMMENT '类型: bug/story/task',
    `zentao_status`    VARCHAR(32)     NOT NULL DEFAULT '' COMMENT '禅道状态(原始值)',
    `test_status`      VARCHAR(32)     NOT NULL DEFAULT 'pending' COMMENT '测试状态',
    `severity`         VARCHAR(16)     NOT NULL DEFAULT 'normal' COMMENT '严重程度: critical/major/normal/minor',
    `priority`         TINYINT         NOT NULL DEFAULT 3 COMMENT '优先级 1-5',
    `reporter`         VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '提出人(禅道用户名)',
    `reporter_email`   VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '提出人邮箱',
    `assignee`         VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '负责人(禅道用户名)',
    `assignee_email`   VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '负责人邮箱',
    `branch`           VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '所属分支',
    `resolved_at`      DATETIME        NULL COMMENT '解决时间',
    `zentao_updated_at` DATETIME       NULL COMMENT '禅道更新时间',
    `synced_at`        DATETIME        NULL COMMENT '最后同步时间',
    `created_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_zentao_project` (`zentao_id`, `project_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_zentao_status` (`zentao_status`),
    KEY `idx_test_status` (`test_status`),
    KEY `idx_assignee` (`assignee`),
    KEY `idx_resolved_at` (`resolved_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='问题单表';

-- 问题单状态变更日志
CREATE TABLE `issue_status_log` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `issue_id`    BIGINT UNSIGNED NOT NULL COMMENT '问题单ID',
    `field`       VARCHAR(32)     NOT NULL COMMENT '变更字段: zentao_status/test_status',
    `old_value`   VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '旧值',
    `new_value`   VARCHAR(64)     NOT NULL COMMENT '新值',
    `trigger_type` VARCHAR(16)    NOT NULL DEFAULT 'system' COMMENT '触发类型: system/manual',
    `operator_id` BIGINT UNSIGNED NULL COMMENT '操作人ID(manual时)',
    `remark`      VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '备注',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='问题单状态变更日志';

-- ============================================================
-- 4. Agent 管理模块
-- ============================================================

-- AI Agent 表
CREATE TABLE `agent` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Agent ID',
    `name`        VARCHAR(64)     NOT NULL COMMENT 'Agent 名称',
    `description` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '描述',
    `is_default`  TINYINT(1)      NOT NULL DEFAULT 0 COMMENT '是否默认 Agent: 1=是 0=否',
    `model_provider` VARCHAR(32)  NOT NULL DEFAULT 'claude' COMMENT '模型提供商: claude/openai/custom',
    `model_name`  VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '模型名称: claude-sonnet-4-20250514 等',
    `api_key_ref` VARCHAR(128)    NOT NULL DEFAULT '' COMMENT 'API Key引用(不存明文, 引用配置名)',
    `base_url`    VARCHAR(256)    NOT NULL DEFAULT '' COMMENT 'API Base URL',
    `max_tokens`  INT             NOT NULL DEFAULT 4096 COMMENT '最大Token数',
    `temperature` DECIMAL(3,2)    NOT NULL DEFAULT 0.30 COMMENT '温度参数',
    `status`      TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 0=停用',
    `config_json` JSON            NULL COMMENT '额外配置(JSON)',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI Agent表';

-- Skill 表
CREATE TABLE `skill` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Skill ID',
    `name`        VARCHAR(64)     NOT NULL COMMENT 'Skill 名称: gen-test 等',
    `description` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '描述',
    `skill_type`  VARCHAR(32)     NOT NULL DEFAULT 'builtin' COMMENT '类型: builtin/custom',
    `prompt_template` TEXT        NULL COMMENT 'Prompt 模板',
    `input_schema`  JSON         NULL COMMENT '输入参数 Schema(JSON)',
    `output_schema` JSON         NULL COMMENT '输出参数 Schema(JSON)',
    `config_json`   JSON         NULL COMMENT '额外配置(JSON)',
    `status`      TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 0=停用',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Skill表';

-- MCP 配置表
CREATE TABLE `mcp_server` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'MCP Server ID',
    `name`        VARCHAR(64)     NOT NULL COMMENT 'MCP Server 名称',
    `description` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '描述',
    `server_type` VARCHAR(32)     NOT NULL DEFAULT 'stdio' COMMENT '类型: stdio/sse/streamable_http',
    `command`     VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '启动命令(stdio模式)',
    `args`        JSON            NULL COMMENT '启动参数(JSON数组)',
    `url`         VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '服务地址(http模式)',
    `env_vars`    JSON            NULL COMMENT '环境变量(JSON)',
    `status`      TINYINT         NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 0=停用',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP Server配置表';

-- Agent-Skill 绑定表
CREATE TABLE `agent_skill` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `agent_id`   BIGINT UNSIGNED NOT NULL COMMENT 'Agent ID',
    `skill_id`   BIGINT UNSIGNED NOT NULL COMMENT 'Skill ID',
    `priority`   INT             NOT NULL DEFAULT 0 COMMENT '优先级',
    `config_override` JSON       NULL COMMENT '覆盖配置',
    `created_at` DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_agent_skill` (`agent_id`, `skill_id`),
    KEY `idx_skill_id` (`skill_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent-Skill绑定表';

-- Agent-MCP 绑定表
CREATE TABLE `agent_mcp` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `agent_id`    BIGINT UNSIGNED NOT NULL COMMENT 'Agent ID',
    `mcp_server_id` BIGINT UNSIGNED NOT NULL COMMENT 'MCP Server ID',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_agent_mcp` (`agent_id`, `mcp_server_id`),
    KEY `idx_mcp_server_id` (`mcp_server_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent-MCP绑定表';

-- ============================================================
-- 5. 测试任务模块
-- ============================================================

-- 测试任务表(一个问题单一次AI生成 = 一个任务)
CREATE TABLE `test_task` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '任务ID',
    `issue_id`      BIGINT UNSIGNED NOT NULL COMMENT '关联问题单ID',
    `project_id`    BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `agent_id`      BIGINT UNSIGNED NULL COMMENT '使用的Agent ID',
    `skill_name`    VARCHAR(64)     NOT NULL DEFAULT 'gen-test' COMMENT '使用的Skill',
    `status`        VARCHAR(32)     NOT NULL DEFAULT 'pending' COMMENT '任务状态',
    `ai_input`      JSON            NULL COMMENT 'AI输入上下文(JSON)',
    `ai_output`     JSON            NULL COMMENT 'AI输出结果(JSON)',
    `error_message` TEXT            NULL COMMENT '错误信息',
    `retry_count`   INT             NOT NULL DEFAULT 0 COMMENT '重试次数',
    `started_at`    DATETIME        NULL COMMENT '开始时间',
    `completed_at`  DATETIME        NULL COMMENT '完成时间',
    `created_by`    BIGINT UNSIGNED NULL COMMENT '创建人ID',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_status` (`status`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试任务表';

-- 测试用例表
CREATE TABLE `test_case` (
    `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用例ID',
    `task_id`      BIGINT UNSIGNED NOT NULL COMMENT '所属任务ID',
    `issue_id`     BIGINT UNSIGNED NOT NULL COMMENT '关联问题单ID',
    `project_id`   BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `title`        VARCHAR(256)    NOT NULL COMMENT '用例标题',
    `category`     VARCHAR(32)     NOT NULL DEFAULT 'normal' COMMENT '分类: main_flow/exception/boundary/regression',
    `precondition` TEXT            NULL COMMENT '前置条件',
    `steps`        TEXT            NULL COMMENT '测试步骤(JSON或MD)',
    `expected`     TEXT            NULL COMMENT '预期结果',
    `actual`       TEXT            NULL COMMENT '实际结果(自测)',
    `self_test_result` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '自测结果: pass/fail/skip/pending',
    `priority`     TINYINT         NOT NULL DEFAULT 2 COMMENT '优先级 1-3',
    `current_version` INT          NOT NULL DEFAULT 1 COMMENT '当前版本号',
    `source`       VARCHAR(16)     NOT NULL DEFAULT 'ai' COMMENT '来源: ai/manual',
    `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_task_id` (`task_id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_category` (`category`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试用例表';

-- 测试用例版本历史表
CREATE TABLE `test_case_version` (
    `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `test_case_id` BIGINT UNSIGNED NOT NULL COMMENT '测试用例ID',
    `version`      INT             NOT NULL COMMENT '版本号',
    `title`        VARCHAR(256)    NOT NULL COMMENT '用例标题',
    `precondition` TEXT            NULL,
    `steps`        TEXT            NULL,
    `expected`     TEXT            NULL,
    `source`       VARCHAR(16)     NOT NULL DEFAULT 'ai' COMMENT '来源: ai/manual',
    `change_note`  VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '变更说明',
    `changed_by`   BIGINT UNSIGNED NULL COMMENT '修改人ID(manual)',
    `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_case_version` (`test_case_id`, `version`),
    KEY `idx_changed_by` (`changed_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试用例版本历史';

-- 测试脚本表
CREATE TABLE `test_script` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '脚本ID',
    `task_id`        BIGINT UNSIGNED NOT NULL COMMENT '所属任务ID',
    `issue_id`       BIGINT UNSIGNED NOT NULL COMMENT '关联问题单ID',
    `project_id`     BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `file_path`      VARCHAR(512)    NOT NULL COMMENT '脚本文件路径(相对于仓库根)',
    `file_content`   LONGTEXT        NULL COMMENT '脚本内容',
    `language`       VARCHAR(16)     NOT NULL DEFAULT 'python' COMMENT '脚本语言',
    `current_version` INT            NOT NULL DEFAULT 1 COMMENT '当前版本号',
    `source`         VARCHAR(16)     NOT NULL DEFAULT 'ai' COMMENT '来源: ai/manual',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_task_id` (`task_id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试脚本表';

-- 测试脚本版本历史表
CREATE TABLE `test_script_version` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `test_script_id`  BIGINT UNSIGNED NOT NULL COMMENT '测试脚本ID',
    `version`         INT             NOT NULL COMMENT '版本号',
    `file_content`    LONGTEXT        NULL COMMENT '脚本内容',
    `source`          VARCHAR(16)     NOT NULL DEFAULT 'ai' COMMENT '来源: ai/manual',
    `change_note`     VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '变更说明',
    `changed_by`      BIGINT UNSIGNED NULL COMMENT '修改人ID',
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_script_version` (`test_script_id`, `version`),
    KEY `idx_changed_by` (`changed_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试脚本版本历史';

-- 测试文档表
CREATE TABLE `test_document` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '文档ID',
    `task_id`        BIGINT UNSIGNED NOT NULL COMMENT '所属任务ID',
    `issue_id`       BIGINT UNSIGNED NOT NULL COMMENT '关联问题单ID',
    `project_id`     BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `title`          VARCHAR(256)    NOT NULL COMMENT '文档标题',
    `file_path`      VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '文档文件路径',
    `content`        LONGTEXT        NULL COMMENT '文档内容(MD)',
    `doc_type`       VARCHAR(32)     NOT NULL DEFAULT 'test_report' COMMENT '类型: test_case_doc/test_report/test_summary',
    `current_version` INT            NOT NULL DEFAULT 1 COMMENT '当前版本号',
    `source`         VARCHAR(16)     NOT NULL DEFAULT 'ai' COMMENT '来源: ai/manual',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_task_id` (`task_id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试文档表';

-- ============================================================
-- 6. Review 审核模块
-- ============================================================

-- Review 任务表(一次AI生成 -> 一个Review任务)
CREATE TABLE `review_task` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'Review ID',
    `test_task_id`  BIGINT UNSIGNED NOT NULL COMMENT '关联测试任务ID',
    `issue_id`      BIGINT UNSIGNED NOT NULL COMMENT '关联问题单ID',
    `project_id`    BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `title`         VARCHAR(256)    NOT NULL COMMENT 'Review标题',
    `status`        VARCHAR(32)     NOT NULL DEFAULT 'pending' COMMENT '状态: pending/approved/rejected/changes_requested',
    `reviewer_id`   BIGINT UNSIGNED NULL COMMENT '审核人ID',
    `submitted_by`  BIGINT UNSIGNED NULL COMMENT '提交人ID(系统自动或人工)',
    `review_note`   TEXT            NULL COMMENT '审核意见',
    `reviewed_at`   DATETIME        NULL COMMENT '审核时间',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_test_task_id` (`test_task_id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_status` (`status`),
    KEY `idx_reviewer_id` (`reviewer_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Review审核任务表';

-- Review 记录表(每次审核操作留一条记录)
CREATE TABLE `review_record` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `review_task_id` BIGINT UNSIGNED NOT NULL COMMENT 'Review任务ID',
    `reviewer_id`   BIGINT UNSIGNED NOT NULL COMMENT '审核人ID',
    `action`        VARCHAR(32)     NOT NULL COMMENT '操作: approve/reject/request_changes/comment',
    `comment`       TEXT            NULL COMMENT '审核评论',
    `diff_snapshot` LONGTEXT        NULL COMMENT '当时的diff快照',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_review_task_id` (`review_task_id`),
    KEY `idx_reviewer_id` (`reviewer_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Review审核记录表';

-- ============================================================
-- 7. Git 提交记录
-- ============================================================

CREATE TABLE `git_commit_log` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `review_task_id` BIGINT UNSIGNED NULL COMMENT '关联Review任务ID',
    `test_task_id`  BIGINT UNSIGNED NULL COMMENT '关联测试任务ID',
    `project_id`    BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `commit_hash`   VARCHAR(64)     NOT NULL DEFAULT '' COMMENT 'Git commit hash',
    `branch`        VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '提交分支',
    `commit_message` VARCHAR(512)   NOT NULL DEFAULT '' COMMENT '提交信息',
    `files_changed` JSON            NULL COMMENT '变更文件列表(JSON)',
    `push_status`   VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'push状态: pending/success/failed',
    `error_message` TEXT            NULL COMMENT '错误信息',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_review_task_id` (`review_task_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_commit_hash` (`commit_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Git提交记录表';

-- ============================================================
-- 8. CI 测试执行模块
-- ============================================================

-- 测试执行记录
CREATE TABLE `test_execution` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '执行ID',
    `project_id`    BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `trigger_type`  VARCHAR(16)     NOT NULL DEFAULT 'schedule' COMMENT '触发方式: schedule/manual/ci_callback',
    `trigger_by`    BIGINT UNSIGNED NULL COMMENT '触发人ID(manual时)',
    `branch`        VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '执行分支',
    `commit_hash`   VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '执行时的commit hash',
    `ci_job_id`     VARCHAR(128)    NOT NULL DEFAULT '' COMMENT 'CI Job ID',
    `ci_job_url`    VARCHAR(512)    NOT NULL DEFAULT '' COMMENT 'CI Job URL',
    `status`        VARCHAR(32)     NOT NULL DEFAULT 'pending' COMMENT '状态: pending/running/passed/failed/error',
    `total_cases`   INT             NOT NULL DEFAULT 0 COMMENT '总用例数',
    `passed_cases`  INT             NOT NULL DEFAULT 0 COMMENT '通过用例数',
    `failed_cases`  INT             NOT NULL DEFAULT 0 COMMENT '失败用例数',
    `skipped_cases` INT             NOT NULL DEFAULT 0 COMMENT '跳过用例数',
    `pass_rate`     DECIMAL(5,2)    NOT NULL DEFAULT 0.00 COMMENT '通过率(%)',
    `duration_sec`  INT             NOT NULL DEFAULT 0 COMMENT '执行时长(秒)',
    `result_detail` JSON            NULL COMMENT '详细结果(JSON)',
    `error_message` TEXT            NULL COMMENT '错误信息',
    `started_at`    DATETIME        NULL COMMENT '开始时间',
    `completed_at`  DATETIME        NULL COMMENT '完成时间',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_status` (`status`),
    KEY `idx_trigger_type` (`trigger_type`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试执行记录表';

-- 测试执行-问题单关联(一次执行可能涉及多个问题单的测试)
CREATE TABLE `test_execution_issue` (
    `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `execution_id`    BIGINT UNSIGNED NOT NULL COMMENT '执行ID',
    `issue_id`        BIGINT UNSIGNED NOT NULL COMMENT '问题单ID',
    `test_task_id`    BIGINT UNSIGNED NULL COMMENT '测试任务ID',
    `case_total`      INT             NOT NULL DEFAULT 0 COMMENT '该问题单用例总数',
    `case_passed`     INT             NOT NULL DEFAULT 0 COMMENT '通过数',
    `case_failed`     INT             NOT NULL DEFAULT 0 COMMENT '失败数',
    `result`          VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT 'pass/fail/error',
    `fail_detail`     JSON            NULL COMMENT '失败详情',
    `created_at`      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_execution_id` (`execution_id`),
    KEY `idx_issue_id` (`issue_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='执行-问题单关联表';

-- ============================================================
-- 9. 人工介入记录
-- ============================================================

CREATE TABLE `manual_intervention` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '介入记录ID',
    `execution_id`   BIGINT UNSIGNED NULL COMMENT '关联执行ID',
    `test_task_id`   BIGINT UNSIGNED NULL COMMENT '关联测试任务ID',
    `issue_id`       BIGINT UNSIGNED NOT NULL COMMENT '关联问题单ID',
    `project_id`     BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `operator_id`    BIGINT UNSIGNED NOT NULL COMMENT '操作人ID',
    `intervention_type` VARCHAR(32)  NOT NULL COMMENT '介入类型: modify_case/modify_script/modify_doc/rerun',
    `description`    TEXT            NULL COMMENT '修改说明',
    `before_snapshot` LONGTEXT       NULL COMMENT '修改前快照',
    `after_snapshot`  LONGTEXT       NULL COMMENT '修改后快照',
    `status`         VARCHAR(16)     NOT NULL DEFAULT 'completed' COMMENT 'completed/in_progress',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_execution_id` (`execution_id`),
    KEY `idx_issue_id` (`issue_id`),
    KEY `idx_operator_id` (`operator_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='人工介入记录表';

-- ============================================================
-- 10. 测试报告与通知
-- ============================================================

CREATE TABLE `test_report` (
    `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '报告ID',
    `execution_id`   BIGINT UNSIGNED NOT NULL COMMENT '关联执行ID',
    `project_id`     BIGINT UNSIGNED NOT NULL COMMENT '所属项目ID',
    `title`          VARCHAR(256)    NOT NULL COMMENT '报告标题',
    `summary`        TEXT            NULL COMMENT '报告摘要',
    `content`        LONGTEXT        NULL COMMENT '报告内容(HTML/MD)',
    `total_issues`   INT             NOT NULL DEFAULT 0 COMMENT '涉及问题单数',
    `total_cases`    INT             NOT NULL DEFAULT 0 COMMENT '总用例数',
    `passed_cases`   INT             NOT NULL DEFAULT 0 COMMENT '通过数',
    `failed_cases`   INT             NOT NULL DEFAULT 0 COMMENT '失败数',
    `pass_rate`      DECIMAL(5,2)    NOT NULL DEFAULT 0.00 COMMENT '通过率(%)',
    `has_intervention` TINYINT       NOT NULL DEFAULT 0 COMMENT '是否经过人工介入',
    `last_modifier`  VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '最近修改人',
    `report_url`     VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '报告链接',
    `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_execution_id` (`execution_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试报告表';

-- 通知发送日志
CREATE TABLE `notification_log` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `report_id`     BIGINT UNSIGNED NULL COMMENT '关联报告ID',
    `channel`       VARCHAR(16)     NOT NULL DEFAULT 'email' COMMENT '通知渠道: email/webhook/im',
    `recipient`     VARCHAR(128)    NOT NULL COMMENT '接收人(邮箱/webhook地址)',
    `subject`       VARCHAR(256)    NOT NULL DEFAULT '' COMMENT '主题',
    `content`       TEXT            NULL COMMENT '通知内容',
    `status`        VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT '状态: pending/sent/failed',
    `error_message` VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '错误信息',
    `sent_at`       DATETIME        NULL COMMENT '发送时间',
    `created_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_report_id` (`report_id`),
    KEY `idx_status` (`status`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='通知发送日志表';

-- ============================================================
-- 11. 操作审计日志
-- ============================================================

CREATE TABLE `operation_log` (
    `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`      BIGINT UNSIGNED NULL COMMENT '操作人ID',
    `username`     VARCHAR(64)     NOT NULL DEFAULT '' COMMENT '操作人用户名',
    `module`       VARCHAR(32)     NOT NULL COMMENT '操作模块',
    `action`       VARCHAR(32)     NOT NULL COMMENT '操作类型: create/update/delete/login/logout/approve/reject等',
    `target_type`  VARCHAR(32)     NOT NULL DEFAULT '' COMMENT '操作对象类型',
    `target_id`    BIGINT UNSIGNED NULL COMMENT '操作对象ID',
    `detail`       JSON            NULL COMMENT '操作详情(JSON)',
    `ip`           VARCHAR(45)     NOT NULL DEFAULT '' COMMENT '客户端IP',
    `user_agent`   VARCHAR(256)    NOT NULL DEFAULT '' COMMENT 'User Agent',
    `created_at`   DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`),
    KEY `idx_module_action` (`module`, `action`),
    KEY `idx_target` (`target_type`, `target_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作审计日志表';

-- ============================================================
-- 11.1 系统设置表
-- ============================================================

CREATE TABLE IF NOT EXISTS `system_setting` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `category`    VARCHAR(32)     NOT NULL COMMENT '分类: zentao/gitlab/ai/mail/cli_runtime',
    `key`         VARCHAR(64)     NOT NULL COMMENT '配置键',
    `value`       TEXT            NOT NULL COMMENT '配置值',
    `encrypted`   TINYINT         NOT NULL DEFAULT 0 COMMENT '是否加密存储: 1=是 0=否',
    `description` VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '说明',
    `updated_by`  BIGINT UNSIGNED NULL COMMENT '最后修改人',
    `created_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_category_key` (`category`, `key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统设置表';

-- ============================================================
-- 11.2 问题单同步日志
-- ============================================================

CREATE TABLE IF NOT EXISTS `issue_sync_log` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `project_id`    BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `status`        VARCHAR(16)     NOT NULL DEFAULT 'running' COMMENT '同步状态: running/success/failed',
    `full_sync`     TINYINT(1)      NOT NULL DEFAULT 0 COMMENT '是否全量同步',
    `added_count`   INT             NOT NULL DEFAULT 0 COMMENT '新增数量',
    `updated_count` INT             NOT NULL DEFAULT 0 COMMENT '更新数量',
    `deleted_count` INT             NOT NULL DEFAULT 0 COMMENT '删除数量',
    `error_message` TEXT            NULL COMMENT '错误信息',
    `started_at`    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
    `completed_at`  DATETIME        NULL COMMENT '结束时间',
    PRIMARY KEY (`id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_status` (`status`),
    KEY `idx_started_at` (`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='问题单同步日志';

-- ============================================================
-- 11.3 问题单同步明细
-- ============================================================

CREATE TABLE IF NOT EXISTS `issue_sync_log_detail` (
    `id`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `sync_log_id`        BIGINT UNSIGNED NOT NULL COMMENT '同步日志ID',
    `project_id`         BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
    `issue_id`           BIGINT UNSIGNED NULL COMMENT '本地问题单ID',
    `zentao_id`          INT             NOT NULL DEFAULT 0 COMMENT '禅道问题单ID',
    `issue_title`        VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '问题单标题',
    `action`             VARCHAR(16)     NOT NULL COMMENT '变更类型: added/updated/deleted',
    `changed_fields_json` JSON           NULL COMMENT '字段变更明细',
    `created_at`         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_sync_log_id` (`sync_log_id`),
    KEY `idx_project_id` (`project_id`),
    KEY `idx_action` (`action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='问题单同步明细';

-- ============================================================
-- 11.4 CLI 交互记录表
-- ============================================================

CREATE TABLE IF NOT EXISTS `cli_interaction` (
    `id`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `task_id`          BIGINT UNSIGNED NOT NULL COMMENT '关联的测试任务ID',
    `interaction_type` VARCHAR(32)     NOT NULL COMMENT '交互类型: ai_question/permission_request',
    `content`          TEXT            NOT NULL COMMENT '交互内容',
    `metadata`         JSON            NULL COMMENT '额外元数据',
    `status`           VARCHAR(16)     NOT NULL DEFAULT 'pending' COMMENT '状态: pending/approved/rejected/answered',
    `user_response`    TEXT            NULL COMMENT '用户回复内容',
    `user_id`          BIGINT UNSIGNED NULL COMMENT '回复的用户ID',
    `responded_at`     DATETIME        NULL COMMENT '回复时间',
    `created_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_task_id` (`task_id`),
    KEY `idx_status` (`status`),
    KEY `idx_type` (`interaction_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='CLI交互记录表';

-- ============================================================
-- 11.5 任务运行事件日志表
-- ============================================================

CREATE TABLE IF NOT EXISTS `task_event_log` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `task_id`    BIGINT UNSIGNED NOT NULL,
    `seq`        INT UNSIGNED NOT NULL DEFAULT 0,
    `type`       VARCHAR(32)    NOT NULL DEFAULT '',
    `stage`      VARCHAR(64)    NOT NULL DEFAULT '',
    `status`     VARCHAR(32)    NOT NULL DEFAULT '',
    `message`    TEXT           NOT NULL,
    `data`       JSON           NULL,
    `created_at` DATETIME(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    INDEX `idx_task_id` (`task_id`),
    INDEX `idx_task_seq` (`task_id`, `seq`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='任务运行事件日志';

-- ============================================================
-- 12. 初始化数据
-- ============================================================

-- 系统设置默认配置
INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`) VALUES
    ('zentao', 'base_url',       '', 0, '禅道服务器地址'),
    ('zentao', 'account',        '', 0, '禅道登录账号'),
    ('zentao', 'password',       '', 1, '禅道登录密码'),
    ('zentao', 'token',          '', 1, '禅道API Token'),
    ('zentao', 'token_expire_at','', 0, 'Token过期时间'),
    ('zentao', 'sync_interval',  '30', 0, '同步频率(分钟)'),
    ('zentao', 'sync_enabled',   '1', 0, '是否启用自动同步: 1=是 0=否'),
    ('gitlab', 'base_url',      '', 0, 'GitLab服务器地址'),
    ('gitlab', 'access_token',  '', 1, 'GitLab Personal Access Token'),
    ('gitlab', 'api_version',   'v4', 0, 'GitLab API版本'),
    ('cli_runtime', 'command', '', 0, 'CLI 可执行命令'),
    ('cli_runtime', 'args_json', '[]', 0, 'CLI 参数(JSON数组字符串)'),
    ('cli_runtime', 'timeout', '20m', 0, 'CLI 超时时间'),
    ('cli_runtime', 'workspace_root', '/tmp/auto-test-flow/cli-runtime', 0, 'CLI 工作区根目录'),
    ('cli_runtime', 'repo_dir_name', 'repo', 0, '仓库目录名称'),
    ('cli_runtime', 'control_dir_name', '.autotestflow', 0, '控制目录名称'),
    ('cli_runtime', 'input_file_name', 'input.json', 0, '输入文件名'),
    ('cli_runtime', 'prompt_file_name', 'prompt.md', 0, 'Prompt 文件名'),
    ('cli_runtime', 'result_file_name', 'result.json', 0, '结果文件名'),
    ('cli_runtime', 'log_file_name', 'cli.log', 0, '日志文件名'),
    ('cli_runtime', 'preserve_workspace', 'true', 0, '是否保留工作区: true/false'),
    ('cli_runtime', 'env_json', '{}', 0, '额外环境变量(JSON对象字符串)'),
    ('mail', 'host', '', 0, 'SMTP服务器地址'),
    ('mail', 'port', '465', 0, 'SMTP端口'),
    ('mail', 'username', '', 0, 'SMTP用户名'),
    ('mail', 'password', '', 1, 'SMTP密码'),
    ('mail', 'from', 'autotest@example.com', 0, '发件人邮箱'),
    ('mail', 'use_ssl', '1', 0, '是否启用SSL: 1=是 0=否'),
    ('mail', 'default_recipients', '', 0, '默认通知收件人'),
    ('mail', 'review_result_subject_template', '[AutoTestFlow] Review结果 - {{title}}', 0, 'Review结果邮件主题模板'),
    ('mail', 'review_result_body_template', '<h2>Review结果通知</h2><p><strong>标题:</strong> {{title}}</p><p><strong>状态:</strong> {{status}}</p><p><strong>审核意见:</strong> {{review_note}}</p><p><strong>Git推送:</strong> {{git_summary}}</p><p><strong>问题单:</strong> {{issue_title}}</p><p><strong>项目:</strong> {{project_name}}</p>', 0, 'Review结果邮件正文模板'),
    ('mail', 'test_report_subject_template', '[AutoTestFlow] 测试报告 - {{title}}', 0, '测试报告邮件主题模板'),
    ('mail', 'test_report_body_template', '<h2>测试报告: {{title}}</h2><p><strong>总用例数:</strong> {{total_cases}}</p><p><strong>通过:</strong> {{passed_cases}} | <strong>失败:</strong> {{failed_cases}}</p><p><strong>通过率:</strong> {{pass_rate}}%</p><p><strong>是否经过人工介入:</strong> {{has_intervention}}</p><hr><p>{{summary}}</p><p><a href="{{report_url}}">查看完整报告</a></p>', 0, '测试报告邮件正文模板')
ON DUPLICATE KEY UPDATE `description` = VALUES(`description`);

-- 初始化角色
INSERT INTO `role` (`code`, `name`, `description`) VALUES
    ('admin',     '管理员',     '系统管理员，拥有全部权限'),
    ('test_lead', '测试负责人', '负责测试管理、审核和任务分配'),
    ('tester',    '测试工程师', '执行测试任务和人工介入'),
    ('dev_lead',  '开发负责人', '查看测试结果和报告'),
    ('viewer',    '查看者',     '只读权限');

-- 初始化权限
INSERT INTO `permission` (`code`, `name`, `module`) VALUES
    -- 用户模块
    ('user:list',    '查看用户列表', 'user'),
    ('user:create',  '创建用户',     'user'),
    ('user:update',  '编辑用户',     'user'),
    ('user:delete',  '删除用户',     'user'),
    -- 项目模块
    ('project:list',   '查看项目列表', 'project'),
    ('project:create', '创建项目',     'project'),
    ('project:update', '编辑项目',     'project'),
    ('project:delete', '删除项目',     'project'),
    -- 问题单模块
    ('issue:list',   '查看问题单',   'issue'),
    ('issue:sync',   '同步问题单',   'issue'),
    ('issue:update', '编辑问题单',   'issue'),
    -- Agent模块
    ('agent:list',   '查看Agent',   'agent'),
    ('agent:manage', '管理Agent',   'agent'),
    -- Review模块
    ('review:list',     '查看Review',  'review'),
    ('review:submit',   '提交Review',  'review'),
    ('review:approve',  '审核Review',  'review'),
    -- 测试模块
    ('test:list',       '查看测试任务', 'test'),
    ('test:create',     '创建测试任务', 'test'),
    ('test:execute',    '执行测试生成', 'test'),
    ('test:trigger',    '触发测试',     'test'),
    ('test:intervene',  '人工介入',     'test'),
    -- 报告模块
    ('report:list',     '查看报告',     'report'),
    ('report:export',   '导出报告',     'report');

-- 管理员拥有全部权限
INSERT INTO `role_permission` (`role_id`, `permission_id`)
SELECT 1, id FROM `permission`;

-- 测试负责人权限
INSERT INTO `role_permission` (`role_id`, `permission_id`)
SELECT 2, id FROM `permission` WHERE `code` IN (
    'user:list', 'project:list', 'project:update',
    'issue:list', 'issue:sync', 'issue:update',
    'agent:list', 'agent:manage',
    'review:list', 'review:submit', 'review:approve',
    'test:list', 'test:create', 'test:execute', 'test:trigger', 'test:intervene',
    'report:list', 'report:export'
);

-- 测试工程师权限
INSERT INTO `role_permission` (`role_id`, `permission_id`)
SELECT 3, id FROM `permission` WHERE `code` IN (
    'project:list', 'issue:list',
    'agent:list',
    'review:list', 'review:submit',
    'test:list', 'test:execute', 'test:intervene',
    'report:list'
);

-- 开发负责人权限
INSERT INTO `role_permission` (`role_id`, `permission_id`)
SELECT 4, id FROM `permission` WHERE `code` IN (
    'project:list', 'issue:list',
    'review:list',
    'test:list',
    'report:list'
);

-- 查看者权限
INSERT INTO `role_permission` (`role_id`, `permission_id`)
SELECT 5, id FROM `permission` WHERE `code` IN (
    'project:list', 'issue:list',
    'review:list',
    'test:list',
    'report:list'
);

-- 初始化管理员用户 (密码: admin123, bcrypt hash)
INSERT INTO `user` (`username`, `password_hash`, `real_name`, `email`, `status`) VALUES
    ('admin', '$2a$10$dNLM8X/X/kI0Sopbb9suoeM0dSgeL0ToAR2nh8scub1s4HFF3pjj2', '系统管理员', 'admin@example.com', 1);

INSERT INTO `user_role` (`user_id`, `role_id`) VALUES (1, 1);
