package handler

import (
	"auto-test-flow/internal/pkg"
	"auto-test-flow/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ZentaoProxyHandler struct {
	zentaoProxy *service.ZentaoProxyService
}

func NewZentaoProxyHandler(logger *zap.Logger) *ZentaoProxyHandler {
	return &ZentaoProxyHandler{
		zentaoProxy: service.NewZentaoProxyService(logger),
	}
}

// GetProjects 获取禅道项目列表(代理接口)
// GET /api/zentao/projects
func (h *ZentaoProxyHandler) GetProjects(c *gin.Context) {
	projects, err := h.zentaoProxy.GetProjects()
	if err != nil {
		pkg.Fail(c, pkg.CodeZentaoError, err.Error())
		return
	}
	pkg.OK(c, projects)
}

// GetProducts 获取禅道产品列表(代理接口)
// GET /api/zentao/products
func (h *ZentaoProxyHandler) GetProducts(c *gin.Context) {
	products, err := h.zentaoProxy.GetProducts()
	if err != nil {
		pkg.Fail(c, pkg.CodeZentaoError, err.Error())
		return
	}
	pkg.OK(c, products)
}

// GetBranches 获取禅道产品分支列表(代理接口)
// GET /api/zentao/products/:id/branches
func (h *ZentaoProxyHandler) GetBranches(c *gin.Context) {
	productID := c.Param("id")
	branches, err := h.zentaoProxy.GetBranches(productID)
	if err != nil {
		pkg.Fail(c, pkg.CodeZentaoError, err.Error())
		return
	}
	pkg.OK(c, branches)
}
