-- 添加禅道用例表（独立于 issue/bug 表）

CREATE TABLE IF NOT EXISTS `zentao_test_case` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `zentao_id` INT NOT NULL COMMENT '禅道用例ID',
    `product_id` BIGINT UNSIGNED NOT NULL COMMENT '产品ID',
    `title` VARCHAR(512) NOT NULL COMMENT '用例标题',
    `precondition` TEXT COMMENT '前置条件',
    `keywords` VARCHAR(255) COMMENT '关键词',
    `priority` TINYINT DEFAULT 3 COMMENT '优先级',
    `type` VARCHAR(32) COMMENT '用例类型',
    `stage` VARCHAR(32) COMMENT '适用阶段',
    `status` VARCHAR(32) DEFAULT 'normal' COMMENT '状态',
    `test_status` VARCHAR(32) DEFAULT 'pending' COMMENT '测试状态',
    `branch` VARCHAR(128) COMMENT '分支',
    `module` VARCHAR(255) COMMENT '模块',
    `steps` TEXT COMMENT '测试步骤',
    `expected` TEXT COMMENT '预期结果',
    `opened_by` VARCHAR(64) COMMENT '创建人',
    `created_by` VARCHAR(64) COMMENT '创建人',
    `synced_at` DATETIME COMMENT '同步时间',
    `zentao_updated_at` DATETIME COMMENT '禅道更新时间',
    `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_ztc_zentao_project` (`zentao_id`, `product_id`),
    KEY `idx_product_id` (`product_id`),
    KEY `idx_test_status` (`test_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='禅道用例表';
