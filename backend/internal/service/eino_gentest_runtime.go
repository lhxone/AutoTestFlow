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

	openaiacl "github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/reduction"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
	"go.uber.org/zap"
)

const (
	defaultGenTestTurnLimit          = 1000
	defaultGenTestToolResultMaxBytes = 12_000
	defaultGenTestReadMaxBytes       = 64_000
	defaultGenTestHistoryMaxBytes    = 90_000
	defaultGenTestInitialPromptBytes = 70_000
	defaultGenTestRecentMessageKeep  = 12
	defaultGenTestRepairLimit        = 2
	defaultGenTestSelfTestTimeout    = 5 * time.Minute
	genTestContextSummaryPrefix      = "## AutoTestFlow 历史上下文压缩摘要"
	chromeProfilePath                = "/tmp/auto-test-flow/chrome-profile"
)

var errStopRepoWalk = errors.New("stop repo walk")

// cleanupChromeProfile 清理 Chrome MCP 占用的 profile 目录
// 返回值: (状态, 错误信息)
// 状态: "not_exist" 目录不存在, "cleaned" 清理成功, "error" 清理失败
func cleanupChromeProfile() (status string, errMsg string) {
	if _, err := os.Stat(chromeProfilePath); os.IsNotExist(err) {
		return "not_exist", ""
	}
	if err := os.RemoveAll(chromeProfilePath); err != nil {
		return "error", err.Error()
	}
	return "cleaned", ""
}

type EinoGenTestRuntime struct {
	logger          *zap.Logger
	eventHub        *TaskEventHub
	interactionRepo *repository.CLIInteractionRepo
	workspace       *GenTestWorkspaceService
	httpClient      *http.Client
}

type genTestToolCallContext struct {
	Task       *model.TestTask
	Input      *GenTestInput
	Workflow   *model.Skill
	Agent      *model.Agent
	Workspace  *RuntimeWorkspace
	MCP        *MCPRuntime
	RAGContext string
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

	// 清理 Chrome MCP 占用的 profile 目录
	status, errMsg := cleanupChromeProfile()
	switch status {
	case "cleaned":
		r.publish(task.ID, taskEventTypeLog, "chrome_profile_cleanup", model.TaskStatusRunning,
			"已清理 Chrome profile 目录", map[string]any{"path": chromeProfilePath})
	case "error":
		r.publish(task.ID, taskEventTypeLog, "chrome_profile_cleanup", model.TaskStatusRunning,
			fmt.Sprintf("清理 Chrome profile 目录失败: %s", errMsg), map[string]any{"path": chromeProfilePath, "error": errMsg})
	case "not_exist":
		r.publish(task.ID, taskEventTypeLog, "chrome_profile_cleanup", model.TaskStatusRunning,
			"Chrome profile 目录不存在，跳过清理", map[string]any{"path": chromeProfilePath})
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

	ragContext := r.retrieveRAGContext(ctx, task, input, workflow)
	prompt := r.buildPrompt(workspace, task, input, workflow, agent, promptCtx, ragContext)
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

	adkCfg := resolveGenTestADKConfig(workflow, agent)
	runtimeLabel := "Eino 原生运行时"
	runtimeType := "eino"
	if adkCfg.Enabled {
		runtimeLabel = "Eino ADK 运行时"
		runtimeType = "adk"
	}
	r.publish(task.ID, taskEventTypeStage, "runtime_started", model.TaskStatusRunning,
		fmt.Sprintf("开始执行 %s\n  provider: %s\n  model: %s", runtimeLabel, execCfg.Provider, execCfg.Model),
		map[string]any{
			"provider":     execCfg.Provider,
			"model":        execCfg.Model,
			"runtime_type": runtimeType,
		})

	agentCtx := &genTestToolCallContext{
		Task:       task,
		Input:      input,
		Workflow:   workflow,
		Agent:      agent,
		Workspace:  workspace,
		MCP:        mcpRuntime,
		RAGContext: ragContext,
	}
	var output *GenTestOutput
	if adkCfg.Enabled {
		output, err = r.runADKAgentLoop(ctx, execCfg, agentCtx, promptCtx, adkCfg)
	} else {
		output, err = r.runAgentLoop(ctx, execCfg, agentCtx, promptCtx)
	}
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

func (r *EinoGenTestRuntime) retrieveRAGContext(ctx context.Context, task *model.TestTask, input *GenTestInput, workflow *model.Skill) string {
	if task == nil || input == nil {
		return ""
	}
	queryParts := []string{
		input.ProjectName,
		input.IssueTitle,
		input.IssueDesc,
		input.IssueSeverity,
	}
	query := strings.TrimSpace(strings.Join(queryParts, "\n"))
	if query == "" {
		return ""
	}
	knowledgeService := NewKnowledgeService(r.logger)
	contextText, err := knowledgeService.RetrieveContextForGeneration(ctx, task.ProjectID, query, workflow)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("RAG 检索失败，降级为无知识库模式",
				zap.Uint64("task_id", task.ID),
				zap.Uint64("project_id", task.ProjectID),
				zap.Error(err))
		}
		r.publish(task.ID, taskEventTypeLog, "rag_retrieve", model.TaskStatusRunning,
			fmt.Sprintf("RAG 检索失败，已降级为无知识库模式: %v", err), nil)
		return ""
	}
	if strings.TrimSpace(contextText) != "" {
		r.publish(task.ID, taskEventTypeLog, "rag_retrieve", model.TaskStatusRunning, "已注入 RAG 知识库检索上下文", nil)
	}
	return contextText
}

func (r *EinoGenTestRuntime) buildPrompt(
	workspace *RuntimeWorkspace,
	task *model.TestTask,
	input *GenTestInput,
	workflow *model.Skill,
	agent *model.Agent,
	promptCtx *CLIPromptContext,
	ragContext string,
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

	ragNote := ""
	if strings.TrimSpace(ragContext) != "" {
		ragNote = "\n" + strings.TrimSpace(ragContext) + "\n"
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
- 必须把实际文件写入仓库目录；测试脚本必须使用 WriteTestScript 写入，测试文档必须使用 WriteTestDoc 写入，以便运行时同步结果草稿。
- 完成后必须调用 SubmitGenTestResult 工具提交结构化结果，而不是只输出自然语言。
- 自测完成后，将 Playwright 报告路径写入 self_test.playwright.report_path（例如 "playwright-report/index.html"）。
- SubmitGenTestResult 必须严格使用标准字段名，禁止使用 path、case_name、prerequisites、module、status、expected_result 等别名字段。
- 若关键信息缺失，可调用 AskUserQuestion；若需要人工许可，可调用 RequestPermission。
- 必须根据当前运行平台选择命令工具；当前平台: %s；可用命令工具: %s。
- WriteTestScript 和 WriteTestDoc 会自动更新结果草稿；编辑已登记的测试脚本或文档后运行时也会刷新草稿。

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
%s%s%s%s`,
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
		ragNote,
	)
}

func (r *EinoGenTestRuntime) runAgentLoop(
	ctx context.Context,
	cfg AgentExecutionConfig,
	toolCtx *genTestToolCallContext,
	promptCtx *CLIPromptContext,
) (*GenTestOutput, error) {
	systemPrompt := "你是 AutoTestFlow 的测试生成代理。严格使用工具探索事实，少猜测，多验证。完成所有动作后调用 SubmitGenTestResult。"
	userPrompt := r.buildPrompt(toolCtx.Workspace, toolCtx.Task, toolCtx.Input, toolCtx.Workflow, toolCtx.Agent, promptCtx, toolCtx.RAGContext)

	history := []runtimeMessage{
		{Role: "system", Text: systemPrompt},
		{Role: "user", Text: userPrompt},
	}
	if estimateRuntimeHistoryBytes(history) > defaultGenTestHistoryMaxBytes {
		compactedPrompt := compactInitialUserPrompt(userPrompt, defaultGenTestInitialPromptBytes)
		if compactedPrompt != userPrompt {
			history[1].Text = compactedPrompt
			r.publish(toolCtx.Task.ID, taskEventTypeLog, "initial_context_compacted", model.TaskStatusRunning,
				fmt.Sprintf("初始 Prompt 已裁剪以避免首轮模型上下文过大，当前估算大小 %d bytes", estimateRuntimeHistoryBytes(history)),
				map[string]any{
					"estimated_bytes": estimateRuntimeHistoryBytes(history),
					"limit_bytes":     defaultGenTestInitialPromptBytes,
				})
		}
	}
	toolSpecs := r.baseToolSpecs()
	toolNames := make(map[string]struct{}, len(toolSpecs))
	for _, tool := range toolSpecs {
		toolNames[tool.Name] = struct{}{}
	}
	if toolCtx.MCP != nil {
		for _, tool := range toolCtx.MCP.OpenAITools() {
			fn, ok := tool["function"].(map[string]any)
			if !ok {
				continue
			}
			name, _ := fn["name"].(string)
			description, _ := fn["description"].(string)
			parameters, _ := fn["parameters"].(map[string]any)
			if strings.TrimSpace(name) == "" {
				continue
			}
			if _, exists := toolNames[name]; exists {
				r.publish(toolCtx.Task.ID, taskEventTypeLog, "mcp_tool_skipped", model.TaskStatusRunning,
					fmt.Sprintf("MCP 工具 %s 与内置工具重名，已跳过以保证内置工具优先", name), nil)
				continue
			}
			toolNames[name] = struct{}{}
			toolSpecs = append(toolSpecs, genTestToolSpec{
				Name:        name,
				Description: description,
				Schema:      parameters,
			})
		}
	}

	repairAttempts := 0
	for turn := 1; turn <= defaultGenTestTurnLimit; turn++ {
		r.publish(toolCtx.Task.ID, taskEventTypeLog, "cli_output_raw", model.TaskStatusRunning, marshalTaskEventJSON(map[string]any{
			"type":    "system",
			"message": fmt.Sprintf("模型回合 %d", turn),
		}), nil)

		var compacted bool
		var compactedCount int
		history, compacted, compactedCount = compactRuntimeHistory(ctx, history, defaultGenTestHistoryMaxBytes, defaultGenTestRecentMessageKeep)
		if compacted {
			r.publish(toolCtx.Task.ID, taskEventTypeLog, "context_compacted", model.TaskStatusRunning,
				fmt.Sprintf("已通过 Eino reduction 自动压缩历史上下文，处理旧消息 %d 条，当前估算大小 %d bytes", compactedCount, estimateRuntimeHistoryBytes(history)),
				map[string]any{
					"compacted_messages": compactedCount,
					"estimated_bytes":    estimateRuntimeHistoryBytes(history),
					"engine":             "eino_reduction",
				})
		}

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
			report := r.runGeneratedSelfTest(ctx, toolCtx, finalOutput)
			finalOutput.SelfTest = report
			_ = r.workspace.WriteResultFile(toolCtx.Workspace, finalOutput)
			if report.Passed {
				return finalOutput, nil
			}
			if report.Command == "" || repairAttempts >= defaultGenTestRepairLimit {
				return finalOutput, nil
			}
			repairAttempts++
			history = append(history, runtimeMessage{
				Role: "user",
				Text: buildSelfTestRepairPrompt(report, repairAttempts, defaultGenTestRepairLimit),
			})
			r.publish(toolCtx.Task.ID, taskEventTypeLog, "self_test_repair", model.TaskStatusRunning,
				fmt.Sprintf("生成脚本真实自测失败，已请求模型修复 (%d/%d)", repairAttempts, defaultGenTestRepairLimit),
				map[string]any{
					"command":  report.Command,
					"exitCode": report.ExitCode,
				})
			continue
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
	chatModel, err := r.newEinoChatModel(ctx, cfg, tools)
	if err != nil {
		return nil, err
	}
	msg, err := chatModel.Generate(ctx, runtimeHistoryToEinoMessages(history))
	if err != nil {
		return nil, fmt.Errorf("Eino ChatModel 调用失败: %w", err)
	}
	return runtimeModelResponseFromEinoMessage(msg)
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

func (r *EinoGenTestRuntime) newEinoChatModel(
	ctx context.Context,
	cfg AgentExecutionConfig,
	tools []genTestToolSpec,
) (einomodel.ToolCallingChatModel, error) {
	var base einomodel.ToolCallingChatModel
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai", "zhipu", "custom":
		temperature := float32(cfg.Temperature)
		client, err := openaiacl.NewClient(ctx, &openaiacl.Config{
			APIKey:      cfg.APIKey,
			BaseURL:     normalizeOpenAICompatibleBaseURL(cfg.BaseURL),
			Model:       cfg.Model,
			MaxTokens:   &cfg.MaxTokens,
			Temperature: &temperature,
			HTTPClient:  r.httpClient,
		})
		if err != nil {
			return nil, fmt.Errorf("初始化 Eino OpenAI ChatModel 失败: %w", err)
		}
		base = client
	case "claude":
		base = &anthropicEinoChatModel{
			cfg:        cfg,
			httpClient: r.httpClient,
		}
	default:
		return nil, fmt.Errorf("不支持的模型提供商: %s", cfg.Provider)
	}

	toolInfos, err := buildEinoToolInfos(tools)
	if err != nil {
		return nil, err
	}
	if len(toolInfos) == 0 {
		return base, nil
	}
	chatModel, err := base.WithTools(toolInfos)
	if err != nil {
		return nil, fmt.Errorf("绑定 Eino 工具失败: %w", err)
	}
	return chatModel, nil
}

func runtimeModelResponseFromEinoMessage(msg *schema.Message) (*runtimeModelResponse, error) {
	if msg == nil {
		return nil, fmt.Errorf("模型服务返回空响应")
	}
	output := &runtimeModelResponse{Text: strings.TrimSpace(msg.Content)}
	for _, call := range msg.ToolCalls {
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

func buildEinoToolInfos(tools []genTestToolSpec) ([]*schema.ToolInfo, error) {
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		info := &schema.ToolInfo{
			Name: name,
			Desc: strings.TrimSpace(tool.Description),
		}
		if len(tool.Schema) > 0 {
			schemaBytes, err := json.Marshal(tool.Schema)
			if err != nil {
				return nil, fmt.Errorf("序列化工具 schema 失败 (%s): %w", tool.Name, err)
			}
			var js einojsonschema.Schema
			if err := json.Unmarshal(schemaBytes, &js); err != nil {
				return nil, fmt.Errorf("解析工具 schema 失败 (%s): %w", tool.Name, err)
			}
			info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&js)
		}
		toolInfos = append(toolInfos, info)
	}
	return toolInfos, nil
}

func normalizeOpenAICompatibleBaseURL(baseURL string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if endpoint == "" {
		return endpoint
	}
	lower := strings.ToLower(endpoint)
	trimmedChatCompletions := false
	if strings.HasSuffix(lower, "/chat/completions") {
		endpoint = strings.TrimRight(endpoint[:len(endpoint)-len("/chat/completions")], "/")
		lower = strings.ToLower(endpoint)
		trimmedChatCompletions = true
	}
	if trimmedChatCompletions || strings.HasSuffix(lower, "/v1") || strings.HasSuffix(lower, "/api/paas/v4") {
		return endpoint
	}
	return endpoint + "/v1"
}

type anthropicEinoChatModel struct {
	cfg        AgentExecutionConfig
	httpClient *http.Client
	tools      []*schema.ToolInfo
}

func (m *anthropicEinoChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	options := einomodel.GetCommonOptions(&einomodel.Options{}, opts...)
	cfg := m.cfg
	if options.Model != nil {
		cfg.Model = *options.Model
	}
	if options.MaxTokens != nil {
		cfg.MaxTokens = *options.MaxTokens
	}
	if options.Temperature != nil {
		cfg.Temperature = float64(*options.Temperature)
	}
	tools := m.tools
	if options.Tools != nil {
		tools = options.Tools
	}

	systemText, messages := buildAnthropicHistory(einoMessagesToRuntimeHistory(input))
	reqBody := map[string]any{
		"model":       cfg.Model,
		"system":      systemText,
		"messages":    messages,
		"tools":       buildAnthropicToolInfoPayloads(tools),
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxTokens,
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolveAnthropicMessagesEndpoint(cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构建 Anthropic 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	httpClient := m.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
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

	out := &schema.Message{Role: schema.Assistant}
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				if out.Content != "" {
					out.Content += "\n"
				}
				out.Content += strings.TrimSpace(block.Text)
			}
		case "tool_use":
			argBytes, _ := json.Marshal(normalizeToolArguments(block.Input))
			out.ToolCalls = append(out.ToolCalls, schema.ToolCall{
				ID:   strings.TrimSpace(block.ID),
				Type: "function",
				Function: schema.FunctionCall{
					Name:      strings.TrimSpace(block.Name),
					Arguments: string(argBytes),
				},
			})
		}
	}
	return out, nil
}

func (m *anthropicEinoChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("Anthropic Eino ChatModel 暂不支持流式调用")
}

func (m *anthropicEinoChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	next := *m
	next.tools = append([]*schema.ToolInfo(nil), tools...)
	return &next, nil
}

func buildAnthropicToolInfoPayloads(tools []*schema.ToolInfo) []map[string]any {
	payloads := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		inputSchema, err := tool.ToJSONSchema()
		if err != nil {
			inputSchema = nil
		}
		payloads = append(payloads, map[string]any{
			"name":         tool.Name,
			"description":  tool.Desc,
			"input_schema": inputSchema,
		})
	}
	return payloads
}

func (r *EinoGenTestRuntime) executeTool(ctx context.Context, toolCtx *genTestToolCallContext, call runtimeToolCall) (*genTestToolResult, error) {
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
	case "WriteTestScript":
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
		draft, err := readGenTestDraft(toolCtx.Workspace)
		if err != nil {
			return nil, err
		}
		draft.TestScript = GenTestScript{
			FilePath:    relativePath,
			FileContent: content,
			Language:    normalizeScriptLanguage(stringArg(call.Arguments, "language", ""), relativePath),
		}
		if strings.TrimSpace(draft.Summary) == "" {
			draft.Summary = "测试脚本草稿已更新"
		}
		if err := writeGenTestDraft(toolCtx.Workspace, draft); err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: fmt.Sprintf("wrote test script %s and updated result draft", relativePath)}, nil
	case "WriteTestDoc":
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
		draft, err := readGenTestDraft(toolCtx.Workspace)
		if err != nil {
			return nil, err
		}
		title := strings.TrimSpace(stringArg(call.Arguments, "title", ""))
		if title == "" && toolCtx.Input != nil {
			title = fmt.Sprintf("测试文档 - %s", toolCtx.Input.IssueTitle)
		}
		draft.TestDoc = GenTestDoc{
			Title:    title,
			FilePath: relativePath,
			Content:  content,
		}
		if strings.TrimSpace(draft.Summary) == "" {
			draft.Summary = "测试文档草稿已更新"
		}
		if err := writeGenTestDraft(toolCtx.Workspace, draft); err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: fmt.Sprintf("wrote test doc %s and updated result draft", relativePath)}, nil
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
		output, err := r.buildSubmittedGenTestOutput(toolCtx, call.Arguments)
		if err != nil {
			return nil, err
		}
		return &genTestToolResult{
			Content:     "gen-test result accepted",
			FinalOutput: output,
		}, nil
	default:
		if toolCtx.MCP != nil {
			if result, ok := toolCtx.MCP.Invoke(ctx, call.Name, call.Arguments); ok {
				return &genTestToolResult{Content: result}, nil
			}
		}
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
	if err := syncEditedArtifactDraft(toolCtx.Workspace, relativePath, updated); err != nil {
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
			Name:        "WriteTestScript",
			Description: "Write the generated test script and update the structured result draft.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":     map[string]any{"type": "string"},
					"content":  map[string]any{"type": "string"},
					"language": map[string]any{"type": "string"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "WriteTestDoc",
			Description: "Write the generated test document and update the structured result draft.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string"},
					"title":   map[string]any{"type": "string"},
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

func compactRuntimeHistory(ctx context.Context, history []runtimeMessage, maxBytes, recentKeep int) ([]runtimeMessage, bool, int) {
	if maxBytes <= 0 || estimateRuntimeHistoryBytes(history) <= maxBytes {
		return history, false, 0
	}
	if len(history) <= 3 {
		next := append([]runtimeMessage(nil), history...)
		changed := false
		for index := range next {
			if next[index].Role != "user" {
				continue
			}
			trimmed := compactInitialUserPrompt(next[index].Text, minRuntimeInt(maxBytes, defaultGenTestInitialPromptBytes))
			if trimmed != next[index].Text {
				next[index].Text = trimmed
				changed = true
			}
			break
		}
		if changed {
			return next, true, 1
		}
		return history, false, 0
	}
	if recentKeep < 4 {
		recentKeep = 4
	}

	reduced, reductionChanged, reductionCount := reduceRuntimeHistoryWithEino(ctx, history, maxBytes)
	if reductionChanged {
		history = reduced
		if estimateRuntimeHistoryBytes(history) <= maxBytes {
			return history, true, reductionCount
		}
	}

	head := append([]runtimeMessage(nil), history[:minRuntimeInt(2, len(history))]...)
	body := append([]runtimeMessage(nil), history[len(head):]...)
	existingSummary := ""
	filteredBody := make([]runtimeMessage, 0, len(body))
	for _, msg := range body {
		if msg.Role == "user" && strings.HasPrefix(strings.TrimSpace(msg.Text), genTestContextSummaryPrefix) {
			existingSummary = strings.TrimSpace(msg.Text)
			continue
		}
		filteredBody = append(filteredBody, msg)
	}

	tailStart := len(filteredBody) - recentKeep
	if tailStart < 0 {
		tailStart = 0
	}
	for tailStart > 0 && filteredBody[tailStart].Role == "tool" {
		tailStart--
	}
	compactable := filteredBody[:tailStart]
	tail := filteredBody[tailStart:]
	if len(compactable) == 0 {
		return history, false, 0
	}

	summary := buildRuntimeContextSummary(existingSummary, compactable)
	next := make([]runtimeMessage, 0, len(head)+1+len(tail))
	next = append(next, head...)
	next = append(next, runtimeMessage{Role: "user", Text: summary})
	next = append(next, tail...)

	for estimateRuntimeHistoryBytes(next) > maxBytes && len(tail) > 4 {
		compactable = append(compactable, tail[0])
		tail = tail[1:]
		for len(tail) > 0 && tail[0].Role == "tool" {
			compactable = append(compactable, tail[0])
			tail = tail[1:]
		}
		summary = buildRuntimeContextSummary(existingSummary, compactable)
		next = append(next[:0], head...)
		next = append(next, runtimeMessage{Role: "user", Text: summary})
		next = append(next, tail...)
	}

	return next, true, reductionCount + len(compactable)
}

func reduceRuntimeHistoryWithEino(ctx context.Context, history []runtimeMessage, maxBytes int) ([]runtimeMessage, bool, int) {
	messages := runtimeHistoryToEinoMessages(history)
	state := &adk.ChatModelAgentState{Messages: messages}
	placeholder := "[工具输出结果已由 Eino reduction 中间件清理；如需完整内容，请重新调用 Read/Grep/Glob/命令工具获取。]"
	middleware, err := reduction.NewClearToolResult(ctx, &reduction.ClearToolResultConfig{
		ToolResultTokenThreshold:   maxRuntimeInt(maxBytes/4, 1),
		KeepRecentTokens:           maxRuntimeInt(maxBytes/8, 1),
		ClearToolResultPlaceholder: placeholder,
		TokenCounter:               countEinoMessageTokens,
	})
	if err != nil || middleware.BeforeChatModel == nil {
		return history, false, 0
	}
	before := make([]string, len(state.Messages))
	for i, msg := range state.Messages {
		if msg != nil {
			before[i] = msg.Content
		}
	}
	if err := middleware.BeforeChatModel(ctx, state); err != nil {
		return history, false, 0
	}
	changedCount := 0
	for i, msg := range state.Messages {
		if msg != nil && i < len(before) && msg.Content != before[i] {
			changedCount++
		}
	}
	if changedCount == 0 {
		return history, false, 0
	}
	return einoMessagesToRuntimeHistory(state.Messages), true, changedCount
}

func runtimeHistoryToEinoMessages(history []runtimeMessage) []adk.Message {
	messages := make([]adk.Message, 0, len(history))
	toolNamesByID := make(map[string]string)
	for _, msg := range history {
		switch msg.Role {
		case "system":
			messages = append(messages, schema.SystemMessage(msg.Text))
		case "user":
			messages = append(messages, schema.UserMessage(msg.Text))
		case "assistant":
			toolCalls := make([]schema.ToolCall, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				argBytes, _ := json.Marshal(call.Arguments)
				toolCalls = append(toolCalls, schema.ToolCall{
					ID:   call.ID,
					Type: "function",
					Function: schema.FunctionCall{
						Name:      call.Name,
						Arguments: string(argBytes),
					},
				})
				if call.ID != "" {
					toolNamesByID[call.ID] = call.Name
				}
			}
			messages = append(messages, schema.AssistantMessage(msg.Text, toolCalls))
		case "tool":
			toolMsg := schema.ToolMessage(msg.ToolResult, msg.ToolCallID, schema.WithToolName(toolNamesByID[msg.ToolCallID]))
			toolMsg.Extra = map[string]any{"tool_error": msg.ToolError}
			messages = append(messages, toolMsg)
		}
	}
	return messages
}

func einoMessagesToRuntimeHistory(messages []adk.Message) []runtimeMessage {
	history := make([]runtimeMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.System:
			history = append(history, runtimeMessage{Role: "system", Text: msg.Content})
		case schema.User:
			history = append(history, runtimeMessage{Role: "user", Text: msg.Content})
		case schema.Assistant:
			toolCalls := make([]runtimeToolCall, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				args := make(map[string]any)
				if strings.TrimSpace(call.Function.Arguments) != "" {
					_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
				}
				toolCalls = append(toolCalls, runtimeToolCall{
					ID:        call.ID,
					Name:      call.Function.Name,
					Arguments: args,
				})
			}
			history = append(history, runtimeMessage{Role: "assistant", Text: msg.Content, ToolCalls: toolCalls})
		case schema.Tool:
			toolError, _ := msg.Extra["tool_error"].(bool)
			history = append(history, runtimeMessage{Role: "tool", ToolCallID: msg.ToolCallID, ToolResult: msg.Content, ToolError: toolError})
		}
	}
	return history
}

func countEinoMessageTokens(msg *schema.Message) int {
	if msg == nil {
		return 0
	}
	count := len(msg.Content) + len(msg.ToolCallID) + len(msg.ToolName)
	for _, call := range msg.ToolCalls {
		count += len(call.ID) + len(call.Type) + len(call.Function.Name) + len(call.Function.Arguments)
	}
	return (count + 3) / 4
}

func estimateRuntimeHistoryBytes(history []runtimeMessage) int {
	total := 0
	for _, msg := range history {
		total += len(msg.Role) + len(msg.Text) + len(msg.ToolCallID) + len(msg.ToolResult) + 32
		for _, call := range msg.ToolCalls {
			total += len(call.ID) + len(call.Name) + 32
			if data, err := json.Marshal(call.Arguments); err == nil {
				total += len(data)
			}
		}
	}
	return total
}

func buildRuntimeContextSummary(existingSummary string, messages []runtimeMessage) string {
	actions := make([]string, 0, 20)
	toolResults := make([]string, 0, 20)
	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			if text := strings.TrimSpace(msg.Text); text != "" {
				actions = append(actions, "assistant: "+truncateTextForSummary(text, 220))
			}
			for _, call := range msg.ToolCalls {
				args := summarizeToolArguments(call.Arguments)
				if args != "" {
					actions = append(actions, fmt.Sprintf("tool call: %s(%s)", call.Name, args))
				} else {
					actions = append(actions, fmt.Sprintf("tool call: %s", call.Name))
				}
			}
		case "tool":
			status := "ok"
			if msg.ToolError {
				status = "error"
			}
			toolResults = append(toolResults, fmt.Sprintf("%s result: %s", status, truncateTextForSummary(msg.ToolResult, 260)))
		case "user":
			if text := strings.TrimSpace(msg.Text); text != "" {
				actions = append(actions, "user: "+truncateTextForSummary(text, 180))
			}
		}
	}

	var b strings.Builder
	b.WriteString(genTestContextSummaryPrefix)
	b.WriteString("\n以下内容由运行时自动生成，用于降低后续模型请求上下文体积。请继续遵循初始任务和最近消息，必要时重新使用工具读取文件获取完整细节。\n")
	if existingSummary != "" {
		b.WriteString("\n### 既有摘要\n")
		b.WriteString(truncateTextForSummary(strings.TrimPrefix(existingSummary, genTestContextSummaryPrefix), 3000))
		b.WriteString("\n")
	}
	if len(actions) > 0 {
		b.WriteString("\n### 已完成动作\n")
		for _, line := range lastRuntimeStrings(actions, 24) {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if len(toolResults) > 0 {
		b.WriteString("\n### 工具结果要点\n")
		for _, line := range lastRuntimeStrings(toolResults, 24) {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return truncateByBytes(b.String(), 12_000)
}

func summarizeToolArguments(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, 4)
	for _, key := range []string{"path", "pattern", "command", "question", "action"} {
		if value := strings.TrimSpace(stringArg(args, key, "")); value != "" {
			parts = append(parts, fmt.Sprintf("%s=%q", key, truncateTextForSummary(value, 120)))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	data, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return truncateTextForSummary(string(data), 160)
}

func truncateTextForSummary(value string, limit int) string {
	value = strings.Join(compactLines(value, 20), " ")
	return truncateText(value, limit)
}

func lastRuntimeStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func compactInitialUserPrompt(prompt string, maxBytes int) string {
	if maxBytes <= 0 || len(prompt) <= maxBytes {
		return prompt
	}
	sections := splitMarkdownSections(prompt)
	for _, sectionName := range []string{
		"## Workflow Prompt Template",
		"## 输入上下文 JSON",
		"## AutoTestFlow 历史上下文压缩摘要",
		"## RAG",
		"## 知识库",
	} {
		sections = trimPromptSection(sections, sectionName, maxRuntimeInt(maxBytes/8, 2000))
		if joined := joinMarkdownSections(sections); len(joined) <= maxBytes {
			return joined
		}
	}
	return truncateByBytes(joinMarkdownSections(sections), maxBytes)
}

type promptSection struct {
	Title string
	Text  string
}

func splitMarkdownSections(prompt string) []promptSection {
	lines := strings.Split(strings.ReplaceAll(prompt, "\r\n", "\n"), "\n")
	sections := make([]promptSection, 0, 12)
	current := promptSection{}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current.Text != "" || current.Title != "" {
				sections = append(sections, current)
			}
			current = promptSection{Title: strings.TrimSpace(line), Text: line + "\n"}
			continue
		}
		current.Text += line + "\n"
	}
	if current.Text != "" || current.Title != "" {
		sections = append(sections, current)
	}
	return sections
}

func trimPromptSection(sections []promptSection, titlePrefix string, limit int) []promptSection {
	for index := range sections {
		if !strings.HasPrefix(sections[index].Title, titlePrefix) {
			continue
		}
		if len(sections[index].Text) <= limit {
			continue
		}
		sections[index].Text = truncateByBytes(sections[index].Text, limit) + "\n[该段已由运行时裁剪，必要时请使用 Read/Grep/Glob 重新获取完整上下文。]\n"
	}
	return sections
}

func joinMarkdownSections(sections []promptSection) string {
	var b strings.Builder
	for _, section := range sections {
		b.WriteString(section.Text)
		if !strings.HasSuffix(section.Text, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
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

func readGenTestDraft(workspace *RuntimeWorkspace) (*GenTestOutput, error) {
	if workspace == nil || strings.TrimSpace(workspace.ResultFile) == "" {
		return &GenTestOutput{}, nil
	}
	data, err := os.ReadFile(workspace.ResultFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &GenTestOutput{}, nil
		}
		return nil, fmt.Errorf("读取结果草稿失败: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return &GenTestOutput{}, nil
	}
	var draft GenTestOutput
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("解析结果草稿失败: %w", err)
	}
	return &draft, nil
}

func writeGenTestDraft(workspace *RuntimeWorkspace, draft *GenTestOutput) error {
	if workspace == nil || strings.TrimSpace(workspace.ResultFile) == "" {
		return nil
	}
	return writeJSONFile(workspace.ResultFile, draft)
}

func syncEditedArtifactDraft(workspace *RuntimeWorkspace, relativePath, updated string) error {
	draft, err := readGenTestDraft(workspace)
	if err != nil {
		return err
	}
	changed := false
	if normalizeRepoRelativePath(draft.TestScript.FilePath) == relativePath {
		draft.TestScript.FilePath = relativePath
		draft.TestScript.FileContent = updated
		if strings.TrimSpace(draft.TestScript.Language) == "" {
			draft.TestScript.Language = normalizeScriptLanguage("", relativePath)
		}
		changed = true
	}
	if normalizeRepoRelativePath(draft.TestDoc.FilePath) == relativePath {
		draft.TestDoc.FilePath = relativePath
		draft.TestDoc.Content = updated
		changed = true
	}
	if !changed {
		return nil
	}
	return writeGenTestDraft(workspace, draft)
}

func (r *EinoGenTestRuntime) buildSubmittedGenTestOutput(toolCtx *genTestToolCallContext, args map[string]any) (*GenTestOutput, error) {
	draft, err := readGenTestDraft(toolCtx.Workspace)
	if err != nil {
		return nil, err
	}
	mergedArgs := mergeSubmittedArgsWithDraft(args, draft)
	output, err := buildGenTestOutput(mergedArgs)
	if err != nil {
		return nil, err
	}
	mergeGenTestOutputFromDraft(output, draft)
	if err := hydrateSubmittedArtifacts(toolCtx.Workspace, output); err != nil {
		return nil, err
	}
	if err := writeGenTestDraft(toolCtx.Workspace, output); err != nil {
		return nil, err
	}
	return output, nil
}

func mergeSubmittedArgsWithDraft(args map[string]any, draft *GenTestOutput) map[string]any {
	merged := make(map[string]any)
	if draft != nil {
		data, err := json.Marshal(draft)
		if err == nil {
			_ = json.Unmarshal(data, &merged)
		}
	}
	for key, value := range args {
		merged[key] = value
	}
	return merged
}

func mergeGenTestOutputFromDraft(output, draft *GenTestOutput) {
	if output == nil || draft == nil {
		return
	}
	if len(output.TestCases) == 0 && len(draft.TestCases) > 0 {
		output.TestCases = draft.TestCases
	}
	if strings.TrimSpace(output.TestScript.FilePath) == "" {
		output.TestScript.FilePath = draft.TestScript.FilePath
	}
	if strings.TrimSpace(output.TestScript.FileContent) == "" {
		output.TestScript.FileContent = draft.TestScript.FileContent
	}
	if strings.TrimSpace(output.TestScript.Language) == "" {
		output.TestScript.Language = draft.TestScript.Language
	}
	if strings.TrimSpace(output.TestDoc.FilePath) == "" {
		output.TestDoc.FilePath = draft.TestDoc.FilePath
	}
	if strings.TrimSpace(output.TestDoc.Content) == "" {
		output.TestDoc.Content = draft.TestDoc.Content
	}
	if strings.TrimSpace(output.TestDoc.Title) == "" {
		output.TestDoc.Title = draft.TestDoc.Title
	}
	if output.SelfTest == nil {
		output.SelfTest = draft.SelfTest
	}
	if strings.TrimSpace(output.Summary) == "" {
		output.Summary = draft.Summary
	}
}

func hydrateSubmittedArtifacts(workspace *RuntimeWorkspace, output *GenTestOutput) error {
	if workspace == nil || output == nil {
		return nil
	}
	if path := strings.TrimSpace(output.TestScript.FilePath); path != "" {
		relativePath := normalizeRepoRelativePath(path)
		output.TestScript.FilePath = relativePath
		if strings.TrimSpace(output.TestScript.Language) == "" {
			output.TestScript.Language = normalizeScriptLanguage("", relativePath)
		}
		if strings.TrimSpace(output.TestScript.FileContent) == "" {
			content, err := readRepoFile(workspace.RepoDir, relativePath)
			if err != nil {
				return fmt.Errorf("test_script.file_path 无法读取: %w", err)
			}
			output.TestScript.FileContent = content
		} else if err := writeRepoFile(workspace.RepoDir, relativePath, output.TestScript.FileContent); err != nil {
			return err
		}
	}
	if path := strings.TrimSpace(output.TestDoc.FilePath); path != "" {
		relativePath := normalizeRepoRelativePath(path)
		output.TestDoc.FilePath = relativePath
		if strings.TrimSpace(output.TestDoc.Content) == "" {
			content, err := readRepoFile(workspace.RepoDir, relativePath)
			if err != nil {
				return fmt.Errorf("test_doc.file_path 无法读取: %w", err)
			}
			output.TestDoc.Content = content
		} else if err := writeRepoFile(workspace.RepoDir, relativePath, output.TestDoc.Content); err != nil {
			return err
		}
	}
	return nil
}

type generatedTestCommand struct {
	Display string
	Program string
	Args    []string
}

func (r *EinoGenTestRuntime) runGeneratedSelfTest(ctx context.Context, toolCtx *genTestToolCallContext, output *GenTestOutput) *SelfTestReport {
	report := &SelfTestReport{Passed: true, Checks: make([]string, 0, 4)}
	if output == nil {
		report.Passed = false
		report.Summary = "运行时输出为空，无法执行真实自测"
		return report
	}
	if err := hydrateSubmittedArtifacts(toolCtx.Workspace, output); err != nil {
		report.Passed = false
		report.Summary = err.Error()
		report.Checks = append(report.Checks, err.Error())
		return report
	}
	cmdSpec, err := detectGeneratedTestCommand(toolCtx.Workspace.RepoDir, output.TestScript)
	if err != nil {
		report.Passed = false
		report.Summary = err.Error()
		report.Checks = append(report.Checks, err.Error())
		return report
	}
	report.Command = cmdSpec.Display
	r.publish(toolCtx.Task.ID, taskEventTypeStage, "generated_self_test_started", model.TaskStatusRunning,
		fmt.Sprintf("开始真实执行生成脚本自测: %s", cmdSpec.Display), nil)
	attempt := runGeneratedTestCommand(ctx, toolCtx.Workspace.RepoDir, cmdSpec)
	report.Attempts = append(report.Attempts, attempt)
	report.Passed = attempt.Passed
	report.ExitCode = attempt.ExitCode
	report.Output = attempt.Output
	report.Checks = append(report.Checks, fmt.Sprintf("真实自测命令: %s", cmdSpec.Display))
	if attempt.Passed {
		report.Summary = "真实自测通过"
		report.ReportPath = detectPlaywrightReportPath(toolCtx.Workspace.RepoDir)
		if report.ReportPath != "" {
			report.Checks = append(report.Checks, "Playwright 报告: "+report.ReportPath)
		}
		r.publish(toolCtx.Task.ID, taskEventTypeStage, "generated_self_test_passed", model.TaskStatusRunning,
			report.Summary, map[string]any{"command": report.Command})
		return report
	}
	report.Summary = fmt.Sprintf("真实自测失败: %s exited with %d", cmdSpec.Display, attempt.ExitCode)
	if attempt.Output != "" {
		report.Checks = append(report.Checks, truncateTextForSummary(attempt.Output, 600))
	}
	r.publish(toolCtx.Task.ID, taskEventTypeLog, "generated_self_test_failed", model.TaskStatusRunning,
		report.Summary, map[string]any{"command": report.Command, "exit_code": report.ExitCode})
	return report
}

func runPersistedGeneratedSelfTest(ctx context.Context, workspace *RuntimeWorkspace, script GenTestScript) *SelfTestReport {
	report := &SelfTestReport{Passed: true, Checks: make([]string, 0, 4)}
	if workspace == nil {
		report.Passed = false
		report.Summary = "运行时工作区为空，无法执行真实自测"
		return report
	}
	cmdSpec, err := detectGeneratedTestCommand(workspace.RepoDir, script)
	if err != nil {
		report.Passed = false
		report.Summary = err.Error()
		report.Checks = append(report.Checks, err.Error())
		return report
	}
	attempt := runGeneratedTestCommand(ctx, workspace.RepoDir, cmdSpec)
	report.Attempts = append(report.Attempts, attempt)
	report.Command = cmdSpec.Display
	report.Passed = attempt.Passed
	report.ExitCode = attempt.ExitCode
	report.Output = attempt.Output
	if attempt.Passed {
		report.Summary = "真实自测通过"
		report.ReportPath = detectPlaywrightReportPath(workspace.RepoDir)
		return report
	}
	report.Summary = fmt.Sprintf("真实自测失败: %s exited with %d", cmdSpec.Display, attempt.ExitCode)
	return report
}

func detectGeneratedTestCommand(repoDir string, script GenTestScript) (generatedTestCommand, error) {
	path := normalizeRepoRelativePath(script.FilePath)
	if strings.TrimSpace(path) == "" {
		return generatedTestCommand{}, fmt.Errorf("无法执行真实自测: test_script.file_path 为空")
	}
	if _, err := readRepoFile(repoDir, path); err != nil {
		return generatedTestCommand{}, fmt.Errorf("无法执行真实自测: %w", err)
	}
	lang := normalizeScriptLanguage(script.Language, path)
	lowerPath := strings.ToLower(path)
	switch {
	case lang == "typescript" || lang == "javascript" || strings.HasSuffix(lowerPath, ".spec.ts") || strings.HasSuffix(lowerPath, ".spec.js"):
		program := executableName("npx")
		args := []string{"playwright", "test", filepath.ToSlash(path), "--reporter=list"}
		return generatedTestCommand{
			Display: "npx playwright test " + quoteCommandArg(filepath.ToSlash(path)) + " --reporter=list",
			Program: program,
			Args:    args,
		}, nil
	case lang == "python" || strings.HasSuffix(lowerPath, ".py"):
		program := executableName("python")
		return generatedTestCommand{
			Display: "python -m pytest " + quoteCommandArg(filepath.ToSlash(path)),
			Program: program,
			Args:    []string{"-m", "pytest", filepath.ToSlash(path)},
		}, nil
	default:
		return generatedTestCommand{}, fmt.Errorf("无法识别生成脚本测试命令: %s", path)
	}
}

func runGeneratedTestCommand(ctx context.Context, repoDir string, cmdSpec generatedTestCommand) SelfTestAttempt {
	start := time.Now()
	attempt := SelfTestAttempt{Command: cmdSpec.Display}
	callCtx, cancel := context.WithTimeout(ctx, defaultGenTestSelfTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(callCtx, cmdSpec.Program, cmdSpec.Args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	attempt.DurationMS = time.Since(start).Milliseconds()
	attempt.Output = truncateByBytes(strings.TrimSpace(string(output)), defaultGenTestToolResultMaxBytes)
	if err == nil {
		attempt.Passed = true
		return attempt
	}
	attempt.Passed = false
	attempt.ExitCode = 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		attempt.ExitCode = exitErr.ExitCode()
	}
	if callCtx.Err() == context.DeadlineExceeded {
		attempt.ExitCode = -1
		if attempt.Output != "" {
			attempt.Output += "\n"
		}
		attempt.Output += "command timed out"
	} else if attempt.Output == "" {
		attempt.Output = err.Error()
	}
	return attempt
}

func buildSelfTestRepairPrompt(report *SelfTestReport, attempt, limit int) string {
	if report == nil {
		return "真实自测失败。请检查已写入的测试脚本和文档，修复后再次调用 SubmitGenTestResult。"
	}
	return fmt.Sprintf(`真实执行生成脚本自测失败，需要修复后重新提交。

修复轮次: %d/%d
命令: %s
退出码: %d
摘要: %s
关键日志:
%s

请使用 Read/Grep/Glob/WriteTestScript/WriteTestDoc/Edit 等工具修复仓库中的测试脚本和文档，然后再次调用 SubmitGenTestResult。`,
		attempt,
		limit,
		report.Command,
		report.ExitCode,
		report.Summary,
		truncateByBytes(report.Output, 6000),
	)
}

func detectPlaywrightReportPath(repoDir string) string {
	reportPath := filepath.Join(repoDir, "playwright-report", "index.html")
	if _, err := os.Stat(reportPath); err == nil {
		return "playwright-report/index.html"
	}
	return ""
}

func executableName(name string) string {
	if runtime.GOOS == "windows" && name == "npx" {
		return "npx.cmd"
	}
	return name
}

func quoteCommandArg(value string) string {
	if strings.ContainsAny(value, " \t\"'") {
		return strconv.Quote(value)
	}
	return value
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

func maxRuntimeInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
