# AutoTestFlow 开发环境启动脚本 (PowerShell)
# 使用方式: .\scripts\start-dev.ps1

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  AutoTestFlow 开发环境启动" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 1. 启动 MySQL
Write-Host "`n[1/4] 启动 MySQL..." -ForegroundColor Yellow
Set-Location $ProjectRoot
docker compose up -d mysql
Write-Host "等待 MySQL 就绪..." -ForegroundColor Gray
Start-Sleep -Seconds 8

# 2. 初始化数据库
Write-Host "`n[2/4] 初始化数据库..." -ForegroundColor Yellow
$sqlFiles = @("backend/migrations/001_init.sql", "backend/migrations/002_seed_skill.sql")
foreach ($sql in $sqlFiles) {
    $fullPath = Join-Path $ProjectRoot $sql
    if (Test-Path $fullPath) {
        Write-Host "  执行: $sql" -ForegroundColor Gray
        Get-Content $fullPath | docker exec -i atf-mysql mysql -uroot -proot 2>$null
    }
}
Write-Host "数据库初始化完成" -ForegroundColor Green

# 3. 启动后端
Write-Host "`n[3/4] 启动后端服务..." -ForegroundColor Yellow
$backendDir = Join-Path $ProjectRoot "backend"
if (Get-Command go -ErrorAction SilentlyContinue) {
    Start-Process -NoNewWindow -FilePath "go" -ArgumentList "run", "cmd/server/main.go", "-config", "configs/config.yaml" -WorkingDirectory $backendDir
    Write-Host "后端服务已启动 -> http://localhost:8080" -ForegroundColor Green
} else {
    Write-Host "  [跳过] 未找到 Go，请先安装 Go 1.22+" -ForegroundColor Red
    Write-Host "  安装后手动运行: cd backend && go run cmd/server/main.go" -ForegroundColor Gray
}

# 4. 启动前端
Write-Host "`n[3/3] 启动前端服务..." -ForegroundColor Yellow
$frontendDir = Join-Path $ProjectRoot "frontend"
if (Get-Command npm -ErrorAction SilentlyContinue) {
    if (-not (Test-Path (Join-Path $frontendDir "node_modules"))) {
        Write-Host "  安装前端依赖..." -ForegroundColor Gray
        Set-Location $frontendDir
        npm install
    }
    Start-Process -NoNewWindow -FilePath "npm" -ArgumentList "run", "dev" -WorkingDirectory $frontendDir
    Write-Host "前端服务已启动 -> http://localhost:3000" -ForegroundColor Green
} else {
    Write-Host "  [跳过] 未找到 npm，请先安装 Node.js 18+" -ForegroundColor Red
    Write-Host "  安装后手动运行: cd frontend && npm install && npm run dev" -ForegroundColor Gray
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "  启动完成!" -ForegroundColor Green
Write-Host "  前端: http://localhost:3000" -ForegroundColor White
Write-Host "  后端: http://localhost:8080" -ForegroundColor White
Write-Host "  登录: admin / admin123" -ForegroundColor White
Write-Host "========================================" -ForegroundColor Cyan
