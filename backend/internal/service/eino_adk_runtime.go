package service

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"auto-test-flow/internal/model"

	"github.com/cloudwego/eino/adk"
	adkreduction "github.com/cloudwego/eino/adk/middlewares/reduction"
	deepadk "github.com/cloudwego/eino/adk/prebuilt/deep"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const (
	defaultGenTestADKMaxIterations                = 100
	defaultGenTestADKReductionMaxTokens           = 24000
	defaultGenTestADKReductionRetentionSuffix     = 4
	defaultGenTestADKReductionClearAtLeastTokens  = 2000
	defaultGenTestADKReductionPublishIntervalSecs = 5

	genTestADKMainAgentName     = "autotestflow_gen_test"
	genTestADKExplorerAgentName = "repo_explorer"
	genTestADKWriterAgentName   = "test_writer"
	genTestADKReviewerAgentName = "self_reviewer"
)

type genTestADKConfig struct {
	Enabled            bool
	MaxIterations      int
	EmitInternalEvents bool
}

type genTestRuntimeConfigPayload struct {
	RuntimeType string `json:"runtime_type"`
	ADK         *struct {
		MaxIterations      int   `json:"max_iterations"`
		EmitInternalEvents *bool `json:"emit_internal_events"`
	} `json:"adk"`
}

type genTestADKTool struct {
	spec     genTestToolSpec
	runtime  *EinoGenTestRuntime
	toolCtx  *genTestToolCallContext
	finalMu  *sync.Mutex
	finalOut **GenTestOutput
}

func resolveGenTestADKConfig(workflow *model.Skill, agent *model.Agent) genTestADKConfig {
	cfg := genTestADKConfig{
		MaxIterations:      defaultGenTestADKMaxIterations,
		EmitInternalEvents: true,
	}

	agentPayload := parseGenTestRuntimePayload(agentConfigJSON(agent))
	workflowPayload := parseGenTestRuntimePayload(skillConfigJSON(workflow))

	applyGenTestADKPayload(&cfg, agentPayload)
	applyGenTestADKPayload(&cfg, workflowPayload)

	runtimeType := strings.TrimSpace(agentPayload.RuntimeType)
	if strings.TrimSpace(workflowPayload.RuntimeType) != "" {
		runtimeType = strings.TrimSpace(workflowPayload.RuntimeType)
	}
	cfg.Enabled = strings.EqualFold(runtimeType, "adk")
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = defaultGenTestADKMaxIterations
	}
	return cfg
}

func parseGenTestRuntimePayload(raw []byte) genTestRuntimeConfigPayload {
	if len(raw) == 0 {
		return genTestRuntimeConfigPayload{}
	}
	var payload genTestRuntimeConfigPayload
	_ = json.Unmarshal(raw, &payload)
	return payload
}

func applyGenTestADKPayload(cfg *genTestADKConfig, payload genTestRuntimeConfigPayload) {
	if cfg == nil || payload.ADK == nil {
		return
	}
	if payload.ADK.MaxIterations > 0 {
		cfg.MaxIterations = payload.ADK.MaxIterations
	}
	if payload.ADK.EmitInternalEvents != nil {
		cfg.EmitInternalEvents = *payload.ADK.EmitInternalEvents
	}
}

func agentConfigJSON(agent *model.Agent) []byte {
	if agent == nil || len(agent.ConfigJSON) == 0 {
		return nil
	}
	return []byte(agent.ConfigJSON)
}

func skillConfigJSON(skill *model.Skill) []byte {
	if skill == nil || len(skill.ConfigJSON) == 0 {
		return nil
	}
	return []byte(skill.ConfigJSON)
}

func (r *EinoGenTestRuntime) runADKAgentLoop(
	ctx context.Context,
	cfg AgentExecutionConfig,
	toolCtx *genTestToolCallContext,
	promptCtx *CLIPromptContext,
	adkCfg genTestADKConfig,
) (*GenTestOutput, error) {
	baseModel, err := r.newEinoChatModel(ctx, cfg, nil)
	if err != nil {
		return nil, err
	}

	toolSpecs := r.collectGenTestToolSpecs(toolCtx)
	var finalMu sync.Mutex
	var finalOutput *GenTestOutput

	mainTools, err := r.buildADKTools(toolCtx, toolSpecs, nil, &finalMu, &finalOutput)
	if err != nil {
		return nil, err
	}
	explorerTools, err := r.buildADKTools(toolCtx, toolSpecs, genTestReadonlyToolSet(), &finalMu, &finalOutput)
	if err != nil {
		return nil, err
	}
	writerTools, err := r.buildADKTools(toolCtx, toolSpecs, genTestWriterToolSet(), &finalMu, &finalOutput)
	if err != nil {
		return nil, err
	}
	reviewerTools, err := r.buildADKTools(toolCtx, toolSpecs, genTestReviewerToolSet(), &finalMu, &finalOutput)
	if err != nil {
		return nil, err
	}

	adkHandlers := r.buildADKReductionHandlers(ctx, toolCtx)

	subAgents, err := r.buildADKSubAgents(ctx, baseModel, adkCfg, adkHandlers, explorerTools, writerTools, reviewerTools)
	if err != nil {
		return nil, err
	}

	systemPrompt := "你是 AutoTestFlow 的测试生成主代理。你可以使用 task 工具委派 repo_explorer、test_writer、self_reviewer 三个子代理，但最终必须由你调用 SubmitGenTestResult 提交结构化结果。"
	userPrompt := r.buildPrompt(toolCtx.Workspace, toolCtx.Task, toolCtx.Input, toolCtx.Workflow, toolCtx.Agent, promptCtx, toolCtx.RAGContext)

	mainAgent, err := deepadk.New(ctx, &deepadk.Config{
		Name:                   genTestADKMainAgentName,
		Description:            "AutoTestFlow 测试资产生成主代理，负责委派子任务并提交最终结构化结果。",
		ChatModel:              baseModel,
		Instruction:            systemPrompt + "\n\n" + userPrompt,
		SubAgents:              subAgents,
		ToolsConfig:            adk.ToolsConfig{ToolsNodeConfig: toolsNodeConfig(mainTools), ReturnDirectly: map[string]bool{"SubmitGenTestResult": true}, EmitInternalEvents: adkCfg.EmitInternalEvents},
		Handlers:               adkHandlers,
		MaxIteration:           adkCfg.MaxIterations,
		WithoutWriteTodos:      true,
		WithoutGeneralSubAgent: true,
		ModelRetryConfig:       genTestADKModelRetryConfig(),
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 Eino ADK DeepAgent 失败: %w", err)
	}

	r.publish(toolCtx.Task.ID, taskEventTypeStage, "adk_runtime_started", model.TaskStatusRunning,
		fmt.Sprintf("开始执行 Eino ADK DeepAgent 运行时\n  max_iterations: %d\n  sub_agents: %s, %s, %s",
			adkCfg.MaxIterations, genTestADKExplorerAgentName, genTestADKWriterAgentName, genTestADKReviewerAgentName),
		map[string]any{"runtime_type": "adk", "max_iterations": adkCfg.MaxIterations, "context_reduction": len(adkHandlers) > 0})

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: mainAgent, EnableStreaming: false})
	iter := runner.Run(ctx, []adk.Message{schema.UserMessage("开始执行当前 AutoTestFlow 测试生成任务。")})
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		r.publishADKEvent(toolCtx.Task.ID, event)
		if event.Err != nil {
			if output, recovered := r.recoverADKDraftOutput(ctx, toolCtx, event.Err); recovered {
				return output, nil
			}
			return nil, event.Err
		}
	}

	finalMu.Lock()
	output := finalOutput
	finalMu.Unlock()
	if output != nil {
		return output, nil
	}
	if output, recovered := r.recoverADKDraftOutput(ctx, toolCtx, fmt.Errorf("Eino ADK 运行时未提交结构化结果")); recovered {
		return output, nil
	}
	return nil, fmt.Errorf("Eino ADK 运行时未提交结构化结果")
}

func (r *EinoGenTestRuntime) buildADKReductionHandlers(ctx context.Context, toolCtx *genTestToolCallContext) []adk.ChatModelAgentMiddleware {
	var taskID uint64
	if toolCtx != nil && toolCtx.Task != nil {
		taskID = toolCtx.Task.ID
	}
	var publishMu sync.Mutex
	var lastPublish time.Time
	middleware, err := adkreduction.New(ctx, &adkreduction.Config{
		SkipTruncation:            true,
		SkipClear:                 false,
		ReadFileToolName:          "Read",
		MaxTokensForClear:         defaultGenTestADKReductionMaxTokens,
		ClearRetentionSuffixLimit: defaultGenTestADKReductionRetentionSuffix,
		ClearAtLeastTokens:        defaultGenTestADKReductionClearAtLeastTokens,
		ClearExcludeTools:         []string{"SubmitGenTestResult"},
		ClearPostProcess: func(ctx context.Context, state *adk.ChatModelAgentState) context.Context {
			if taskID == 0 {
				return ctx
			}
			publishMu.Lock()
			defer publishMu.Unlock()
			if !lastPublish.IsZero() && time.Since(lastPublish) < time.Duration(defaultGenTestADKReductionPublishIntervalSecs)*time.Second {
				return ctx
			}
			lastPublish = time.Now()
			r.publish(taskID, taskEventTypeLog, "adk_context_reduction", model.TaskStatusRunning,
				"ADK 上下文已触发自动清理：历史工具输出已折叠，仅保留最近关键对话，继续请求模型",
				map[string]any{
					"max_tokens_for_clear":      defaultGenTestADKReductionMaxTokens,
					"retention_suffix_limit":    defaultGenTestADKReductionRetentionSuffix,
					"clear_at_least_tokens":     defaultGenTestADKReductionClearAtLeastTokens,
					"excluded_tools":            []string{"SubmitGenTestResult"},
					"reduction_middleware":      "cloudwego/eino/adk/middlewares/reduction",
					"reduction_skip_truncation": true,
				})
			return ctx
		},
	})
	if err != nil {
		r.logger.Warn("初始化 ADK 上下文清理中间件失败", zap.Uint64("task_id", taskID), zap.Error(err))
		if taskID > 0 {
			r.publish(taskID, taskEventTypeLog, "adk_context_reduction", model.TaskStatusWarning,
				fmt.Sprintf("ADK 上下文清理中间件初始化失败，已继续执行: %v", err), nil)
		}
		return nil
	}
	return []adk.ChatModelAgentMiddleware{middleware}
}

func (r *EinoGenTestRuntime) recoverADKDraftOutput(ctx context.Context, toolCtx *genTestToolCallContext, reason error) (*GenTestOutput, bool) {
	if toolCtx == nil || toolCtx.Workspace == nil {
		return nil, false
	}
	var taskID uint64
	if toolCtx.Task != nil {
		taskID = toolCtx.Task.ID
	}
	draft, err := readGenTestDraft(toolCtx.Workspace)
	if err != nil {
		r.logger.Warn("读取 ADK 结果草稿失败", zap.Uint64("task_id", taskID), zap.Error(err))
		return nil, false
	}
	if !hasRecoverableGenTestDraft(draft) {
		return nil, false
	}
	if err := hydrateSubmittedArtifacts(toolCtx.Workspace, draft); err != nil {
		r.logger.Warn("恢复 ADK 结果草稿失败", zap.Uint64("task_id", taskID), zap.Error(err))
		return nil, false
	}
	reasonText := "Eino ADK 运行时未提交结构化结果"
	if reason != nil {
		reasonText = reason.Error()
	}
	if strings.TrimSpace(draft.Summary) == "" {
		draft.Summary = "ADK 运行未完成提交，已从结果草稿恢复测试资产"
	}
	if draft.SelfTest == nil {
		draft.SelfTest = &SelfTestReport{
			Passed:  false,
			Summary: "ADK 运行未完成提交，已从结果草稿恢复测试资产",
			Checks:  []string{reasonText},
		}
	} else {
		draft.SelfTest.Passed = false
		if strings.TrimSpace(draft.SelfTest.Summary) == "" {
			draft.SelfTest.Summary = "ADK 运行未完成提交，已从结果草稿恢复测试资产"
		}
		if len(draft.SelfTest.Checks) == 0 {
			draft.SelfTest.Checks = []string{reasonText}
		}
	}
	if err := writeGenTestDraft(toolCtx.Workspace, draft); err != nil {
		r.logger.Warn("写入恢复后的 ADK 结果草稿失败", zap.Uint64("task_id", taskID), zap.Error(err))
	}
	status := model.TaskStatusRunning
	if ctx != nil && ctx.Err() != nil {
		status = model.TaskStatusWarning
	}
	if taskID > 0 {
		r.publish(taskID, taskEventTypeLog, "adk_fallback_result", status,
			"ADK 未完成 SubmitGenTestResult，已从结果草稿恢复测试资产并继续后续流程",
			map[string]any{"reason": reasonText})
	}
	return draft, true
}

func hasRecoverableGenTestDraft(draft *GenTestOutput) bool {
	if draft == nil {
		return false
	}
	if len(draft.TestCases) > 0 {
		return true
	}
	if strings.TrimSpace(draft.TestScript.FilePath) != "" || strings.TrimSpace(draft.TestScript.FileContent) != "" {
		return true
	}
	if strings.TrimSpace(draft.TestDoc.FilePath) != "" || strings.TrimSpace(draft.TestDoc.Content) != "" {
		return true
	}
	return false
}

func (r *EinoGenTestRuntime) collectGenTestToolSpecs(toolCtx *genTestToolCallContext) []genTestToolSpec {
	toolSpecs := r.baseToolSpecs()
	toolNames := make(map[string]struct{}, len(toolSpecs))
	for _, tool := range toolSpecs {
		toolNames[tool.Name] = struct{}{}
	}
	if toolCtx != nil && toolCtx.MCP != nil {
		for _, toolPayload := range toolCtx.MCP.OpenAITools() {
			fn, ok := toolPayload["function"].(map[string]any)
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
				if toolCtx.Task != nil {
					r.publish(toolCtx.Task.ID, taskEventTypeLog, "mcp_tool_skipped", model.TaskStatusRunning,
						fmt.Sprintf("MCP 工具 %s 与内置工具重名，已跳过以保证内置工具优先", name), nil)
				}
				continue
			}
			toolNames[name] = struct{}{}
			toolSpecs = append(toolSpecs, genTestToolSpec{Name: name, Description: description, Schema: parameters})
		}
	}
	return toolSpecs
}

func (r *EinoGenTestRuntime) buildADKSubAgents(
	ctx context.Context,
	baseModel einomodel.BaseChatModel,
	adkCfg genTestADKConfig,
	handlers []adk.ChatModelAgentMiddleware,
	explorerTools []einotool.BaseTool,
	writerTools []einotool.BaseTool,
	reviewerTools []einotool.BaseTool,
) ([]adk.Agent, error) {
	explorer, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:             genTestADKExplorerAgentName,
		Description:      "只读探索仓库结构、测试框架、现有 helper 和相关实现，输出事实摘要和建议。",
		Instruction:      "你是只读仓库探索子代理。只能使用 Read、Glob、Grep 获取事实，不要写文件，不要提交最终结果。",
		Model:            baseModel,
		ToolsConfig:      adk.ToolsConfig{ToolsNodeConfig: toolsNodeConfig(explorerTools), EmitInternalEvents: adkCfg.EmitInternalEvents},
		Handlers:         handlers,
		MaxIterations:    adkCfg.MaxIterations,
		ModelRetryConfig: genTestADKModelRetryConfig(),
	})
	if err != nil {
		return nil, err
	}
	writer, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:             genTestADKWriterAgentName,
		Description:      "根据已知事实生成或修订测试脚本和测试文档，负责写入测试资产草稿。",
		Instruction:      "你是测试资产写入子代理。使用 WriteTestScript 和 WriteTestDoc 写入测试脚本与文档；不要调用 SubmitGenTestResult。",
		Model:            baseModel,
		ToolsConfig:      adk.ToolsConfig{ToolsNodeConfig: toolsNodeConfig(writerTools), EmitInternalEvents: adkCfg.EmitInternalEvents},
		Handlers:         handlers,
		MaxIterations:    adkCfg.MaxIterations,
		ModelRetryConfig: genTestADKModelRetryConfig(),
	})
	if err != nil {
		return nil, err
	}
	reviewer, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:             genTestADKReviewerAgentName,
		Description:      "审查已生成测试资产，运行允许的平台命令完成自测并输出修复建议。",
		Instruction:      "你是测试资产审查子代理。只运行主代理指定的目标测试命令并分析当前测试的 stdout/stderr、截图、HTML 报告或 error-context；不要创建、编辑、删除文件，不要探索无关 test-results，不要运行无关测试，不要调用 SubmitGenTestResult。失败时用不超过 1200 个中文字符返回失败原因、已检查证据和修复建议。",
		Model:            baseModel,
		ToolsConfig:      adk.ToolsConfig{ToolsNodeConfig: toolsNodeConfig(reviewerTools), EmitInternalEvents: adkCfg.EmitInternalEvents},
		Handlers:         handlers,
		MaxIterations:    adkCfg.MaxIterations,
		ModelRetryConfig: genTestADKModelRetryConfig(),
	})
	if err != nil {
		return nil, err
	}
	return []adk.Agent{explorer, writer, reviewer}, nil
}

func (r *EinoGenTestRuntime) buildADKTools(
	toolCtx *genTestToolCallContext,
	specs []genTestToolSpec,
	allow map[string]struct{},
	finalMu *sync.Mutex,
	finalOut **GenTestOutput,
) ([]einotool.BaseTool, error) {
	tools := make([]einotool.BaseTool, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		if allow != nil {
			if _, ok := allow[name]; !ok {
				continue
			}
		}
		tools = append(tools, &genTestADKTool{spec: spec, runtime: r, toolCtx: toolCtx, finalMu: finalMu, finalOut: finalOut})
	}
	return tools, nil
}

func (t *genTestADKTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	infos, err := buildEinoToolInfos([]genTestToolSpec{t.spec})
	if err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("工具定义为空: %s", t.spec.Name)
	}
	return infos[0], nil
}

func (t *genTestADKTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	args := make(map[string]any)
	if strings.TrimSpace(argumentsInJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("解析 ADK tool arguments 失败 (%s): %w", t.spec.Name, err)
		}
	}
	result, err := t.runtime.executeTool(ctx, t.toolCtx, runtimeToolCall{Name: t.spec.Name, Arguments: args})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	if result.FinalOutput != nil && t.finalMu != nil && t.finalOut != nil {
		t.finalMu.Lock()
		*t.finalOut = result.FinalOutput
		t.finalMu.Unlock()
	}
	return truncateByBytes(result.Content, defaultGenTestToolResultMaxBytes), nil
}

func (r *EinoGenTestRuntime) publishADKEvent(taskID uint64, event *adk.AgentEvent) {
	if event == nil {
		return
	}
	stage := "adk_event"
	if strings.TrimSpace(event.AgentName) != "" {
		stage = "adk_" + event.AgentName
	}
	if event.Err != nil {
		r.publish(taskID, taskEventTypeError, stage, model.TaskStatusRunning, event.Err.Error(), map[string]any{"agent_name": event.AgentName})
		return
	}
	if event.Output != nil && event.Output.MessageOutput != nil {
		msg, err := event.Output.MessageOutput.GetMessage()
		if err == nil && msg != nil {
			if payload := buildADKStructuredPayload(event.AgentName, msg); payload != nil {
				r.publish(taskID, taskEventTypeLog, stage, model.TaskStatusRunning, marshalTaskEventJSON(payload), map[string]any{"agent_name": event.AgentName})
			}
		}
	}
	if event.Action != nil {
		r.publish(taskID, taskEventTypeLog, stage, model.TaskStatusRunning, "ADK action emitted", map[string]any{"agent_name": event.AgentName})
	}
}

func buildADKStructuredPayload(agentName string, msg *schema.Message) map[string]any {
	if msg == nil {
		return nil
	}
	base := map[string]any{
		"agent_name": agentName,
		"source":     "adk",
	}
	if msg.Role == schema.Tool {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			return nil
		}
		base["type"] = "user"
		base["message"] = map[string]any{
			"content": []map[string]any{
				{
					"type":         "tool_result",
					"tool_call_id": msg.ToolCallID,
					"tool_name":    msg.ToolName,
					"content":      truncateToolResult(content),
					"is_error":     false,
				},
			},
		}
		return base
	}

	contentBlocks := make([]map[string]any, 0, 1+len(msg.ToolCalls))
	if text := strings.TrimSpace(msg.Content); text != "" {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": text})
	}
	for _, call := range msg.ToolCalls {
		args := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		}
		contentBlocks = append(contentBlocks, map[string]any{
			"type":  "tool_use",
			"id":    call.ID,
			"name":  call.Function.Name,
			"input": args,
		})
	}
	if len(contentBlocks) == 0 {
		return nil
	}
	base["type"] = "assistant"
	base["message"] = map[string]any{
		"content": contentBlocks,
	}
	return base
}

func toolsNodeConfig(tools []einotool.BaseTool) compose.ToolsNodeConfig {
	return compose.ToolsNodeConfig{Tools: tools, ExecuteSequentially: true}
}

func genTestADKModelRetryConfig() *adk.ModelRetryConfig {
	return &adk.ModelRetryConfig{
		MaxRetries: modelConnectionRetryAttempts,
		IsRetryAble: func(_ context.Context, err error) bool {
			return isModelConnectionError(err)
		},
		BackoffFunc: func(_ context.Context, _ int) time.Duration {
			return modelConnectionRetryDelay
		},
	}
}

func genTestReadonlyToolSet() map[string]struct{} {
	return map[string]struct{}{"Read": {}, "Glob": {}, "Grep": {}}
}

func genTestWriterToolSet() map[string]struct{} {
	return map[string]struct{}{"Read": {}, "Glob": {}, "Grep": {}, "WriteTestScript": {}, "WriteTestDoc": {}, "Edit": {}}
}

func genTestReviewerToolSet() map[string]struct{} {
	allow := map[string]struct{}{"Read": {}, "Glob": {}, "Grep": {}}
	for _, spec := range commandToolSpecs(runtime.GOOS) {
		allow[spec.Name] = struct{}{}
	}
	return allow
}
