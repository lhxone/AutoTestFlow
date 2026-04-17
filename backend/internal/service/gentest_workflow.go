package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"auto-test-flow/internal/model"

	"github.com/cloudwego/eino/compose"
)

type genTestWorkflowState struct {
	Task          *model.TestTask
	Input         *GenTestInput
	Workflow      *model.Skill
	Agent         *model.Agent
	RuntimeConfig WorkflowRuntimeConfig
	PromptContext *CLIPromptContext
	SelfTest      *SelfTestReport
}

type genTestWorkflowEngine struct {
	once   sync.Once
	runner compose.Runnable[uint64, *genTestWorkflowState]
	err    error
}

func newGenTestWorkflowEngine() *genTestWorkflowEngine {
	return &genTestWorkflowEngine{}
}

func (e *genTestWorkflowEngine) get(s *GenTestService) (compose.Runnable[uint64, *genTestWorkflowState], error) {
	e.once.Do(func() {
		e.runner, e.err = buildGenTestWorkflow(s)
	})
	return e.runner, e.err
}

func buildGenTestWorkflow(s *GenTestService) (compose.Runnable[uint64, *genTestWorkflowState], error) {
	wf := compose.NewWorkflow[uint64, *genTestWorkflowState]()

	wf.AddLambdaNode("prepare_context", compose.InvokableLambda(func(ctx context.Context, taskID uint64) (*genTestWorkflowState, error) {
		return s.prepareWorkflowContext(ctx, taskID)
	})).AddInput(compose.START)

	wf.AddLambdaNode("mcp_preflight", compose.InvokableLambda(func(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
		return s.performMCPPreflight(ctx, state)
	})).AddInput("prepare_context")

	wf.AddLambdaNode("generate_assets", compose.InvokableLambda(func(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
		return s.generateWorkflowAssets(ctx, state)
	})).AddInput("mcp_preflight")

	wf.AddLambdaNode("self_test", compose.InvokableLambda(func(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
		return s.runWorkflowSelfTest(ctx, state)
	})).AddInput("generate_assets")

	wf.AddLambdaNode("finalize_task", compose.InvokableLambda(func(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
		return s.finalizeWorkflowTask(ctx, state)
	})).AddInput("self_test")

	wf.End().AddInput("finalize_task")

	return wf.Compile(context.Background(), compose.WithGraphName("gen-test-eino-workflow"))
}

func (s *GenTestService) prepareWorkflowContext(ctx context.Context, taskID uint64) (*genTestWorkflowState, error) {
	task, err := s.testTaskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("测试任务不存在: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	input, err := s.loadInputForTask(task)
	if err != nil {
		return nil, err
	}

	workflow, err := s.loadWorkflow(task.SkillName)
	if err != nil {
		return nil, err
	}

	agent, err := s.resolveAgentForTask(task)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	task.Status = model.TaskStatusRunning
	task.ErrorMessage = ""
	task.CompletedAt = nil
	task.StartedAt = &now
	task.RetryCount = 0
	task.AIOutput = nil
	if err := s.testTaskRepo.Update(task); err != nil {
		return nil, err
	}
	_ = s.issueRepo.ForceUpdateTestStatus(task.IssueID, model.TestStatusGenerating)

	state := &genTestWorkflowState{
		Task:          task,
		Input:         input,
		Workflow:      workflow,
		Agent:         agent,
		RuntimeConfig: s.ResolveWorkflowRuntimeConfig(task.SkillName),
		PromptContext: &CLIPromptContext{},
	}

	s.publishTaskEvent(task.ID, TaskEvent{
		Type:    taskEventTypeStage,
		Stage:   "context_loaded",
		Status:  model.TaskStatusRunning,
		Message: fmt.Sprintf("Eino 工作流已加载任务上下文\n  workflow: %s\n  agent: %s", task.SkillName, safeAgentName(agent)),
		Data: map[string]any{
			"workflow_name": task.SkillName,
			"agent_name":    safeAgentName(agent),
		},
	})

	return state, nil
}

func (s *GenTestService) performMCPPreflight(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
	if state == nil || state.Task == nil {
		return nil, fmt.Errorf("workflow state 为空")
	}

	chromeServers := findChromeMCPServers(state.Agent)
	if state.PromptContext == nil {
		state.PromptContext = &CLIPromptContext{}
	}
	state.PromptContext.ChromeMCPServers = chromeServers

	if state.Agent == nil || len(state.Agent.MCPServers) == 0 {
		s.publishTaskEvent(state.Task.ID, TaskEvent{
			Type:    taskEventTypeLog,
			Stage:   "mcp_preflight",
			Status:  model.TaskStatusRunning,
			Message: "未配置 MCP Server，继续使用 Eino 原生工具能力",
		})
		return state, nil
	}

	preflightCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	runtime, err := NewMCPRuntime(preflightCtx, s.logger, state.Agent)
	if err != nil {
		s.publishTaskEvent(state.Task.ID, TaskEvent{
			Type:    taskEventTypeLog,
			Stage:   "mcp_preflight",
			Status:  model.TaskStatusRunning,
			Message: fmt.Sprintf("MCP 预检失败，生成流程继续执行: %v", err),
		})
		state.PromptContext.MCPCapabilitySummary = fmt.Sprintf("MCP 预检失败: %v", err)
		return state, nil
	}
	defer runtime.Close()

	summary := strings.TrimSpace(runtime.CapabilitySummary())
	state.PromptContext.MCPCapabilitySummary = summary

	message := "MCP 预检完成"
	if len(chromeServers) > 0 {
		message = fmt.Sprintf("%s，已发现 Chrome MCP: %s", message, strings.Join(chromeServers, ", "))
	}
	if summary != "" {
		message += "\n" + summary
	}

	s.publishTaskEvent(state.Task.ID, TaskEvent{
		Type:    taskEventTypeStage,
		Stage:   "mcp_preflight",
		Status:  model.TaskStatusRunning,
		Message: message,
	})

	return state, nil
}

func (s *GenTestService) generateWorkflowAssets(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
	err := s.runStageWithRetry(ctx, state.Task.ID, "generate_assets", 2, func(execCtx context.Context) error {
		return s.runGenerateTask(execCtx, state.Task, state.Input, state.Workflow, state.Agent, state.PromptContext)
	})
	if err != nil {
		s.publishTaskEvent(state.Task.ID, TaskEvent{
			Type:    taskEventTypeError,
			Stage:   "runtime_failed",
			Status:  model.TaskStatusFailed,
			Message: err.Error(),
		})
		return nil, err
	}
	return state, nil
}

func (s *GenTestService) runWorkflowSelfTest(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
	s.publishTaskEvent(state.Task.ID, TaskEvent{
		Type:    taskEventTypeStage,
		Stage:   "self_test_started",
		Status:  model.TaskStatusRunning,
		Message: "开始执行工作流自测校验",
	})

	var report *SelfTestReport
	err := s.runStageWithRetry(ctx, state.Task.ID, "self_test", 2, func(execCtx context.Context) error {
		var innerErr error
		report, innerErr = s.SelfTestTask(execCtx, state.Task.ID)
		return innerErr
	})
	if err != nil {
		return nil, err
	}

	state.SelfTest = report
	s.publishTaskEvent(state.Task.ID, TaskEvent{
		Type:    taskEventTypeStage,
		Stage:   "self_test_completed",
		Status:  model.TaskStatusRunning,
		Message: report.Summary,
	})

	return state, nil
}

func (s *GenTestService) finalizeWorkflowTask(ctx context.Context, state *genTestWorkflowState) (*genTestWorkflowState, error) {
	if err := s.runStageWithRetry(ctx, state.Task.ID, "finalize_task", 1, func(execCtx context.Context) error {
		return s.FinalizeTask(state.Task.ID, state.SelfTest)
	}); err != nil {
		return nil, err
	}

	refreshedTask, err := s.testTaskRepo.GetByID(state.Task.ID)
	if err == nil {
		state.Task = refreshedTask
	}

	s.publishTaskEvent(state.Task.ID, TaskEvent{
		Type:    taskEventTypeStage,
		Stage:   "workflow_completed",
		Status:  state.Task.Status,
		Message: "Eino 工作流执行完成",
	})

	return state, nil
}

func (s *GenTestService) runStageWithRetry(
	ctx context.Context,
	taskID uint64,
	stage string,
	maxAttempts int,
	action func(context.Context) error,
) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			s.incrementTaskRetryCount(taskID)
			s.publishTaskEvent(taskID, TaskEvent{
				Type:    taskEventTypeLog,
				Stage:   stage,
				Status:  model.TaskStatusRunning,
				Message: fmt.Sprintf("阶段 %s 重试中 (%d/%d)", stage, attempt, maxAttempts),
			})
		}

		if err := action(ctx); err == nil {
			return nil
		} else {
			lastErr = err
			s.publishTaskEvent(taskID, TaskEvent{
				Type:    taskEventTypeLog,
				Stage:   stage,
				Status:  model.TaskStatusRunning,
				Message: fmt.Sprintf("阶段 %s 执行失败 (%d/%d): %v", stage, attempt, maxAttempts, err),
			})
		}

		if attempt == maxAttempts {
			break
		}
		if err := sleepWithContext(ctx, time.Duration(attempt)*time.Second); err != nil {
			return err
		}
	}

	return fmt.Errorf("%s 阶段失败，已达到最大重试次数: %w", stage, lastErr)
}

func (s *GenTestService) incrementTaskRetryCount(taskID uint64) {
	task, err := s.testTaskRepo.GetByID(taskID)
	if err != nil {
		return
	}
	task.RetryCount++
	_ = s.testTaskRepo.Update(task)
}

func findChromeMCPServers(agent *model.Agent) []string {
	if agent == nil || len(agent.MCPServers) == 0 {
		return nil
	}

	servers := make([]string, 0, len(agent.MCPServers))
	for _, server := range agent.MCPServers {
		if server.Status == 0 {
			continue
		}
		candidate := strings.ToLower(strings.Join([]string{
			server.Name,
			server.Description,
			server.Command,
			server.URL,
		}, " "))
		if strings.Contains(candidate, "chrome") || strings.Contains(candidate, "devtools") {
			servers = append(servers, server.Name)
		}
	}
	return servers
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
