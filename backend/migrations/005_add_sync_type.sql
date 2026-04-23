-- 添加 sync_type 字段区分 Bug 同步和用例同步

-- 1. issue_sync_log 表添加 sync_type 字段
ALTER TABLE issue_sync_log ADD COLUMN sync_type VARCHAR(16) NOT NULL DEFAULT 'issue' COMMENT '同步类型: issue-Bug同步, testcase-用例同步' AFTER project_id;
CREATE INDEX idx_issue_sync_log_sync_type ON issue_sync_log(sync_type);

-- 2. issue_sync_log_detail 表添加 sync_type 字段
ALTER TABLE issue_sync_log_detail ADD COLUMN sync_type VARCHAR(16) NOT NULL DEFAULT 'issue' COMMENT '同步类型' AFTER project_id;

-- 3. issue_sync_log_detail 表添加 test_case_id 字段用于用例同步
ALTER TABLE issue_sync_log_detail ADD COLUMN test_case_id BIGINT UNSIGNED NULL COMMENT '用例ID' AFTER issue_id;
CREATE INDEX idx_issue_sync_log_detail_test_case_id ON issue_sync_log_detail(test_case_id);

-- 4. 添加 sync_type 索引
CREATE INDEX idx_issue_sync_log_detail_sync_type ON issue_sync_log_detail(sync_type);
