-- ============================================================
-- 初始化 gen-test Skill
-- ============================================================

SET NAMES utf8mb4;
USE auto_test_flow;

INSERT INTO `skill` (`name`, `description`, `skill_type`, `prompt_template`, `input_schema`, `output_schema`, `status`)
VALUES (
    'gen-test',
    'AI自动生成测试用例、测试脚本和测试文档。基于项目文档和问题单内容，生成覆盖主流程/异常/边界/回归的完整测试方案。',
    'builtin',
    '你是一个专业的测试工程师。请根据以下信息生成测试用例、测试脚本和测试文档。\n\n## 项目: {{project_name}}\n\n## 功能文档\n{{func_doc_content}}\n\n## 设计文档\n{{design_doc_content}}\n\n## 数据库文档\n{{db_doc_content}}\n\n## 历史测试文档\n{{test_doc_content}}\n\n## 待测试问题单\n- 标题: {{issue_title}}\n- 描述: {{issue_description}}\n- 严重程度: {{issue_severity}}\n\n请严格按照JSON格式返回测试用例、测试脚本和测试文档。',
    '{"type":"object","properties":{"project_name":{"type":"string"},"func_doc_content":{"type":"string"},"design_doc_content":{"type":"string"},"db_doc_content":{"type":"string"},"test_doc_content":{"type":"string"},"issue_title":{"type":"string"},"issue_description":{"type":"string"},"issue_severity":{"type":"string"}}}',
    '{"type":"object","properties":{"test_cases":{"type":"array"},"test_script":{"type":"object"},"test_doc":{"type":"object"},"summary":{"type":"string"}}}',
    1
);

-- 初始化一个默认 Agent
INSERT INTO `agent` (`name`, `description`, `model_provider`, `model_name`, `api_key_ref`, `max_tokens`, `temperature`, `status`)
VALUES (
    'default-agent',
    '默认测试生成Agent，使用Claude模型',
    'claude',
    'claude-sonnet-4-20250514',
    'AI_API_KEY',
    8192,
    0.30,
    1
);

-- 绑定 Agent 和 Skill
INSERT INTO `agent_skill` (`agent_id`, `skill_id`, `priority`)
VALUES (1, 1, 1);
