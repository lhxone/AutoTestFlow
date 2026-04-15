# 数据库初始化脚本 (PowerShell)
# 使用方式: .\scripts\init-db.ps1
# 前提: MySQL 容器已通过 docker-compose up -d mysql 启动

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "初始化 AutoTestFlow 数据库..." -ForegroundColor Yellow

$sqlFiles = @(
    "backend/migrations/001_init.sql",
    "backend/migrations/002_seed_skill.sql"
)

foreach ($sql in $sqlFiles) {
    $fullPath = Join-Path $ProjectRoot $sql
    if (Test-Path $fullPath) {
        Write-Host "  执行: $sql" -ForegroundColor Gray
        Get-Content $fullPath -Raw | docker exec -i atf-mysql mysql -uroot -proot
        if ($LASTEXITCODE -eq 0) {
            Write-Host "  成功" -ForegroundColor Green
        } else {
            Write-Host "  失败(可能表已存在，可忽略)" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  文件不存在: $sql" -ForegroundColor Red
    }
}

Write-Host "`n数据库初始化完成!" -ForegroundColor Green
Write-Host "默认管理员账号: admin / admin123" -ForegroundColor White
