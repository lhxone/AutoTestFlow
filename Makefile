.PHONY: help db-up db-down backend-run backend-build frontend-dev frontend-build

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ====== 数据库 ======
db-up: ## 启动MySQL(Docker)
	docker-compose up -d mysql

db-down: ## 停止MySQL
	docker-compose down

db-init: ## 初始化数据库(执行SQL)
	docker exec -i atf-mysql mysql -uroot -proot auto_test_flow < backend/migrations/001_init.sql

# ====== 后端 ======
backend-deps: ## 安装后端依赖
	cd backend && go mod tidy

backend-run: ## 运行后端(开发模式)
	cd backend && go run cmd/server/main.go -config configs/config.yaml

backend-build: ## 编译后端
	cd backend && go build -o bin/server cmd/server/main.go

# ====== 前端 ======
frontend-deps: ## 安装前端依赖
	cd frontend && npm install

frontend-dev: ## 运行前端(开发模式)
	cd frontend && npm run dev

frontend-build: ## 编译前端
	cd frontend && npm run build

# ====== 一键启动 ======
dev: db-up ## 一键启动开发环境(DB + Backend + Frontend)
	@echo "等待MySQL就绪..."
	@sleep 5
	@echo "MySQL已启动，请分别运行: make backend-run 和 make frontend-dev"
