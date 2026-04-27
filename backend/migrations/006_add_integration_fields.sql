-- ============================================================
-- 流水线集成字段
-- 用于支持 DevFlow 提交通知和 CI/CD 部署通知功能
-- ============================================================

-- 1. issue 表添加研发流水线提交时间字段
ALTER TABLE issue ADD COLUMN dev_flow_submit_time DATETIME NULL COMMENT '研发流水线提交时间' AFTER synced_at;

-- 2. 添加索引优化查询性能
CREATE INDEX idx_issue_dev_flow_submit_time ON issue(dev_flow_submit_time);

-- 3. system_setting 表添加集成配置项
INSERT INTO system_setting (category, `key`, value, encrypted, description, created_at, updated_at) VALUES
('integration', 'api_token', '', 0, '流水线集成API认证Token', NOW(), NOW()),
('integration', 'max_concurrent_tasks', '1', 0, 'CI/CD部署完成后并行生成测试任务的最大数量', NOW(), NOW());
