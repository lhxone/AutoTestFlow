package handler

import (
	"strconv"

	"auto-test-flow/internal/dto"
	"auto-test-flow/internal/model"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/repository"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AgentHandler struct {
	agentRepo     *repository.AgentRepo
	skillRepo     *repository.SkillRepo
	mcpServerRepo *repository.MCPServerRepo
	agentService  *service.AgentService
}

func NewAgentHandler(logger *zap.Logger) *AgentHandler {
	return &AgentHandler{
		agentRepo:     repository.NewAgentRepo(),
		skillRepo:     repository.NewSkillRepo(),
		mcpServerRepo: repository.NewMCPServerRepo(),
		agentService:  service.NewAgentService(logger),
	}
}

// ListAgents Agent列表
// GET /api/agents
func (h *AgentHandler) ListAgents(c *gin.Context) {
	var query dto.AgentListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误")
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	offset := (query.Page - 1) * query.PageSize

	agents, total, err := h.agentRepo.List(query.Keyword, query.Status, offset, query.PageSize)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OKPage(c, agents, total, query.Page, query.PageSize)
}

// CreateAgent 创建Agent
// POST /api/agents
func (h *AgentHandler) CreateAgent(c *gin.Context) {
	var req dto.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	agent := &model.Agent{
		Name:          req.Name,
		Description:   req.Description,
		IsDefault:     req.IsDefault,
		ModelProvider: req.ModelProvider,
		ModelName:     req.ModelName,
		APIKeyRef:     req.APIKeyRef,
		BaseURL:       req.BaseURL,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		Status:        1,
		ConfigJSON:    model.JSON(req.ConfigJSON),
	}

	if agent.MaxTokens == 0 {
		agent.MaxTokens = 4096
	}

	if err := h.agentRepo.Create(agent); err != nil {
		pkg.Fail(c, pkg.CodeDuplicate, "Agent名称已存在")
		return
	}
	if req.IsDefault {
		if err := h.agentRepo.SetDefault(agent.ID); err != nil {
			pkg.Fail(c, pkg.CodeInternalError, "设置默认Agent失败: "+err.Error())
			return
		}
	}

	// 绑定Workflows
	_ = h.agentRepo.SetSkills(agent.ID, req.WorkflowIDs)
	// 绑定MCP Servers
	if len(req.MCPServerIDs) > 0 {
		_ = h.agentRepo.SetMCPServers(agent.ID, req.MCPServerIDs)
	}

	agent, _ = h.agentRepo.GetByID(agent.ID)
	pkg.OK(c, agent)
}

// GetAgent Agent详情
// GET /api/agents/:id
func (h *AgentHandler) GetAgent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	agent, err := h.agentRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "Agent不存在")
		return
	}

	pkg.OK(c, agent)
}

// UpdateAgent 更新Agent
// PUT /api/agents/:id
func (h *AgentHandler) UpdateAgent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	agent, err := h.agentRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "Agent不存在")
		return
	}

	var req dto.UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	if req.Name != "" {
		agent.Name = req.Name
	}
	if req.Description != "" {
		agent.Description = req.Description
	}
	if req.IsDefault != nil {
		agent.IsDefault = *req.IsDefault
	}
	if req.ModelProvider != "" {
		agent.ModelProvider = req.ModelProvider
	}
	if req.ModelName != "" {
		agent.ModelName = req.ModelName
	}
	if req.APIKeyRef != "" {
		agent.APIKeyRef = req.APIKeyRef
	}
	if req.BaseURL != "" {
		agent.BaseURL = req.BaseURL
	}
	if req.MaxTokens != nil {
		agent.MaxTokens = *req.MaxTokens
	}
	if req.Temperature != nil {
		agent.Temperature = *req.Temperature
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}
	// 始终更新 config_json
	agent.ConfigJSON = model.JSON(req.ConfigJSON)

	if err := h.agentRepo.Update(agent); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, "更新Agent失败: "+err.Error())
		return
	}
	if req.IsDefault != nil && *req.IsDefault {
		if err := h.agentRepo.SetDefault(id); err != nil {
			pkg.Fail(c, pkg.CodeInternalError, "设置默认Agent失败: "+err.Error())
			return
		}
	}

	_ = h.agentRepo.SetSkills(id, req.WorkflowIDs)
	if len(req.MCPServerIDs) > 0 {
		_ = h.agentRepo.SetMCPServers(id, req.MCPServerIDs)
	}

	agent, _ = h.agentRepo.GetByID(id)
	pkg.OK(c, agent)
}

// DeleteAgent 删除Agent
// DELETE /api/agents/:id
func (h *AgentHandler) DeleteAgent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	if err := h.agentRepo.Delete(id); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// TestConnection 测试Agent连接
// POST /api/agents/test
func (h *AgentHandler) TestConnection(c *gin.Context) {
	var req dto.TestAgentConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	result := h.agentService.TestConnection(&req)
	pkg.OK(c, result)
}

// ListWorkflows Workflow列表
// GET /api/workflows
func (h *AgentHandler) ListWorkflows(c *gin.Context) {
	skills, err := h.skillRepo.ListAll()
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, skills)
}

// CreateWorkflow 创建Workflow
// POST /api/workflows
func (h *AgentHandler) CreateWorkflow(c *gin.Context) {
	var req dto.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	skill := &model.Skill{
		Name:           req.Name,
		Description:    req.Description,
		SkillType:      req.WorkflowType,
		PromptTemplate: req.PromptTemplate,
		InputSchema:    model.JSON(req.InputSchema),
		OutputSchema:   model.JSON(req.OutputSchema),
		ConfigJSON:     model.JSON(req.ConfigJSON),
		Status:         1,
	}

	if err := h.skillRepo.Create(skill); err != nil {
		pkg.Fail(c, pkg.CodeDuplicate, "Workflow名称已存在")
		return
	}

	pkg.OK(c, skill)
}

// UpdateWorkflow 更新Workflow
// PUT /api/workflows/:id
func (h *AgentHandler) UpdateWorkflow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	skill, err := h.skillRepo.GetByID(id)
	if err != nil {
		pkg.Fail(c, pkg.CodeNotFound, "Workflow不存在")
		return
	}

	var req dto.UpdateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	if req.Name != "" {
		skill.Name = req.Name
	}
	if req.Description != "" {
		skill.Description = req.Description
	}
	if req.WorkflowType != "" {
		skill.SkillType = req.WorkflowType
	}
	if req.PromptTemplate != "" {
		skill.PromptTemplate = req.PromptTemplate
	}
	if len(req.InputSchema) > 0 {
		skill.InputSchema = model.JSON(req.InputSchema)
	}
	if len(req.OutputSchema) > 0 {
		skill.OutputSchema = model.JSON(req.OutputSchema)
	}
	if len(req.ConfigJSON) > 0 {
		skill.ConfigJSON = model.JSON(req.ConfigJSON)
	}
	if req.Status != nil {
		skill.Status = *req.Status
	}

	if err := h.skillRepo.Update(skill); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, skill)
}

// DeleteWorkflow 删除Workflow
// DELETE /api/workflows/:id
func (h *AgentHandler) DeleteWorkflow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, "无效的ID")
		return
	}

	if err := h.skillRepo.Delete(id); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}

	pkg.OK(c, nil)
}

// ListSkills 兼容旧Skill接口
func (h *AgentHandler) ListSkills(c *gin.Context) { h.ListWorkflows(c) }

// CreateSkill 兼容旧Skill接口
func (h *AgentHandler) CreateSkill(c *gin.Context) { h.CreateWorkflow(c) }

// UpdateSkill 兼容旧Skill接口
func (h *AgentHandler) UpdateSkill(c *gin.Context) { h.UpdateWorkflow(c) }

// DeleteSkill 兼容旧Skill接口
func (h *AgentHandler) DeleteSkill(c *gin.Context) { h.DeleteWorkflow(c) }

// ListMCPServers MCP Server列表
// GET /api/mcp-servers
func (h *AgentHandler) ListMCPServers(c *gin.Context) {
	servers, err := h.mcpServerRepo.ListAll()
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, servers)
}

// CreateMCPServer 创建MCP Server
// POST /api/mcp-servers
func (h *AgentHandler) CreateMCPServer(c *gin.Context) {
	var req dto.CreateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}

	server := &model.MCPServer{
		Name:        req.Name,
		Description: req.Description,
		ServerType:  req.ServerType,
		Command:     req.Command,
		Args:        model.JSON(req.Args),
		URL:         req.URL,
		EnvVars:     model.JSON(req.EnvVars),
		Status:      1,
	}

	if err := h.mcpServerRepo.Create(server); err != nil {
		pkg.Fail(c, pkg.CodeDuplicate, "MCP Server名称已存在")
		return
	}

	pkg.OK(c, server)
}
