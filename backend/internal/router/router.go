package router

import (
	"auto-test-flow/internal/handler"
	"auto-test-flow/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Setup 初始化路由
func Setup(logger *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.CORS())

	// 初始化 Handlers
	authH := handler.NewAuthHandler()
	userH := handler.NewUserHandler()
	dashboardH := handler.NewDashboardHandler()
	projectH := handler.NewProjectHandler()
	issueH := handler.NewIssueHandler(logger)
	agentH := handler.NewAgentHandler(logger)
	reviewH := handler.NewReviewHandler(logger)
	testTaskH := handler.NewTestTaskHandler(logger)
	execH := handler.NewExecutionHandler(logger)
	ciCallbackH := handler.NewCICallbackHandler(logger)
	settingH := handler.NewSettingHandler(logger)
	zentaoProxyH := handler.NewZentaoProxyHandler(logger)
	cliInteractionH := handler.NewCLIInteractionHandler(nil)
	zentaoTestCaseH := handler.NewZentaoTestCaseHandler(logger)
	integrationH := handler.NewIntegrationHandler(logger)
	knowledgeH := handler.NewKnowledgeHandler(logger)

	api := r.Group("/api")
	{
		// ====== 公开接口(无需认证) ======
		auth := api.Group("/auth")
		{
			auth.POST("/login", authH.Login)
			auth.POST("/refresh", authH.RefreshToken)
		}

		// ====== 需要认证的接口 ======
		protected := api.Group("")
		protected.Use(middleware.JWTAuth())
		{
			// 认证相关
			protected.GET("/auth/me", authH.GetCurrentUser)
			protected.PUT("/auth/password", authH.ChangePassword)
			protected.POST("/auth/logout", authH.Logout)

			// 工作台
			protected.GET("/dashboard/stats", dashboardH.GetStats)
			protected.GET("/dashboard/recent-activities", dashboardH.GetRecentActivities)
			protected.GET("/dashboard/monitor", dashboardH.GetMonitorKPI)

			// 用户管理
			users := protected.Group("/users")
			{
				users.GET("", middleware.RequirePermission("user:list"), userH.List)
				users.GET("/login-logs", middleware.RequirePermission("user:list"), userH.ListLoginLogs)
				users.POST("", middleware.RequirePermission("user:create"), userH.Create)
				users.GET("/:id", middleware.RequirePermission("user:list"), userH.GetByID)
				users.PUT("/:id", middleware.RequirePermission("user:update"), userH.Update)
				users.DELETE("/:id", middleware.RequirePermission("user:delete"), userH.Delete)
			}

			// 项目管理
			projects := protected.Group("/projects")
			{
				projects.GET("", middleware.RequirePermission("project:list"), projectH.List)
				projects.POST("", middleware.RequirePermission("project:create"), projectH.Create)
				projects.GET("/:id", middleware.RequirePermission("project:list"), projectH.GetByID)
				projects.GET("/:id/issue-sync-logs", middleware.RequirePermission("project:list"), projectH.ListIssueSyncLogs)
				projects.GET("/:id/issue-sync-logs/:logId", middleware.RequirePermission("project:list"), projectH.GetIssueSyncLogDetail)
				projects.PUT("/:id", middleware.RequirePermission("project:update"), projectH.Update)
				projects.DELETE("/:id", middleware.RequirePermission("project:delete"), projectH.Delete)
			}

			// 全局采集记录
			issueSyncLogs := protected.Group("/issue-sync-logs")
			{
				issueSyncLogs.GET("", middleware.RequirePermission("project:list"), projectH.ListAllIssueSyncLogs)
				issueSyncLogs.GET("/:logId", middleware.RequirePermission("project:list"), projectH.GetIssueSyncLogDetailByID)
			}

			// 问题单
			issues := protected.Group("/issues")
			{
				issues.GET("", middleware.RequirePermission("issue:list"), issueH.List)
				issues.GET("/:id", middleware.RequirePermission("issue:list"), issueH.GetByID)
				issues.POST("/sync", middleware.RequirePermission("issue:sync"), issueH.Sync)
				issues.PUT("/:id/test-status", middleware.RequirePermission("issue:update"), issueH.UpdateTestStatus)
				issues.GET("/:id/interventions", middleware.RequirePermission("test:list"), testTaskH.GetInterventionHistory)
			}

			// Agent管理
			agents := protected.Group("/agents")
			{
				agents.GET("", middleware.RequirePermission("agent:list"), agentH.ListAgents)
				agents.POST("", middleware.RequirePermission("agent:manage"), agentH.CreateAgent)
				agents.POST("/test", middleware.RequirePermission("agent:manage"), agentH.TestConnection)
				agents.GET("/:id", middleware.RequirePermission("agent:list"), agentH.GetAgent)
				agents.PUT("/:id", middleware.RequirePermission("agent:manage"), agentH.UpdateAgent)
				agents.DELETE("/:id", middleware.RequirePermission("agent:manage"), agentH.DeleteAgent)
			}

			// Workflow管理
			workflows := protected.Group("/workflows")
			{
				workflows.GET("", middleware.RequirePermission("agent:list"), agentH.ListWorkflows)
				workflows.POST("", middleware.RequirePermission("agent:manage"), agentH.CreateWorkflow)
				workflows.PUT("/:id", middleware.RequirePermission("agent:manage"), agentH.UpdateWorkflow)
				workflows.DELETE("/:id", middleware.RequirePermission("agent:manage"), agentH.DeleteWorkflow)
			}

			// Skill管理(兼容旧接口)
			skills := protected.Group("/skills")
			{
				skills.GET("", middleware.RequirePermission("agent:list"), agentH.ListSkills)
				skills.POST("", middleware.RequirePermission("agent:manage"), agentH.CreateSkill)
				skills.PUT("/:id", middleware.RequirePermission("agent:manage"), agentH.UpdateSkill)
				skills.DELETE("/:id", middleware.RequirePermission("agent:manage"), agentH.DeleteSkill)
			}

			// MCP Server管理
			mcpServers := protected.Group("/mcp-servers")
			{
				mcpServers.GET("", middleware.RequirePermission("agent:list"), agentH.ListMCPServers)
				mcpServers.POST("", middleware.RequirePermission("agent:manage"), agentH.CreateMCPServer)
				mcpServers.PUT("/:id", middleware.RequirePermission("agent:manage"), agentH.UpdateMCPServer)
				mcpServers.DELETE("/:id", middleware.RequirePermission("agent:manage"), agentH.DeleteMCPServer)
			}

			// Review审核
			reviews := protected.Group("/reviews")
			{
				reviews.GET("", middleware.RequirePermission("review:list"), reviewH.List)
				reviews.GET("/:id", middleware.RequirePermission("review:list"), reviewH.GetDetail)
				reviews.POST("/:id/review", middleware.RequirePermission("review:approve"), reviewH.DoReview)
			}

			// 测试任务
			testTasks := protected.Group("/test-tasks")
			{
				testTasks.GET("", middleware.RequirePermission("test:list"), testTaskH.List)
				testTasks.POST("", middleware.RequirePermission("test:create"), testTaskH.Create)
				testTasks.GET("/:id", middleware.RequirePermission("test:execute"), testTaskH.GetByID)
				testTasks.PUT("/:id", middleware.RequirePermission("test:execute"), testTaskH.Update)
				testTasks.DELETE("/:id", middleware.RequirePermission("test:execute"), testTaskH.Delete)
				testTasks.POST("/:id/generate", middleware.RequirePermission("test:execute"), testTaskH.GenerateTest)
				testTasks.POST("/:id/publish", middleware.RequirePermission("test:execute"), testTaskH.Publish)
				testTasks.GET("/:id/events", middleware.RequirePermission("test:execute"), testTaskH.StreamEvents)
				testTasks.GET("/:id/logs", middleware.RequirePermission("test:execute"), testTaskH.GetLogs)
				testTasks.GET("/:id/cases", middleware.RequirePermission("test:execute"), testTaskH.GetTestCases)
				testTasks.GET("/:id/scripts", middleware.RequirePermission("test:execute"), testTaskH.GetTestScripts)
				testTasks.GET("/:id/self-test-report", middleware.RequirePermission("test:execute"), testTaskH.GetSelfTestReport)
				testTasks.GET("/:id/workspace/*filepath", middleware.RequirePermission("test:execute"), testTaskH.GetWorkspaceFile)

				// CLI交互
				cliInteractionH.RegisterRoutes(testTasks)
			}

			// 人工修改接口
			protected.GET("/test-cases", middleware.RequirePermission("test:list"), testTaskH.ListTestCases)
			protected.PUT("/test-cases/:id", middleware.RequirePermission("test:intervene"), testTaskH.UpdateTestCase)
			protected.PUT("/test-scripts/:id", middleware.RequirePermission("test:intervene"), testTaskH.UpdateTestScript)

			// 测试执行记录
			protected.GET("/executions", middleware.RequirePermission("test:list"), testTaskH.ListExecutions)

			// 触发测试执行
			protected.POST("/executions/trigger", middleware.RequirePermission("test:trigger"), execH.TriggerExecution)

			// 系统设置(仅管理员)
			settings := protected.Group("/settings")
			settings.Use(middleware.RequireRoles("admin"))
			{
				// 禅道配置
				settings.GET("/zentao", settingH.GetZentaoSettings)
				settings.PUT("/zentao", settingH.SaveZentaoSettings)
				settings.POST("/zentao/test", settingH.TestZentaoConnection)
				// GitLab配置
				settings.GET("/gitlab", settingH.GetGitLabSettings)
				settings.PUT("/gitlab", settingH.SaveGitLabSettings)
				settings.POST("/gitlab/test", settingH.TestGitLabConnection)
				// 邮件配置
				settings.GET("/mail", settingH.GetMailSettings)
				settings.PUT("/mail", settingH.SaveMailSettings)
				settings.POST("/mail/test", settingH.TestMailConnection)
				settings.POST("/mail/send-test", settingH.SendTestMail)
				// CLI Runtime 配置
				settings.GET("/cli-runtime", settingH.GetCLIRuntimeSettings)
				settings.PUT("/cli-runtime", settingH.SaveCLIRuntimeSettings)
			}

			// 知识库配置
			protected.GET("/knowledge-base/config", middleware.RequirePermission("knowledge:list"), knowledgeH.GetConfig)
			protected.PUT("/knowledge-base/config", middleware.RequirePermission("knowledge:manage"), knowledgeH.SaveConfig)

			// RAG 知识库
			knowledgeBases := protected.Group("/knowledge-bases")
			{
				knowledgeBases.POST("", middleware.RequirePermission("knowledge:manage"), knowledgeH.CreateKB)
				knowledgeBases.GET("", middleware.RequirePermission("knowledge:list"), knowledgeH.ListKB)
				knowledgeBases.GET("/:id", middleware.RequirePermission("knowledge:list"), knowledgeH.GetKB)
				knowledgeBases.PUT("/:id", middleware.RequirePermission("knowledge:manage"), knowledgeH.UpdateKB)
				knowledgeBases.DELETE("/:id", middleware.RequirePermission("knowledge:manage"), knowledgeH.DeleteKB)
				knowledgeBases.GET("/:id/stats", middleware.RequirePermission("knowledge:list"), knowledgeH.Stats)
				knowledgeBases.POST("/:id/documents", middleware.RequirePermission("knowledge:manage"), knowledgeH.AddDocument)
				knowledgeBases.POST("/:id/documents/batch", middleware.RequirePermission("knowledge:manage"), knowledgeH.BatchDocuments)
				knowledgeBases.GET("/:id/documents", middleware.RequirePermission("knowledge:list"), knowledgeH.ListDocuments)
				knowledgeBases.POST("/:id/documents/:docId/rebuild", middleware.RequirePermission("knowledge:manage"), knowledgeH.RebuildDocument)
				knowledgeBases.DELETE("/:id/documents/:docId", middleware.RequirePermission("knowledge:manage"), knowledgeH.DeleteDocument)
				knowledgeBases.POST("/:id/chunks/rebuild", middleware.RequirePermission("knowledge:manage"), knowledgeH.RebuildKB)
				knowledgeBases.POST("/:id/query", middleware.RequirePermission("knowledge:list"), knowledgeH.Query)
				knowledgeBases.GET("/:id/graph", middleware.RequirePermission("knowledge:list"), knowledgeH.Graph)
			}

			// 禅道代理接口(获取项目集/分支下拉选项)
			zentao := protected.Group("/zentao")
			{
				zentao.GET("/products", zentaoProxyH.GetProducts)
				zentao.GET("/products/:id/branches", zentaoProxyH.GetBranches)
			}

			// 禅道用例管理
			zentaoTestCases := protected.Group("/test-cases/zentao")
			{
				zentaoTestCases.GET("", middleware.RequirePermission("test:list"), zentaoTestCaseH.List)
				zentaoTestCases.GET("/:id", middleware.RequirePermission("test:list"), zentaoTestCaseH.GetByID)
				zentaoTestCases.POST("/sync", middleware.RequirePermission("test:create"), zentaoTestCaseH.Sync)
				zentaoTestCases.POST("/generate", middleware.RequirePermission("test:execute"), zentaoTestCaseH.GenerateScript)
			}
		}
	}

	// CI 回调接口(公开，由CI系统调用，通过Token验证)
	api.POST("/ci/callback", ciCallbackH.Callback)

	// 流水线集成接口(公开，通过Token验证)
	integration := api.Group("/integration")
	integration.Use(middleware.TokenAuth())
	{
		integration.POST("/devflow-submit", integrationH.DevFlowSubmit)
		integration.POST("/cicd-deploy", integrationH.CICDDeploy)
		integration.GET("/project-metrics", integrationH.GetProjectMetrics)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
