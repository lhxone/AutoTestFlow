package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

const (
	defaultGenTestTurnLimit          = 1000
	defaultGenTestToolResultMaxBytes = 12_000
	defaultGenTestReadMaxBytes       = 64_000
)

var errStopRepoWalk = errors.New("stop repo walk")

type EinoGenTestRuntime struct {
	logger          *zap.Logger
	eventHub        *TaskEventHub
	interactionRepo *repository.CLIInteractionRepo
	workspace       *GenTestWorkspaceService
	httpClient      *http.Client
}

type genTestToolCallContext struct {
	Task      *model.TestTask
	Input     *GenTestInput
	Workflow  *model.Skill
	Agent     *model.Agent
	Workspace *RuntimeWorkspace
	MCP       *MCPRuntime
}

type genTestToolResult struct {
	Content     string
	IsError     bool
	FinalOutput *GenTestOutput
}

type genTestToolSpec struct {
	Name        string
	Description string
	Schema      map[string]any
}

type runtimeToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type runtimeMessage struct {
	Role       string
	Text       string
	ToolCalls  []runtimeToolCall
	ToolCallID string
	ToolResult string
	ToolError  bool
}

type runtimeModelResponse struct {
	Text      string
	ToolCalls []runtimeToolCall
}

func NewEinoGenTestRuntime(logger *zap.Logger) *EinoGenTestRuntime {
	return &EinoGenTestRuntime{
		logger:          logger,
		eventHub:        DefaultTaskEventHub,
		interactionRepo: repository.NewCLIInteractionRepo(),
		workspace:       NewGenTestWorkspaceService(logger),
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (r *EinoGenTestRuntime) Generate(
	ctx context.Context,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
) (*GenTestOutput, error) {
	if task == nil {
		return nil, fmt.Errorf("测试任务不能为空")
	}
	if task.Project == nil {
		return nil, fmt.Errorf("测试任务缺少项目信息")
	}

	workspaceCfg, err := ResolveGenTestWorkspaceConfig(agent)
	if err != nil {
		return nil, err
	}

	workspace, err := r.workspace.Prepare(ctx, task.ID, task, workspaceCfg)
	if err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "workspace_prepared", model.TaskStatusRunning,
		fmt.Sprintf("Eino 运行时工作区已准备完成\n  工作区: %s\n  仓库目录: %s", workspace.RootDir, workspace.RepoDir),
		map[string]any{
			"workspace_dir": workspace.RootDir,
			"repo_dir":      workspace.RepoDir,
		})

	prompt := r.buildPrompt(workspace, task, input, workflow, agent, promptCtx)
	if err := r.workspace.WriteControlFiles(workspace, input, prompt, agent); err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "control_files_written", model.TaskStatusRunning,
		fmt.Sprintf("运行时输入文件和 Prompt 已写入\n  输入文件: %s\n  Prompt: %s\n  结果文件: %s",
			workspace.InputFile, workspace.PromptFile, workspace.ResultFile),
		map[string]any{
			"input_file":  workspace.InputFile,
			"prompt_file": workspace.PromptFile,
			"result_file": workspace.ResultFile,
		})

	var mcpRuntime *MCPRuntime
	if agent != nil && len(agent.MCPServers) > 0 {
		if runtime, runtimeErr := NewMCPRuntime(ctx, r.logger, agent); runtimeErr != nil {
			r.publish(task.ID, taskEventTypeLog, "mcp_runtime", model.TaskStatusRunning, fmt.Sprintf("MCP 运行时初始化失败，已跳过: %v", runtimeErr), nil)
		} else {
			mcpRuntime = runtime
			defer mcpRuntime.Close()
		}
	}

	execCfg := ResolveAgentExecutionConfig(agent)
	if strings.TrimSpace(execCfg.APIKey) == "" {
		return nil, fmt.Errorf("Agent 未配置可用 API Key")
	}
	if strings.TrimSpace(execCfg.BaseURL) == "" {
		return nil, fmt.Errorf("Agent 未配置可用 Base URL")
	}

	r.publish(task.ID, taskEventTypeStage, "runtime_started", model.TaskStatusRunning,
		fmt.Sprintf("开始执行 Eino 原生运行时\n  provider: %s\n  model: %s", execCfg.Provider, execCfg.Model),
		map[string]any{
			"provider": execCfg.Provider,
			"model":    execCfg.Model,
		})

	agentCtx := &genTestToolCallContext{
		Task:      task,
		Input:     input,
		Workflow:  workflow,
		Agent:     agent,
		Workspace: workspace,
		MCP:       mcpRuntime,
	}
	output, err := r.runAgentLoop(ctx, execCfg, agentCtx, promptCtx)
	if err != nil {
		return nil, err
	}

	if err := r.workspace.SyncArtifacts(workspace.RepoDir, task, input, output); err != nil {
		return nil, err
	}
	r.publish(task.ID, taskEventTypeStage, "artifacts_synced", model.TaskStatusRunning, "测试脚本和测试文档已同步到仓库工作区", map[string]any{
		"script_file": output.TestScript.FilePath,
		"doc_file":    output.TestDoc.FilePath,
	})

	output.Workspace = workspace
	if err := r.workspace.WriteResultFile(workspace, output); err != nil {
		r.logger.Warn("写入运行时结果文件失败", zap.Uint64("task_id", task.ID), zap.Error(err))
	}
	r.publish(task.ID, taskEventTypeStage, "result_loaded", model.TaskStatusRunning, "已生成结构化结果文件", nil)
	return output, nil
}

func (r *EinoGenTestRuntime) buildPrompt(
	workspace *RuntimeWorkspace,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
) string {
	workflowPrompt := ""
	if workflow != nil && strings.TrimSpace(workflow.PromptTemplate) != "" {
		workflowPrompt = "\n## Workflow Prompt Template\n" + strings.TrimSpace(workflow.PromptTemplate) + "\n"
	}

	mcpNote := "\n## MCP 预检\n- 当前未发现可用的 MCP 能力摘要，若生成过程中需要外部浏览器/系统能力，请仅使用已暴露的工具。\n"
	if promptCtx != nil {
		parts := make([]string, 0, 4)
		if len(promptCtx.ChromeMCPServers) > 0 {
			parts = append(parts, fmt.Sprintf("- 已接入 Chrome MCP Server: %s", strings.Join(promptCtx.ChromeMCPServers, ", ")))
		}
		if summary := strings.TrimSpace(promptCtx.MCPCapabilitySummary); summary != "" {
			parts = append(parts, "- MCP 能力摘要:")
			parts = append(parts, summary)
		}
		if len(parts) > 0 {
			mcpNote = "\n## MCP 预检\n" + strings.Join(parts, "\n") + "\n"
		}
	}

	agentNote := ""
	if agent != nil {
		agentNote = fmt.Sprintf("\n## Agent\n- name: %s\n- description: %s\n- provider: %s\n- model: %s\n",
			agent.Name, strings.TrimSpace(agent.Description), agent.ModelProvider, agent.ModelName)
	}

	inputJSON, _ := json.MarshalIndent(input, "", "  ")
	return fmt.Sprintf(`# AutoTestFlow Eino Runtime

你正在执行 AutoTestFlow 的生成测试用例/脚本任务，请在当前仓库目录内完成测试资产生成。

## 强制规则
- 优先遵循流程：探索测试结构 -> 发现共享工具(helper路径下)/数据 -> 生成测试文档和脚本 -> 运行自测 -> 修复。
- 不需要确保所有用例通过，因为功能未修复而导致的用例失败可以视为已经生成成功，允许调用SubmitGenTestResult 工具。
- 你必须通过可用工具真实地探索仓库和运行命令，不要凭空假设项目结构。
- 如果是 Web UI 测试且有 Chrome MCP，优先用 Chrome MCP 通过data-testid确认 DOM、交互和选择器。
- 生成的测试脚本必须包含至少一个可执行断言。
- 必须把实际文件写入仓库目录。
- 完成后必须调用 SubmitGenTestResult 工具提交结构化结果，而不是只输出自然语言。
- 自测完成后，将 Playwright 报告路径写入 self_test.playwright.report_path（例如 "playwright-report/index.html"）。
- SubmitGenTestResult 必须严格使用标准字段名，禁止使用 path、case_name、prerequisites、module、status、expected_result 等别名字段。
- 若关键信息缺失，可调用 AskUserQuestion；若需要人工许可，可调用 RequestPermission。
- 必须根据当前运行平台选择命令工具；当前平台: %s；可用命令工具: %s。
- 在创建文件/编辑文件后，必须实时更新结果文件，不要等最后一步才写入结果文件。

## SubmitGenTestResult 标准示例
你最终调用 SubmitGenTestResult 时，参数必须遵循下面的结构；字段名必须完全一致：

{
	"test_cases": [
		{
			"title": "验证语言从简体中文切换到 English 后导航菜单翻译为英文",
			"category": "顶部导航-语言切换",
			"precondition": "1. 用户已登录系统\n2. 当前页面语言为简体中文",
			"steps": "1. 打开首页\n2. 切换语言到 English\n3. 检查导航菜单文本",
			"expected": "导航菜单项显示为英文，不再包含中文文本",
			"self_test_result": "pass",
			"priority": 1
		}
	],
	"test_script": {
		"file_path": "test-cases/zentao/script/example.spec.ts",
		"file_content": "",
		"language": "typescript"
	},
	"test_doc": {
		"title": "禅道回归测试文档",
		"file_path": "test-cases/zentao/docs/example.md",
		"content": ""
	},
	"self_test": {
		"passed": true,
		"summary": "自测通过",
		"checks": ["Playwright 用例通过"],
		"playwright": {
			"passed": true,
			"report_path": "playwright-report/index.html",
			"summary": "所有 Playwright 用例通过"
		}
	},
	"summary": "已生成测试用例、脚本和文档"
}

禁止输出以下错误写法：
- test_script.path
- test_doc.path
- test_cases.case_name
- test_cases.prerequisites
- test_cases.module
- test_cases.status
- test_cases.expected_result


严格按照如下结构生成测试用例markdown文件。

**文件名**
ZentaoBugTestcase-{禅道问题单的ID}-{禅道问题单标题的英文翻译，简写}

**内容要求**
严格以中文简体输出测试用例文档，UTF-8编码

**文档结构**（严格按照以下结构，禁止新增/缺少内容块）：

Title
{用不超过80字描述[模块] [场景] [触发条件] [异常现象]，用禅道问题单标题}

Objective
{用一句话描述测试目的：To verify the function that ...}

Prerequisites
1. {具体前置条件}
2. {具体前置条件}
...

| Step | Procedure | Expected result |
|------|-----------|-----------------|
| 1 | {操作步骤描述} | {该步骤完成后的具体可见结果} |
| 2 | {操作步骤描述} | {该步骤完成后的具体可见结果} |
| ... | ... | ... |

Remarks
{备注（可选）}

Caution
{注意事项（可选）}


## 可用路径
- 仓库根目录: %s
- 输入文件: %s
- Prompt 文件: %s
- 结果文件: %s

## 当前任务
- task_id: %d
- issue_id: %d
- project: %s
- issue_title: %s
- issue_severity: %s

## 输入上下文 JSON
%s
%s%s%s`,
		runtime.GOOS,
		strings.Join(availableCommandToolNames(runtime.GOOS), ", "),
		workspace.RepoDir,
		workspace.InputFile,
		workspace.PromptFile,
		workspace.ResultFile,
		task.ID,
		task.IssueID,
		task.Project.Name,
		input.IssueTitle,
		input.IssueSeverity,
		string(inputJSON),
		workflowPrompt,
		mcpNote,
		agentNote,
	)
}

func (r *EinoGenTestRuntime) runAgentLoop(
	ctx context.Context,
	cfg AgentExecutionConfig,
	toolCtx *genTestToolCallContext,
	promptCtx *CLIPromptContext,
) (*GenTestOutput, error) {
	systemPrompt := "你是 AutoTestFlow 的测试生成代理。严格使用工具探索事实，少猜测，多验证。完成所有动作后调用 SubmitGenTestResult。"
	userPrompt := r.buildPrompt(toolCtx.Workspace, toolCtx.Task, toolCtx.Input, toolCtx.Workflow, toolCtx.Agent, promptCtx)

	history := []runtimeMessage{
		{Role: "system", Text: systemPrompt},
		{Role: "user", Text: userPrompt},
	}
	toolSpecs := r.baseToolSpecs()
	if toolCtx.MCP != nil {
		for _, tool := range toolCtx.MCP.OpenAITools() {
			fn, ok := tool["function"].(map[string]any)
			if !ok {
				continue
			}
			name, _ := fn["name"].(string)
			description, _ := fn["description"].(string)
			parameters, _ := fn["parameters"].(map[string]any)
			toolSpecs = append(toolSpecs, genTestToolSpec{
				Name:        name,
				Description: description,
				Schema:      parameters,
			})
		}
	}

	for turn := 1; turn <= defaultGenTestTurnLimit; turn++ {
		r.publish(toolCtx.Task.ID, taskEventTypeLog, "cli_output_raw", model.TaskStatusRunning, marshalTaskEventJSON(map[string]any{
			"type":    "system",
			"message": fmt.Sprintf("模型回合 %d", turn),
		}), nil)

		resp, err := r.callModelWithConnectionRetry(ctx, cfg, history, toolSpecs, toolCtx.Task.ID, turn)
		if err != nil {
			if isModelConnectionError(err) {
				r.publish(toolCtx.Task.ID, taskEventTypeLog, "cli_output_raw", model.TaskStatusRunning, marshalTaskEventJSON(map[string]any{
					"type":    "system",
					"message": fmt.Sprintf("模型回合 %d 连接错误重试 %d 次仍失败，进入下一回合: %v", turn, modelConnectionRetryAttempts, err),
				}), nil)
				continue
			}
			return nil, err
		}

		history = append(history, runtimeMessage{
			Role:      "assistant",
			Text:      resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		if text := strings.TrimSpace(resp.Text); text != "" {
			r.publishAssistantText(toolCtx.Task.ID, cfg.Model, text)
			if output, parseErr := parseGenTestOutputFromText(text); parseErr == nil && output != nil {
				return output, nil
			}
		}

		if len(resp.ToolCalls) == 0 {
			if turn == defaultGenTestTurnLimit {
				break
			}
			history = append(history, runtimeMessage{
				Role: "user",
				Text: "请不要只返回说明，继续通过工具完成写文件/自测，并调用 SubmitGenTestResult 提交结构化结果。",
			})
			continue
		}

		var finalOutput *GenTestOutput
		for _, call := range resp.ToolCalls {
			r.publishAssistantToolCall(toolCtx.Task.ID, call.Name, call.Arguments)
			result, err := r.executeTool(ctx, toolCtx, call)
			if err != nil {
				result = &genTestToolResult{
					Content: fmt.Sprintf("tool execution failed: %v", err),
					IsError: true,
				}
			}
			if result == nil {
				result = &genTestToolResult{Content: "tool executed"}
			}
			if result.FinalOutput != nil {
				finalOutput = result.FinalOutput
			}
			history = append(history, runtimeMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				ToolResult: truncateToolResult(result.Content),
				ToolError:  result.IsError,
			})
			r.publishToolResult(toolCtx.Task.ID, result.Content, result.IsError)
		}
		if finalOutput != nil {
			return finalOutput, nil
		}
	}

	return nil, fmt.Errorf("模型在 %d 个回合内未提交结构化结果", defaultGenTestTurnLimit)
}

func (r *EinoGenTestRuntime) callModel(
	ctx context.Context,
	cfg AgentExecutionConfig,
	history []runtimeMessage,
	tools []genTestToolSpec,
) (*runtimeModelResponse, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "claude":
		return r.callAnthropic(ctx, cfg, history, tools)
	case "openai", "zhipu", "custom":
		return r.callOpenAICompatible(ctx, cfg, history, tools)
	default:
		return nil, fmt.Errorf("不支持的模型提供商: %s", cfg.Provider)
	}
}

func (r *EinoGenTestRuntime) callModelWithConnectionRetry(
	ctx context.Context,
	cfg AgentExecutionConfig,
	history []runtimeMessage,
	tools []genTestToolSpec,
	taskID uint64,
	turn int,
) (*runtimeModelResponse, error) {
	resp, err := r.callModel(ctx, cfg, history, tools)
	if err == nil {
		return resp, nil
	}
	if !isModelConnectionError(err) {
		return nil, err
	}

	lastErr := err
	for retry := 1; retry <= modelConnectionRetryAttempts; retry++ {
		r.publish(taskID, taskEventTypeLog, "cli_output_raw", model.TaskStatusRunning, marshalTaskEventJSON(map[string]any{
			"type":    "system",
			"message": fmt.Sprintf("模型回合 %d 连接错误，重试中 (%d/%d)，20秒后重试: %v", turn, retry, modelConnectionRetryAttempts, lastErr),
		}), nil)

		if err := sleepWithContext(ctx, modelConnectionRetryDelay); err != nil {
			return nil, err
		}
		resp, err := r.callModel(ctx, cfg, history, tools)
		if err == nil {
			return resp, nil
		}
		if !isModelConnectionError(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

func (r *EinoGenTestRuntime) callOpenAICompatible(
	ctx context.Context,
	cfg AgentExecutionConfig,
	history []runtimeMessage,
	tools []genTestToolSpec,
) (*runtimeModelResponse, error) {
	endpoint := resolveOpenAICompatibleEndpoint(cfg.BaseURL)

	reqBody := map[string]any{
		"model":       cfg.Model,
		"messages":    buildOpenAIHistory(history),
		"tools":       buildOpenAIToolPayloads(tools),
		"tool_choice": "auto",
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxTokens,
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构建 OpenAI 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求模型服务失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("模型服务返回错误 %d: %s", resp.StatusCode, truncateText(string(respBody), 400))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析模型响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("模型服务返回空响应")
	}

	choice := result.Choices[0].Message
	output := &runtimeModelResponse{Text: strings.TrimSpace(choice.Content)}
	for _, call := range choice.ToolCalls {
		args := make(map[string]any)
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("解析 tool arguments 失败 (%s): %w", call.Function.Name, err)
			}
		}
		output.ToolCalls = append(output.ToolCalls, runtimeToolCall{
			ID:        strings.TrimSpace(call.ID),
			Name:      strings.TrimSpace(call.Function.Name),
			Arguments: args,
		})
	}
	return output, nil
}

func (r *EinoGenTestRuntime) callAnthropic(
	ctx context.Context,
	cfg AgentExecutionConfig,
	history []runtimeMessage,
	tools []genTestToolSpec,
) (*runtimeModelResponse, error) {
	endpoint := resolveAnthropicMessagesEndpoint(cfg.BaseURL)

	systemText, messages := buildAnthropicHistory(history)
	reqBody := map[string]any{
		"model":       cfg.Model,
		"system":      systemText,
		"messages":    messages,
		"tools":       buildAnthropicToolPayloads(tools),
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxTokens,
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构建 Anthropic 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Anthropic 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Anthropic 返回错误 %d: %s", resp.StatusCode, truncateText(string(respBody), 400))
	}

	var result struct {
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Input any    `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 响应失败: %w", err)
	}

	output := &runtimeModelResponse{}
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				if output.Text != "" {
					output.Text += "\n"
				}
				output.Text += strings.TrimSpace(block.Text)
			}
		case "tool_use":
			output.ToolCalls = append(output.ToolCalls, runtimeToolCall{
				ID:        strings.TrimSpace(block.ID),
				Name:      strings.TrimSpace(block.Name),
				Arguments: normalizeToolArguments(block.Input),
			})
		}
	}
	return output, nil
}

func (r *EinoGenTestRuntime) executeTool(ctx context.Context, toolCtx *genTestToolCallContext, call runtimeToolCall) (*genTestToolResult, error) {
	if toolCtx.MCP != nil {
		if result, ok := toolCtx.MCP.Invoke(ctx, call.Name, call.Arguments); ok {
			return &genTestToolResult{Content: result}, nil
		}
	}

	switch call.Name {
	case "Read":
		path, err := requiredString(call.Arguments, "path")
		if err != nil {
			return nil, err
		}
		return r.runRead(toolCtx.Workspace, path)
	case "Glob":
		pattern, err := requiredString(call.Arguments, "pattern")
		if err != nil {
			return nil, err
		}
		return r.runGlob(toolCtx.Workspace.RepoDir, pattern)
	case "Grep":
		pattern, err := requiredString(call.Arguments, "pattern")
		if err != nil {
			return nil, err
		}
		scope := stringArg(call.Arguments, "path", ".")
		maxMatches := intArg(call.Arguments, "max_matches", 50)
		return r.runGrep(toolCtx.Workspace.RepoDir, pattern, scope, maxMatches)
	case "Write":
		path, err := requiredString(call.Arguments, "path")
		if err != nil {
			return nil, err
		}
		content, err := requiredString(call.Arguments, "content")
		if err != nil {
			return nil, err
		}
		relativePath := normalizeRepoRelativePath(path)
		if err := writeRepoFile(toolCtx.Workspace.RepoDir, relativePath, content); err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: fmt.Sprintf("wrote %s (%d bytes)", relativePath, len(content))}, nil
	case "Edit":
		return r.runEdit(toolCtx, call.Arguments)
	case "PowerShell":
		if runtime.GOOS != "windows" {
			return nil, fmt.Errorf("当前运行平台为 %s，禁止使用 PowerShell，请改用 Bash", runtime.GOOS)
		}
		command, err := requiredString(call.Arguments, "command")
		if err != nil {
			return nil, err
		}
		return r.runCommand(ctx, toolCtx, "powershell", []string{"-NoProfile", "-Command", command})
	case "Bash":
		if runtime.GOOS == "windows" {
			return nil, fmt.Errorf("当前运行平台为 Windows，禁止使用 Bash，请改用 PowerShell")
		}
		command, err := requiredString(call.Arguments, "command")
		if err != nil {
			return nil, err
		}
		return r.runCommand(ctx, toolCtx, "bash", []string{"-lc", command})
	case "AskUserQuestion":
		question, err := requiredString(call.Arguments, "question")
		if err != nil {
			return nil, err
		}
		answer, err := r.waitForInteraction(ctx, uint(toolCtx.Task.ID), "ai_question", question)
		if err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: answer}, nil
	case "RequestPermission":
		action, err := requiredString(call.Arguments, "action")
		if err != nil {
			return nil, err
		}
		reason := stringArg(call.Arguments, "reason", "")
		reply, err := r.waitForInteraction(ctx, uint(toolCtx.Task.ID), "permission_request", strings.TrimSpace(action+"\n"+reason))
		if err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: reply}, nil
	case "SubmitGenTestResult":
		output, err := buildGenTestOutput(call.Arguments)
		if err != nil {
			return nil, err
		}
		return &genTestToolResult{
			Content:     "gen-test result accepted",
			FinalOutput: output,
		}, nil
	default:
		return nil, fmt.Errorf("未知工具: %s", call.Name)
	}
}

func (r *EinoGenTestRuntime) runRead(workspace *RuntimeWorkspace, path string) (*genTestToolResult, error) {
	fullPath, displayPath, err := resolveReadableToolPath(workspace, path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &genTestToolResult{Content: fmt.Sprintf("path not found: %s\nUse Glob to discover existing files before reading.", displayPath)}, nil
		}
		return nil, fmt.Errorf("读取产物路径失败: %w", err)
	}
	if info.IsDir() {
		content, err := listRepoDirectoryForRead(fullPath, displayPath)
		if err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: content}, nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取产物文件失败: %w", err)
	}
	return &genTestToolResult{Content: truncateByBytes(string(data), defaultGenTestReadMaxBytes)}, nil
}

func resolveReadableToolPath(workspace *RuntimeWorkspace, path string) (string, string, error) {
	if workspace == nil {
		return "", "", fmt.Errorf("运行时工作区不能为空")
	}
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		trimmedPath = "."
	}
	if filepath.IsAbs(trimmedPath) {
		cleanRoot, err := filepath.Abs(workspace.RootDir)
		if err != nil {
			return "", "", fmt.Errorf("解析运行时工作区失败: %w", err)
		}
		cleanTarget, err := filepath.Abs(trimmedPath)
		if err != nil {
			return "", "", fmt.Errorf("解析读取路径失败: %w", err)
		}
		rootPrefix := cleanRoot + string(filepath.Separator)
		if cleanTarget != cleanRoot && !strings.HasPrefix(cleanTarget, rootPrefix) {
			return "", "", fmt.Errorf("读取路径非法，绝对路径仅允许位于运行时工作区内: %s", trimmedPath)
		}
		return cleanTarget, cleanTarget, nil
	}

	relativePath := normalizeRepoRelativePath(trimmedPath)
	fullPath, err := resolveRepoToolPath(workspace.RepoDir, relativePath)
	if err != nil {
		return "", "", err
	}
	return fullPath, relativePath, nil
}

func resolveRepoToolPath(repoDir, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		relativePath = "."
	}
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("仓库路径必须是相对路径: %s", relativePath)
	}

	cleanRepo, err := filepath.Abs(repoDir)
	if err != nil {
		return "", fmt.Errorf("解析仓库目录失败: %w", err)
	}
	targetPath := filepath.Join(cleanRepo, filepath.FromSlash(relativePath))
	cleanTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("解析仓库路径失败: %w", err)
	}

	repoPrefix := cleanRepo + string(filepath.Separator)
	if cleanTarget != cleanRepo && !strings.HasPrefix(cleanTarget, repoPrefix) {
		return "", fmt.Errorf("仓库路径非法: %s", relativePath)
	}
	return cleanTarget, nil
}

func listRepoDirectoryForRead(dir, relativePath string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("读取产物目录失败: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 200 {
		names = append(names[:200], fmt.Sprintf("... %d more entries", len(entries)-200))
	}
	if len(names) == 0 {
		return fmt.Sprintf("directory: %s\n(empty)", relativePath), nil
	}
	return fmt.Sprintf("directory: %s\n%s", relativePath, strings.Join(names, "\n")), nil
}

func compileRepoGlob(pattern string) (func(string) bool, error) {
	normalized := normalizeRepoRelativePath(pattern)
	if normalized == "" {
		return nil, fmt.Errorf("Glob pattern 不能为空")
	}
	var builder strings.Builder
	builder.WriteString("^")
	for i := 0; i < len(normalized); {
		switch {
		case strings.HasPrefix(normalized[i:], "**/"):
			builder.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(normalized[i:], "**"):
			builder.WriteString(".*")
			i += 2
		default:
			ch := normalized[i]
			switch ch {
			case '*':
				builder.WriteString("[^/]*")
			case '?':
				builder.WriteString("[^/]")
			default:
				builder.WriteString(regexp.QuoteMeta(string(ch)))
			}
			i++
		}
	}
	builder.WriteString("$")
	re, err := regexp.Compile(builder.String())
	if err != nil {
		return nil, fmt.Errorf("Glob pattern 无效: %w", err)
	}
	return re.MatchString, nil
}

func shouldSkipRepoSearchDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".pnpm-store", "dist", "build", "coverage", ".next", ".nuxt":
		return true
	default:
		return false
	}
}

func looksBinary(data []byte) bool {
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func (r *EinoGenTestRuntime) runGlob(repoDir, pattern string) (*genTestToolResult, error) {
	matcher, err := compileRepoGlob(pattern)
	if err != nil {
		return nil, err
	}
	matches := make([]string, 0, 200)
	err = filepath.WalkDir(repoDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && shouldSkipRepoSearchDir(name) && path != repoDir {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return nil
		}
		normalized := filepath.ToSlash(relPath)
		if matcher(normalized) {
			matches = append(matches, normalized)
		}
		if len(matches) >= 200 {
			return errStopRepoWalk
		}
		return nil
	})
	if err != nil && err != errStopRepoWalk {
		return nil, fmt.Errorf("Glob 执行失败: %w", err)
	}
	sort.Strings(matches)
	return &genTestToolResult{Content: strings.Join(matches, "\n")}, nil
}

func (r *EinoGenTestRuntime) runGrep(repoDir, pattern, scope string, maxMatches int) (*genTestToolResult, error) {
	if maxMatches <= 0 {
		maxMatches = 50
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("Grep pattern 不是有效正则: %w", err)
	}
	scopePath, err := resolveRepoToolPath(repoDir, normalizeRepoRelativePath(scope))
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(scopePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &genTestToolResult{Content: fmt.Sprintf("path not found: %s", normalizeRepoRelativePath(scope))}, nil
		}
		return nil, fmt.Errorf("读取搜索范围失败: %w", err)
	}

	matches := make([]string, 0, maxMatches)
	visitFile := func(path string) {
		if len(matches) >= maxMatches {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil || looksBinary(data) {
			return
		}
		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		for index, line := range lines {
			if re.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(relPath), index+1, line))
				if len(matches) >= maxMatches {
					return
				}
			}
		}
	}

	if !info.IsDir() {
		visitFile(scopePath)
		return &genTestToolResult{Content: truncateByBytes(strings.Join(matches, "\n"), defaultGenTestToolResultMaxBytes)}, nil
	}

	err = filepath.WalkDir(scopePath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && shouldSkipRepoSearchDir(name) && path != scopePath {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		visitFile(path)
		if len(matches) >= maxMatches {
			return errStopRepoWalk
		}
		return nil
	})
	if err != nil && err != errStopRepoWalk {
		return nil, fmt.Errorf("Grep 执行失败: %w", err)
	}
	return &genTestToolResult{Content: truncateByBytes(strings.Join(matches, "\n"), defaultGenTestToolResultMaxBytes)}, nil
}

func (r *EinoGenTestRuntime) runEdit(toolCtx *genTestToolCallContext, args map[string]any) (*genTestToolResult, error) {
	path, err := requiredString(args, "path")
	if err != nil {
		return nil, err
	}
	oldString, err := requiredString(args, "old_string")
	if err != nil {
		return nil, err
	}
	newString, err := requiredString(args, "new_string")
	if err != nil {
		return nil, err
	}
	replaceAll := boolArg(args, "replace_all", false)

	relativePath := normalizeRepoRelativePath(path)
	content, err := readRepoFile(toolCtx.Workspace.RepoDir, relativePath)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(content, oldString) {
		return nil, fmt.Errorf("Edit 未找到目标片段: %s", truncateText(oldString, 80))
	}
	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldString, newString)
	} else {
		updated = strings.Replace(content, oldString, newString, 1)
	}
	if err := writeRepoFile(toolCtx.Workspace.RepoDir, relativePath, updated); err != nil {
		return nil, err
	}
	return &genTestToolResult{Content: fmt.Sprintf("edited %s", relativePath)}, nil
}

func (r *EinoGenTestRuntime) runCommand(
	ctx context.Context,
	toolCtx *genTestToolCallContext,
	program string,
	args []string,
) (*genTestToolResult, error) {
	if _, err := exec.LookPath(program); err != nil {
		return nil, fmt.Errorf("命令不可用: %s，请确认运行镜像已安装该命令: %w", program, err)
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(callCtx, program, args...)
	cmd.Dir = toolCtx.Workspace.RepoDir
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(truncateByBytes(string(output), defaultGenTestToolResultMaxBytes))
	if err != nil {
		if result == "" {
			result = "(no output)"
		}
		return &genTestToolResult{
			Content: fmt.Sprintf("%s failed: %v\n%s", program, err, result),
			IsError: true,
		}, nil
	}
	if result == "" {
		result = fmt.Sprintf("%s completed successfully with no output", program)
	}
	return &genTestToolResult{Content: result}, nil
}

func (r *EinoGenTestRuntime) waitForInteraction(ctx context.Context, taskID uint, interactionType, content string) (string, error) {
	metadata, _ := json.Marshal(map[string]any{
		"source": "eino_runtime",
	})
	interaction := &model.CLIInteraction{
		TaskID:          taskID,
		InteractionType: interactionType,
		Content:         content,
		Status:          "pending",
		Metadata:        model.JSON(metadata),
	}
	if err := r.interactionRepo.Create(interaction); err != nil {
		return "", fmt.Errorf("创建交互记录失败: %w", err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			current, err := r.interactionRepo.GetByID(interaction.ID)
			if err != nil {
				return "", err
			}
			switch current.Status {
			case "answered", "approved":
				if strings.TrimSpace(current.UserResponse) == "" && current.Status == "approved" {
					return "approved", nil
				}
				return strings.TrimSpace(current.UserResponse), nil
			case "rejected":
				return "", fmt.Errorf("用户拒绝: %s", strings.TrimSpace(current.UserResponse))
			}
		}
	}
}

func availableCommandToolNames(goos string) []string {
	if goos == "windows" {
		return []string{"PowerShell"}
	}
	return []string{"Bash"}
}

func commandToolSpecs(goos string) []genTestToolSpec {
	if goos == "windows" {
		return []genTestToolSpec{
			{
				Name:        "PowerShell",
				Description: "Execute a PowerShell command in the repository root. Only available on Windows runtime.",
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string"},
					},
					"required": []string{"command"},
				},
			},
		}
	}
	return []genTestToolSpec{
		{
			Name:        "Bash",
			Description: "Execute a bash command in the repository root. Only available on Linux/macOS runtime; use this in Docker/Linux deployments.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
}

func (r *EinoGenTestRuntime) baseToolSpecs() []genTestToolSpec {
	specs := []genTestToolSpec{
		{
			Name:        "Read",
			Description: "Read a file by repository-relative path, or by absolute path within the current runtime workspace. If the path is a directory, returns its immediate entries. If it does not exist, returns a not-found hint; use Glob to discover repository files.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "Glob",
			Description: "Find repository files by glob pattern, for example **/*.spec.ts.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "Grep",
			Description: "Search repository file contents with a regular expression.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern":     map[string]any{"type": "string"},
					"path":        map[string]any{"type": "string"},
					"max_matches": map[string]any{"type": "integer"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "Write",
			Description: "Write a repository file by relative path.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string"},
					"content": map[string]any{"type": "string"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "Edit",
			Description: "Replace one snippet in a repository file.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":        map[string]any{"type": "string"},
					"old_string":  map[string]any{"type": "string"},
					"new_string":  map[string]any{"type": "string"},
					"replace_all": map[string]any{"type": "boolean"},
				},
				"required": []string{"path", "old_string", "new_string"},
			},
		},
	}
	specs = append(specs, commandToolSpecs(runtime.GOOS)...)
	specs = append(specs, []genTestToolSpec{
		{
			Name:        "AskUserQuestion",
			Description: "Ask the user for missing information and wait for the reply.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string"},
				},
				"required": []string{"question"},
			},
		},
		{
			Name:        "RequestPermission",
			Description: "Request explicit user permission before a risky action.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{"type": "string"},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "SubmitGenTestResult",
			Description: "Submit the final structured gen-test result after files are written and self-test is done.",
			Schema: map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"properties": map[string]any{
					"test_cases":  map[string]any{"type": "array"},
					"test_script": map[string]any{"type": "object"},
					"test_doc":    map[string]any{"type": "object"},
					"self_test":   map[string]any{"type": "object"},
					"summary":     map[string]any{"type": "string"},
				},
				"required": []string{"test_cases", "test_script", "test_doc", "summary"},
			},
		},
	}...)
	return specs
}

func buildOpenAIHistory(history []runtimeMessage) []map[string]any {
	result := make([]map[string]any, 0, len(history))
	for _, msg := range history {
		switch msg.Role {
		case "system", "user":
			result = append(result, map[string]any{
				"role":    msg.Role,
				"content": msg.Text,
			})
		case "assistant":
			item := map[string]any{
				"role":    "assistant",
				"content": msg.Text,
			}
			if len(msg.ToolCalls) > 0 {
				toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
				for _, call := range msg.ToolCalls {
					argBytes, _ := json.Marshal(call.Arguments)
					toolCalls = append(toolCalls, map[string]any{
						"id":   call.ID,
						"type": "function",
						"function": map[string]any{
							"name":      call.Name,
							"arguments": string(argBytes),
						},
					})
				}
				item["tool_calls"] = toolCalls
			}
			result = append(result, item)
		case "tool":
			result = append(result, map[string]any{
				"role":         "tool",
				"tool_call_id": msg.ToolCallID,
				"content":      msg.ToolResult,
			})
		}
	}
	return result
}

func buildAnthropicHistory(history []runtimeMessage) (string, []map[string]any) {
	systemParts := make([]string, 0, 2)
	messages := make([]map[string]any, 0, len(history))
	for _, msg := range history {
		switch msg.Role {
		case "system":
			if strings.TrimSpace(msg.Text) != "" {
				systemParts = append(systemParts, msg.Text)
			}
		case "user":
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": msg.Text},
				},
			})
		case "assistant":
			content := make([]map[string]any, 0, 1+len(msg.ToolCalls))
			if strings.TrimSpace(msg.Text) != "" {
				content = append(content, map[string]any{
					"type": "text",
					"text": msg.Text,
				})
			}
			for _, call := range msg.ToolCalls {
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    call.ID,
					"name":  call.Name,
					"input": call.Arguments,
				})
			}
			messages = append(messages, map[string]any{
				"role":    "assistant",
				"content": content,
			})
		case "tool":
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": msg.ToolCallID,
						"content":     msg.ToolResult,
						"is_error":    msg.ToolError,
					},
				},
			})
		}
	}
	return strings.Join(systemParts, "\n\n"), messages
}

func buildOpenAIToolPayloads(tools []genTestToolSpec) []map[string]any {
	payloads := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		payloads = append(payloads, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Schema,
			},
		})
	}
	return payloads
}

func buildAnthropicToolPayloads(tools []genTestToolSpec) []map[string]any {
	payloads := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		payloads = append(payloads, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.Schema,
		})
	}
	return payloads
}

func buildGenTestOutput(args map[string]any) (*GenTestOutput, error) {
	normalizedArgs := normalizeGenTestOutputArgs(args)
	validationErrs := validateNormalizedGenTestOutputArgs(normalizedArgs)
	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("SubmitGenTestResult 参数无效: %s", strings.Join(validationErrs, "; "))
	}
	data, err := json.Marshal(normalizedArgs)
	if err != nil {
		return nil, err
	}
	var output GenTestOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func normalizeGenTestOutputArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	normalized, ok := deepCloneAny(args).(map[string]any)
	if !ok || normalized == nil {
		return map[string]any{}
	}

	if testCases, ok := normalized["test_cases"].([]any); ok {
		normalizedCases := make([]any, 0, len(testCases))
		for _, item := range testCases {
			caseMap, ok := item.(map[string]any)
			if !ok {
				normalizedCases = append(normalizedCases, item)
				continue
			}
			cloneCase, _ := deepCloneAny(caseMap).(map[string]any)
			if cloneCase == nil {
				cloneCase = map[string]any{}
			}
			aliasStringField(cloneCase, "title", "case_name", "name")
			aliasStringField(cloneCase, "category", "module", "case_type")
			aliasStringField(cloneCase, "precondition", "prerequisites", "preconditions")
			aliasStringField(cloneCase, "expected", "expected_result", "expected_results", "result")
			aliasStringField(cloneCase, "self_test_result", "status", "self_result")
			aliasIntField(cloneCase, "priority", "severity_level", "level")
			if rawSteps, exists := cloneCase["steps"]; exists {
				cloneCase["steps"] = normalizeStepValue(rawSteps)
			}
			normalizedCases = append(normalizedCases, cloneCase)
		}
		normalized["test_cases"] = normalizedCases
	}

	if testScript, ok := normalized["test_script"].(map[string]any); ok {
		cloneScript, _ := deepCloneAny(testScript).(map[string]any)
		if cloneScript == nil {
			cloneScript = map[string]any{}
		}
		aliasStringField(cloneScript, "file_path", "path", "script_path")
		aliasStringField(cloneScript, "file_content", "content", "script_content")
		aliasStringField(cloneScript, "language", "lang")
		normalized["test_script"] = cloneScript
	}

	if testDoc, ok := normalized["test_doc"].(map[string]any); ok {
		cloneDoc, _ := deepCloneAny(testDoc).(map[string]any)
		if cloneDoc == nil {
			cloneDoc = map[string]any{}
		}
		aliasStringField(cloneDoc, "file_path", "path", "doc_path")
		aliasStringField(cloneDoc, "content", "file_content", "doc_content")
		aliasStringField(cloneDoc, "title", "name", "doc_title")
		normalized["test_doc"] = cloneDoc
	}

	return normalized
}

func validateNormalizedGenTestOutputArgs(args map[string]any) []string {
	errs := make([]string, 0, 8)

	testCasesRaw, ok := args["test_cases"]
	if !ok {
		return append(errs, "缺少 test_cases")
	}
	testCases, ok := testCasesRaw.([]any)
	if !ok {
		return append(errs, "test_cases 必须是数组")
	}
	for index, item := range testCases {
		caseMap, ok := item.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("test_cases[%d] 必须是对象", index))
			continue
		}
		if strings.TrimSpace(stringArg(caseMap, "title", "")) == "" {
			errs = append(errs, fmt.Sprintf("test_cases[%d].title 不能为空", index))
		}
		if strings.TrimSpace(stringArg(caseMap, "steps", "")) == "" {
			errs = append(errs, fmt.Sprintf("test_cases[%d].steps 不能为空", index))
		}
		if strings.TrimSpace(stringArg(caseMap, "expected", "")) == "" {
			errs = append(errs, fmt.Sprintf("test_cases[%d].expected 不能为空", index))
		}
	}

	if testScript, ok := args["test_script"].(map[string]any); ok {
		if strings.TrimSpace(stringArg(testScript, "file_path", "")) == "" && strings.TrimSpace(stringArg(testScript, "file_content", "")) == "" {
			errs = append(errs, "test_script.file_path 和 test_script.file_content 不能同时为空")
		}
	} else {
		errs = append(errs, "test_script 必须是对象")
	}

	if testDoc, ok := args["test_doc"].(map[string]any); ok {
		if strings.TrimSpace(stringArg(testDoc, "file_path", "")) == "" && strings.TrimSpace(stringArg(testDoc, "content", "")) == "" {
			errs = append(errs, "test_doc.file_path 和 test_doc.content 不能同时为空")
		}
	} else {
		errs = append(errs, "test_doc 必须是对象")
	}

	if strings.TrimSpace(stringArg(args, "summary", "")) == "" {
		errs = append(errs, "summary 不能为空")
	}

	return errs
}

func aliasStringField(target map[string]any, canonical string, aliases ...string) {
	if strings.TrimSpace(stringArg(target, canonical, "")) != "" {
		return
	}
	for _, alias := range aliases {
		if value := strings.TrimSpace(stringArg(target, alias, "")); value != "" {
			target[canonical] = value
			return
		}
	}
}

func aliasIntField(target map[string]any, canonical string, aliases ...string) {
	if _, exists := target[canonical]; exists {
		return
	}
	for _, alias := range aliases {
		if value, exists := target[alias]; exists && value != nil {
			target[canonical] = intArg(target, alias, 0)
			return
		}
	}
}

func normalizeStepValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for index, item := range v {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%d. %s", index+1, text))
		}
		return strings.Join(parts, "\n")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func deepCloneAny(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(data, &cloned); err != nil {
		return value
	}
	return cloned
}

func parseGenTestOutputFromText(text string) (*GenTestOutput, error) {
	candidate := extractJSONObject(text)
	if candidate == "" {
		return nil, fmt.Errorf("未找到 JSON 对象")
	}
	var output GenTestOutput
	if err := json.Unmarshal([]byte(candidate), &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func extractJSONObject(text string) string {
	re := regexp.MustCompile(`(?s)\{.*\}`)
	return strings.TrimSpace(re.FindString(text))
}

func requiredString(args map[string]any, key string) (string, error) {
	value := strings.TrimSpace(stringArg(args, key, ""))
	if value == "" {
		return "", fmt.Errorf("缺少参数: %s", key)
	}
	return value, nil
}

func stringArg(args map[string]any, key, fallback string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func intArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func normalizeToolArguments(input any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	switch v := input.(type) {
	case map[string]any:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return map[string]any{}
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			return map[string]any{}
		}
		return parsed
	}
}

func compactLines(value string, maxLines int) []string {
	rawLines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	lines := make([]string, 0, minRuntimeInt(len(rawLines), maxLines))
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
		if len(lines) >= maxLines {
			break
		}
	}
	return lines
}

func truncateByBytes(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n...truncated..."
}

func truncateToolResult(value string) string {
	return truncateByBytes(strings.TrimSpace(value), defaultGenTestToolResultMaxBytes)
}

func marshalTaskEventJSON(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (r *EinoGenTestRuntime) publish(taskID uint64, eventType, stage, status, message string, data map[string]any) {
	if r.eventHub != nil {
		r.eventHub.Publish(taskID, TaskEvent{
			Type:      eventType,
			Stage:     stage,
			Status:    status,
			Message:   message,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}

func (r *EinoGenTestRuntime) publishAssistantText(taskID uint64, modelName, text string) {
	payload := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"model": modelName,
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		},
	}
	r.publish(taskID, taskEventTypeLog, "cli_output", model.TaskStatusRunning, marshalTaskEventJSON(payload), nil)
}

func (r *EinoGenTestRuntime) publishAssistantToolCall(taskID uint64, name string, input map[string]any) {
	payload := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []map[string]any{
				{
					"type":  "tool_use",
					"name":  name,
					"input": input,
				},
			},
		},
	}
	r.publish(taskID, taskEventTypeLog, "cli_output_raw", model.TaskStatusRunning, marshalTaskEventJSON(payload), nil)
}

func (r *EinoGenTestRuntime) publishToolResult(taskID uint64, content string, isError bool) {
	payload := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []map[string]any{
				{
					"type":     "tool_result",
					"content":  truncateToolResult(content),
					"is_error": isError,
				},
			},
		},
	}
	stage := "cli_output_raw"
	if !isError {
		stage = "cli_output"
	}
	r.publish(taskID, taskEventTypeLog, stage, model.TaskStatusRunning, marshalTaskEventJSON(payload), nil)
}

func minRuntimeInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
