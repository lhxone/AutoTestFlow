package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"auto-test-flow/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

const (
	mcpToolKindCallTool     = "call_tool"
	mcpToolKindReadResource = "read_resource"
	mcpToolKindGetPrompt    = "get_prompt"
	maxMCPToolResultLength  = 8000
)

type MCPRuntime struct {
	logger          *zap.Logger
	sessions        []*mcp.ClientSession
	toolBindings    map[string]*MCPToolBinding
	capabilityNotes []string
}

type MCPToolBinding struct {
	ModelToolName string
	ServerName    string
	Description   string
	Kind          string
	Session       *mcp.ClientSession
	ToolName      string
	InputSchema   map[string]any
}

func NewMCPRuntime(ctx context.Context, logger *zap.Logger, agent *model.Agent) (*MCPRuntime, error) {
	runtime := &MCPRuntime{
		logger:       logger,
		toolBindings: make(map[string]*MCPToolBinding),
	}
	if agent == nil || len(agent.MCPServers) == 0 {
		return runtime, nil
	}

	for _, server := range agent.MCPServers {
		if server.Status == 0 {
			continue
		}
		if err := runtime.attachServer(ctx, server); err != nil {
			logger.Warn("连接 MCP Server 失败，已忽略",
				zap.String("server_name", server.Name),
				zap.String("server_type", server.ServerType),
				zap.Error(err))
		}
	}

	return runtime, nil
}

func (r *MCPRuntime) Close() {
	for _, session := range r.sessions {
		_ = session.Close()
	}
}

func (r *MCPRuntime) HasTools() bool {
	return len(r.toolBindings) > 0
}

func (r *MCPRuntime) CapabilitySummary() string {
	if len(r.capabilityNotes) == 0 {
		return ""
	}
	return strings.Join(r.capabilityNotes, "\n")
}

func (r *MCPRuntime) OpenAITools() []map[string]any {
	if len(r.toolBindings) == 0 {
		return nil
	}
	tools := make([]map[string]any, 0, len(r.toolBindings))
	for _, binding := range r.toolBindings {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        binding.ModelToolName,
				"description": binding.Description,
				"parameters":  normalizeJSONSchema(binding.InputSchema),
			},
		})
	}
	return tools
}

func (r *MCPRuntime) AnthropicTools() []map[string]any {
	if len(r.toolBindings) == 0 {
		return nil
	}
	tools := make([]map[string]any, 0, len(r.toolBindings))
	for _, binding := range r.toolBindings {
		tools = append(tools, map[string]any{
			"name":         binding.ModelToolName,
			"description":  binding.Description,
			"input_schema": normalizeJSONSchema(binding.InputSchema),
		})
	}
	return tools
}

func (r *MCPRuntime) Invoke(ctx context.Context, toolName string, input map[string]any) (string, bool) {
	binding, ok := r.toolBindings[toolName]
	if !ok {
		return "", false
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var (
		result string
		err    error
	)
	switch binding.Kind {
	case mcpToolKindCallTool:
		result, err = r.invokeMCPTool(callCtx, binding, input)
	case mcpToolKindReadResource:
		result, err = r.readMCPResource(callCtx, binding, input)
	case mcpToolKindGetPrompt:
		result, err = r.getMCPPrompt(callCtx, binding, input)
	default:
		err = fmt.Errorf("不支持的 MCP 工具类型: %s", binding.Kind)
	}

	if err != nil {
		r.logger.Warn("执行 MCP 工具失败",
			zap.String("tool_name", toolName),
			zap.String("server_name", binding.ServerName),
			zap.Error(err))
		return truncateMCPResult("ERROR: " + err.Error()), true
	}

	r.logger.Info("执行 MCP 工具完成",
		zap.String("tool_name", toolName),
		zap.String("server_name", binding.ServerName))

	return truncateMCPResult(result), true
}

func (r *MCPRuntime) attachServer(ctx context.Context, server model.MCPServer) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "auto-test-flow",
		Version: "1.0.0",
	}, nil)

	connectCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	session, err := client.Connect(connectCtx, buildMCPTransport(server), nil)
	if err != nil {
		return err
	}
	r.sessions = append(r.sessions, session)

	info := session.InitializeResult()
	serverLabel := server.Name
	if info != nil && info.ServerInfo != nil && strings.TrimSpace(info.ServerInfo.Name) != "" {
		serverLabel = fmt.Sprintf("%s (%s)", server.Name, info.ServerInfo.Name)
	}

	if info != nil && strings.TrimSpace(info.Instructions) != "" {
		r.capabilityNotes = append(r.capabilityNotes,
			fmt.Sprintf("MCP Server %s 使用说明: %s", serverLabel, strings.TrimSpace(info.Instructions)))
	}

	if tools, err := session.ListTools(ctx, nil); err == nil && len(tools.Tools) > 0 {
		names := make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
			r.registerMCPTool(server, session, tool)
		}
		r.capabilityNotes = append(r.capabilityNotes,
			fmt.Sprintf("MCP Server %s 可用工具: %s", serverLabel, strings.Join(names, ", ")))
	}

	if prompts, err := session.ListPrompts(ctx, nil); err == nil && len(prompts.Prompts) > 0 {
		names := make([]string, 0, len(prompts.Prompts))
		for _, prompt := range prompts.Prompts {
			names = append(names, prompt.Name)
		}
		r.registerPromptTool(server, session, prompts.Prompts)
		r.capabilityNotes = append(r.capabilityNotes,
			fmt.Sprintf("MCP Server %s 可用 prompts: %s", serverLabel, strings.Join(names, ", ")))
	}

	resources, _ := session.ListResources(ctx, nil)
	resourceTemplates, _ := session.ListResourceTemplates(ctx, nil)
	if (resources != nil && len(resources.Resources) > 0) || (resourceTemplates != nil && len(resourceTemplates.ResourceTemplates) > 0) {
		r.registerResourceTool(server, session, resources, resourceTemplates)
		resourceHints := make([]string, 0, 8)
		if resources != nil {
			for _, resource := range resources.Resources {
				resourceHints = append(resourceHints, resource.URI)
			}
		}
		if resourceTemplates != nil {
			for _, template := range resourceTemplates.ResourceTemplates {
				resourceHints = append(resourceHints, template.URITemplate)
			}
		}
		r.capabilityNotes = append(r.capabilityNotes,
			fmt.Sprintf("MCP Server %s 可读取资源: %s", serverLabel, strings.Join(resourceHints, ", ")))
	}

	return nil
}

func (r *MCPRuntime) registerMCPTool(server model.MCPServer, session *mcp.ClientSession, tool *mcp.Tool) {
	modelToolName := buildModelToolName(server.Name, tool.Name)
	r.toolBindings[modelToolName] = &MCPToolBinding{
		ModelToolName: modelToolName,
		ServerName:    server.Name,
		Description:   buildToolDescription(server.Name, tool.Description, "MCP tool"),
		Kind:          mcpToolKindCallTool,
		Session:       session,
		ToolName:      tool.Name,
		InputSchema:   coerceJSONSchema(tool.InputSchema),
	}
}

func (r *MCPRuntime) registerPromptTool(server model.MCPServer, session *mcp.ClientSession, prompts []*mcp.Prompt) {
	modelToolName := buildModelToolName(server.Name, "get-prompt")
	names := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		names = append(names, prompt.Name)
	}
	r.toolBindings[modelToolName] = &MCPToolBinding{
		ModelToolName: modelToolName,
		ServerName:    server.Name,
		Description:   fmt.Sprintf("Read an MCP prompt from server %s. Available prompt names: %s", server.Name, strings.Join(names, ", ")),
		Kind:          mcpToolKindGetPrompt,
		Session:       session,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
					"enum": names,
				},
				"arguments": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
			"required": []string{"name"},
		},
	}
}

func (r *MCPRuntime) registerResourceTool(server model.MCPServer, session *mcp.ClientSession, resources *mcp.ListResourcesResult, templates *mcp.ListResourceTemplatesResult) {
	hints := make([]string, 0, 8)
	if resources != nil {
		for _, resource := range resources.Resources {
			hints = append(hints, resource.URI)
		}
	}
	if templates != nil {
		for _, template := range templates.ResourceTemplates {
			hints = append(hints, template.URITemplate)
		}
	}
	modelToolName := buildModelToolName(server.Name, "read-resource")
	r.toolBindings[modelToolName] = &MCPToolBinding{
		ModelToolName: modelToolName,
		ServerName:    server.Name,
		Description:   fmt.Sprintf("Read an MCP resource from server %s. Known URIs or templates: %s", server.Name, strings.Join(hints, ", ")),
		Kind:          mcpToolKindReadResource,
		Session:       session,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uri": map[string]any{
					"type":        "string",
					"description": "Resource URI to read from the MCP server",
				},
			},
			"required": []string{"uri"},
		},
	}
}

func buildMCPTransport(server model.MCPServer) mcp.Transport {
	switch server.ServerType {
	case "stdio":
		args := parseJSONArray(server.Args)
		cmd := exec.Command(strings.TrimSpace(server.Command), args...)
		cmd.Env = mergeCommandEnv(server.EnvVars)
		return &mcp.CommandTransport{Command: cmd}
	case "sse":
		return &mcp.SSEClientTransport{
			Endpoint:   strings.TrimSpace(server.URL),
			HTTPClient: &http.Client{Timeout: 30 * time.Second},
		}
	default:
		return &mcp.StreamableClientTransport{
			Endpoint:   strings.TrimSpace(server.URL),
			HTTPClient: &http.Client{Timeout: 30 * time.Second},
		}
	}
}

func mergeCommandEnv(raw model.JSON) []string {
	envMap := parseJSONStringMap(raw)
	if len(envMap) == 0 {
		return nil
	}
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return append(os.Environ(), env...)
}

func parseJSONArray(raw model.JSON) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var anyList []any
	if err := json.Unmarshal(raw, &anyList); err != nil {
		return nil
	}
	list = make([]string, 0, len(anyList))
	for _, item := range anyList {
		list = append(list, strings.TrimSpace(fmt.Sprint(item)))
	}
	return list
}

func parseJSONStringMap(raw model.JSON) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var result map[string]string
	if err := json.Unmarshal(raw, &result); err == nil {
		return result
	}
	var anyMap map[string]any
	if err := json.Unmarshal(raw, &anyMap); err != nil {
		return nil
	}
	result = make(map[string]string, len(anyMap))
	for key, value := range anyMap {
		result[key] = fmt.Sprint(value)
	}
	return result
}

func buildModelToolName(serverName, toolName string) string {
	sanitize := func(value string) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return "unnamed"
		}
		var builder strings.Builder
		for _, r := range value {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				builder.WriteRune(r)
			case r == '-', r == '_', r == '.':
				builder.WriteRune(r)
			default:
				builder.WriteByte('-')
			}
		}
		return strings.Trim(builder.String(), "-")
	}
	return fmt.Sprintf("mcp.%s.%s", sanitize(serverName), sanitize(toolName))
}

func buildToolDescription(serverName, description, kind string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return fmt.Sprintf("Invoke %s on MCP server %s", kind, serverName)
	}
	return fmt.Sprintf("%s (MCP server: %s)", description, serverName)
}

func coerceJSONSchema(raw any) map[string]any {
	if raw == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	switch value := raw.(type) {
	case map[string]any:
		return normalizeJSONSchema(value)
	case json.RawMessage:
		var schema map[string]any
		if err := json.Unmarshal(value, &schema); err == nil {
			return normalizeJSONSchema(schema)
		}
	case []byte:
		var schema map[string]any
		if err := json.Unmarshal(value, &schema); err == nil {
			return normalizeJSONSchema(schema)
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return normalizeJSONSchema(schema)
}

func normalizeJSONSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	if schema["type"] == "object" {
		if _, ok := schema["properties"]; !ok {
			schema["properties"] = map[string]any{}
		}
	}
	return schema
}

func (r *MCPRuntime) invokeMCPTool(ctx context.Context, binding *MCPToolBinding, input map[string]any) (string, error) {
	result, err := binding.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      binding.ToolName,
		Arguments: input,
	})
	if err != nil {
		return "", err
	}
	return formatCallToolResult(result), nil
}

func (r *MCPRuntime) readMCPResource(ctx context.Context, binding *MCPToolBinding, input map[string]any) (string, error) {
	uri, _ := input["uri"].(string)
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", fmt.Errorf("缺少 uri 参数")
	}
	result, err := binding.Session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		return "", err
	}
	return formatReadResourceResult(result), nil
}

func (r *MCPRuntime) getMCPPrompt(ctx context.Context, binding *MCPToolBinding, input map[string]any) (string, error) {
	name, _ := input["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("缺少 prompt name 参数")
	}
	args := make(map[string]string)
	if rawArgs, ok := input["arguments"].(map[string]any); ok {
		for key, value := range rawArgs {
			args[key] = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	result, err := binding.Session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}
	return formatGetPromptResult(name, result), nil
}

func formatCallToolResult(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if result.StructuredContent != nil {
		if data, err := json.MarshalIndent(result.StructuredContent, "", "  "); err == nil {
			parts = append(parts, "structured:\n"+string(data))
		}
	}
	if len(result.Content) > 0 {
		parts = append(parts, formatMCPContents(result.Content))
	}
	if result.IsError {
		parts = append(parts, "tool_call_status: error")
	}
	return strings.Join(parts, "\n")
}

func formatReadResourceResult(result *mcp.ReadResourceResult) string {
	if result == nil || len(result.Contents) == 0 {
		return ""
	}
	parts := make([]string, 0, len(result.Contents))
	for _, content := range result.Contents {
		if content == nil {
			continue
		}
		if content.Text != "" {
			parts = append(parts, fmt.Sprintf("resource %s:\n%s", content.URI, content.Text))
			continue
		}
		if len(content.Blob) > 0 {
			parts = append(parts, fmt.Sprintf("resource %s: [binary content %d bytes, mime=%s]", content.URI, len(content.Blob), content.MIMEType))
		}
	}
	return strings.Join(parts, "\n")
}

func formatGetPromptResult(name string, result *mcp.GetPromptResult) string {
	if result == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("prompt %s", name)}
	if strings.TrimSpace(result.Description) != "" {
		parts = append(parts, result.Description)
	}
	for idx, message := range result.Messages {
		if message == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%d] %s:\n%s", idx+1, message.Role, formatMCPContents([]mcp.Content{message.Content})))
	}
	return strings.Join(parts, "\n")
}

func formatMCPContents(contents []mcp.Content) string {
	parts := make([]string, 0, len(contents))
	for _, content := range contents {
		switch value := content.(type) {
		case *mcp.TextContent:
			parts = append(parts, value.Text)
		case *mcp.ImageContent:
			parts = append(parts, fmt.Sprintf("[image mime=%s bytes=%d]", value.MIMEType, len(value.Data)))
		case *mcp.AudioContent:
			parts = append(parts, fmt.Sprintf("[audio mime=%s bytes=%d]", value.MIMEType, len(value.Data)))
		case *mcp.ResourceLink:
			parts = append(parts, fmt.Sprintf("[resource_link %s]", value.URI))
		case *mcp.EmbeddedResource:
			if value.Resource == nil {
				continue
			}
			if value.Resource.Text != "" {
				parts = append(parts, fmt.Sprintf("[resource %s]\n%s", value.Resource.URI, value.Resource.Text))
			} else if len(value.Resource.Blob) > 0 {
				parts = append(parts, fmt.Sprintf("[resource %s binary=%d mime=%s]", value.Resource.URI, len(value.Resource.Blob), value.Resource.MIMEType))
			}
		default:
			data, _ := json.Marshal(value)
			parts = append(parts, string(data))
		}
	}
	return strings.Join(parts, "\n")
}

func truncateMCPResult(result string) string {
	result = strings.TrimSpace(result)
	if len(result) <= maxMCPToolResultLength {
		return result
	}
	return result[:maxMCPToolResultLength] + "\n...[truncated]"
}
