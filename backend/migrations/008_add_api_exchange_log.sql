-- ============================================================
-- API 收发历史记录
-- 用于记录 DevFlow 提交通知、CI/CD 部署通知、研发流水线测试结果回调等接口的请求与响应
-- ============================================================

CREATE TABLE IF NOT EXISTS `api_exchange_log` (
    `id`                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `api_name`            VARCHAR(64)     NOT NULL COMMENT 'API名称: devflow_submit/cicd_deploy/external_task_test_result',
    `direction`           VARCHAR(16)     NOT NULL COMMENT '方向: inbound/outbound',
    `method`              VARCHAR(16)     NOT NULL COMMENT 'HTTP方法',
    `url`                 VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '请求URL或路径',
    `remote_addr`         VARCHAR(128)    NOT NULL DEFAULT '' COMMENT '对端地址',
    `request_headers`     JSON            NULL COMMENT '请求头(JSON，敏感值脱敏)',
    `request_body`        LONGTEXT        NULL COMMENT '请求体',
    `response_status`     INT             NOT NULL DEFAULT 0 COMMENT 'HTTP响应状态码',
    `response_body`       LONGTEXT        NULL COMMENT '响应体',
    `result_status`       VARCHAR(16)     NOT NULL DEFAULT 'success' COMMENT '处理结果: success/failed',
    `error_message`       TEXT            NULL COMMENT '错误信息',
    `duration_millis`     BIGINT          NOT NULL DEFAULT 0 COMMENT '耗时(毫秒)',
    `related_issue_id`    BIGINT UNSIGNED NULL COMMENT '关联问题单ID',
    `related_task_id`     BIGINT UNSIGNED NULL COMMENT '关联测试任务ID',
    `related_dev_task_id` VARCHAR(191)    NOT NULL DEFAULT '' COMMENT '关联研发流水线任务ID',
    `created_at`          DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`id`),
    KEY `idx_api_name` (`api_name`),
    KEY `idx_direction` (`direction`),
    KEY `idx_result_status` (`result_status`),
    KEY `idx_related_issue_id` (`related_issue_id`),
    KEY `idx_related_task_id` (`related_task_id`),
    KEY `idx_related_dev_task_id` (`related_dev_task_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='API收发历史记录表';
