package handler

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"auto-test-flow/internal/middleware"
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type KnowledgeHandler struct {
	service *service.KnowledgeService
	logger  *zap.Logger
}

func NewKnowledgeHandler(logger *zap.Logger) *KnowledgeHandler {
	return &KnowledgeHandler{service: service.NewKnowledgeService(logger), logger: logger}
}

func (h *KnowledgeHandler) GetConfig(c *gin.Context) {
	pkg.OK(c, h.service.GetConfig())
}

func (h *KnowledgeHandler) SaveConfig(c *gin.Context) {
	var req service.KnowledgeBaseConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	if err := h.service.SaveConfig(c.Request.Context(), req, middleware.GetCurrentUserID(c)); err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, nil)
}

func (h *KnowledgeHandler) CreateKB(c *gin.Context) {
	var req service.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	kb, err := h.service.CreateKB(req)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OK(c, kb)
}

func (h *KnowledgeHandler) ListKB(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	var page pkg.PageQuery
	_ = c.ShouldBindQuery(&page)
	list, total, err := h.service.ListKB(projectID, c.Query("keyword"), page.GetOffset(), page.PageSize)
	if err != nil {
		pkg.Fail(c, pkg.CodeInternalError, err.Error())
		return
	}
	pkg.OKPage(c, list, total, page.Page, page.PageSize)
}

func (h *KnowledgeHandler) GetKB(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	id, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	kb, err := h.service.GetKB(projectID, id)
	h.writeResult(c, kb, err)
}

func (h *KnowledgeHandler) UpdateKB(c *gin.Context) {
	id, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	var req service.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	kb, err := h.service.UpdateKB(id, req)
	h.writeResult(c, kb, err)
}

func (h *KnowledgeHandler) DeleteKB(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	id, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	err := h.service.DeleteKB(c.Request.Context(), projectID, id)
	h.writeResult(c, nil, err)
}

func (h *KnowledgeHandler) Stats(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	id, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	stats, err := h.service.Stats(projectID, id)
	h.writeResult(c, stats, err)
}

func (h *KnowledgeHandler) AddDocument(c *gin.Context) {
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	req, err := h.bindDocumentRequest(c)
	if err != nil {
		pkg.Fail(c, pkg.CodeParamError, err.Error())
		return
	}
	doc, err := h.service.AddDocument(c.Request.Context(), kbID, req)
	h.writeResult(c, doc, err)
}

func (h *KnowledgeHandler) BatchDocuments(c *gin.Context) {
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	var req service.BatchKnowledgeDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	docs, err := h.service.BatchAddDocuments(c.Request.Context(), kbID, req)
	h.writeResult(c, docs, err)
}

func (h *KnowledgeHandler) ListDocuments(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	var page pkg.PageQuery
	_ = c.ShouldBindQuery(&page)
	docs, total, err := h.service.ListDocuments(projectID, kbID, page.GetOffset(), page.PageSize)
	if err != nil {
		h.writeResult(c, nil, err)
		return
	}
	pkg.OKPage(c, docs, total, page.Page, page.PageSize)
}

func (h *KnowledgeHandler) RebuildDocument(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	docID, ok := parsePathUint64(c, "docId")
	if !ok {
		return
	}
	h.writeResult(c, nil, h.service.RebuildDocument(c.Request.Context(), projectID, kbID, docID))
}

func (h *KnowledgeHandler) DeleteDocument(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	docID, ok := parsePathUint64(c, "docId")
	if !ok {
		return
	}
	h.writeResult(c, nil, h.service.DeleteDocument(c.Request.Context(), projectID, kbID, docID))
}

func (h *KnowledgeHandler) RebuildKB(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	h.writeResult(c, nil, h.service.RebuildKB(c.Request.Context(), projectID, kbID))
}

func (h *KnowledgeHandler) Query(c *gin.Context) {
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	var req service.KnowledgeQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	results, err := h.service.Query(c.Request.Context(), kbID, req)
	h.writeResult(c, results, err)
}

func (h *KnowledgeHandler) Chat(c *gin.Context) {
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	var req service.KnowledgeChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.CodeParamError, "参数错误: "+err.Error())
		return
	}
	result, err := h.service.Chat(c.Request.Context(), kbID, req)
	h.writeResult(c, result, err)
}

func (h *KnowledgeHandler) Graph(c *gin.Context) {
	projectID, ok := parseRequiredUint64(c, "project_id")
	if !ok {
		return
	}
	kbID, ok := parsePathUint64(c, "id")
	if !ok {
		return
	}
	graph, err := h.service.Graph(projectID, kbID)
	h.writeResult(c, graph, err)
}

func (h *KnowledgeHandler) bindDocumentRequest(c *gin.Context) (service.KnowledgeDocumentRequest, error) {
	var req service.KnowledgeDocumentRequest
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		projectID, _ := strconv.ParseUint(c.PostForm("project_id"), 10, 64)
		req.ProjectID = projectID
		req.SourceType = c.PostForm("source_type")
		req.SourcePath = c.PostForm("source_path")
		req.Title = c.PostForm("title")
		req.Content = c.PostForm("content")
		file, err := c.FormFile("file")
		if err == nil && file != nil {
			opened, err := file.Open()
			if err != nil {
				return req, err
			}
			defer opened.Close()
			body, err := io.ReadAll(io.LimitReader(opened, 4*1024*1024))
			if err != nil {
				return req, err
			}
			req.Content = string(body)
			if req.Title == "" {
				req.Title = file.Filename
			}
			if req.SourcePath == "" {
				req.SourcePath = file.Filename
			}
			if req.SourceType == "" {
				req.SourceType = "markdown"
			}
		}
	} else if err := c.ShouldBindJSON(&req); err != nil {
		return req, err
	}
	if req.ProjectID == 0 {
		return req, errors.New("project_id 必填")
	}
	return req, nil
}

func (h *KnowledgeHandler) writeResult(c *gin.Context, data any, err error) {
	if err == nil {
		pkg.OK(c, data)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pkg.Forbidden(c, "知识库不存在或无权访问该项目")
		return
	}
	pkg.Fail(c, pkg.CodeInternalError, err.Error())
}

func parseRequiredUint64(c *gin.Context, key string) (uint64, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		pkg.Fail(c, pkg.CodeParamError, key+" 必填")
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		pkg.Fail(c, pkg.CodeParamError, key+" 格式错误")
		return 0, false
	}
	return parsed, true
}

func parsePathUint64(c *gin.Context, key string) (uint64, bool) {
	parsed, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || parsed == 0 {
		pkg.Fail(c, pkg.CodeParamError, key+" 格式错误")
		return 0, false
	}
	return parsed, true
}
