-- ============================================================
-- 运行时设置
-- 控制待生成工单消费及生成任务执行上限
-- ============================================================

INSERT INTO system_setting (category, `key`, value, encrypted, description, created_at, updated_at) VALUES
('runtime', 'task_timeout_minutes', '30', 0, '生成类任务最大执行超时时间(分钟)', NOW(), NOW()),
('runtime', 'pending_generate_interval_minutes', '1', 0, '待生成工单定时巡检间隔(分钟)', NOW(), NOW());
