-- ============================================================
-- 添加研发流水线任务ID字段
-- ============================================================

-- 1. issue 表添加研发流水线任务ID字段
ALTER TABLE issue ADD COLUMN dev_task_id VARCHAR(128) DEFAULT '' COMMENT '研发流水线任务ID' AFTER dev_flow_submit_time;

-- 2. system_setting 表添加研发流水线通知配置
INSERT INTO system_setting (category, `key`, value, encrypted, description, created_at, updated_at) VALUES
('integration', 'devflow_callback_url', '', 0, '研发流水线回调URL（测试失败时通知）', NOW(), NOW()),
('integration', 'devflow_api_key', '', 0, '研发流水线回调API Key', NOW(), NOW());
