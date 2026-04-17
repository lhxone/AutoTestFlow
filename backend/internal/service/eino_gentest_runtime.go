package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"auto-test-flow/internal/model"
	"auto-test-flow/internal/repository"

	"go.uber.org/zap"
)

const (
	defaultGenTestTurnLimit          = 24
	defaultGenTestToolResultMaxBytes = 12_000
	defaultGenTestReadMaxBytes       = 64_000
)

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
	Workspace *CLIRuntimeWorkspace
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
			Timeout: 90 * time.Second,
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
	workspace *CLIRuntimeWorkspace,
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

你正在执行 AutoTestFlow 的 gen-test 任务，请在当前仓库目录内完成测试资产生成。

## 强制规则
- 优先遵循 gen-test 技能流程：探索测试结构 -> 发现共享工具/数据 -> 生成测试文档和脚本 -> 运行自测 -> 修复 -> 重试。
- 你必须通过可用工具真实地探索仓库和运行命令，不要凭空假设项目结构。
- 如果是 Web UI 测试且有 Chrome MCP，优先用 Chrome MCP 确认 DOM、交互和选择器。
- 禁止默认执行 npm install；如确需安装依赖，只允许使用 pnpm。
- 生成的测试脚本必须包含至少一个可执行断言。
- 必须把实际文件写入仓库目录。
- 完成后必须调用 SubmitGenTestResult 工具提交结构化结果，而不是只输出自然语言。
- 若关键信息缺失，可调用 AskUserQuestion；若需要人工许可，可调用 RequestPermission。

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

		resp, err := r.callModel(ctx, cfg, history, toolSpecs)
		if err != nil {
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
		content, err := readRepoFile(toolCtx.Workspace.RepoDir, normalizeRepoRelativePath(path))
		if err != nil {
			return nil, err
		}
		return &genTestToolResult{Content: truncateByBytes(content, defaultGenTestReadMaxBytes)}, nil
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
		command, err := requiredString(call.Arguments, "command")
		if err != nil {
			return nil, err
		}
		return r.runCommand(ctx, toolCtx, "powershell", []string{"-NoProfile", "-Command", command})
	case "Bash":
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

func (r *EinoGenTestRuntime) runGlob(repoDir, pattern string) (*genTestToolResult, error) {
	cmd := exec.Command("rg", "--files", "-g", pattern)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("Glob 执行失败: %w", err)
	}
	lines := compactLines(string(output), 200)
	return &genTestToolResult{Content: strings.Join(lines, "\n")}, nil
}

func (r *EinoGenTestRuntime) runGrep(repoDir, pattern, scope string, maxMatches int) (*genTestToolResult, error) {
	if maxMatches <= 0 {
		maxMatches = 50
	}
	args := []string{"-n", "--no-heading", "--color", "never", "--max-count", strconv.Itoa(maxMatches), pattern, scope}
	cmd := exec.Command("rg", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("Grep 执行失败: %w", err)
	}
	return &genTestToolResult{Content: truncateByBytes(string(output), defaultGenTestToolResultMaxBytes)}, nil
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
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(callCtx, program, args...)
	cmd.Dir = toolCtx.Workspace.RepoDir
	output, err := cmd.CombinedOutput()
	result := truncateByBytes(string(output), defaultGenTestToolResultMaxBytes)
	if err != nil {
		return &genTestToolResult{
			Content: fmt.Sprintf("%s failed: %v\n%s", program, err, result),
			IsError: true,
		}, nil
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

func (r *EinoGenTestRuntime) baseToolSpecs() []genTestToolSpec {
	return []genTestToolSpec{
		{
			Name:        "Read",
			Description: "Read a repository file by relative path.",
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
			Description: "Find repository files by rg glob pattern, for example **/*.spec.ts.",
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
			Description: "Search file contents with ripgrep.",
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
		{
			Name:        "PowerShell",
			Description: "Execute a PowerShell command in the repository root.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "Bash",
			Description: "Execute a bash command in the repository root.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
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
	}
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
	data, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	var output GenTestOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return &output, nil
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
