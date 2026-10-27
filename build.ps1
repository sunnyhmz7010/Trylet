param([switch]$Run)
$ErrorActionPreference = 'Stop'
$practicePreviousDirectory = Get-Location
try {
    Set-Location -LiteralPath $PSScriptRoot
    & (Join-Path $PSScriptRoot 'build\make-icon.ps1')
    if (-not (Test-Path -LiteralPath 'frontend\dist')) { New-Item -ItemType Directory -Path 'frontend\dist' | Out-Null }
    if (-not (Test-Path -LiteralPath 'frontend\dist\index.html')) {
        [IO.File]::WriteAllText((Join-Path $PSScriptRoot 'frontend\dist\index.html'), '<!doctype html><html lang="zh-CN"><title>Trylet</title></html>', [Text.UTF8Encoding]::new($false))
    }
    npm --prefix frontend ci --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw '前端依赖安装失败' }
    go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0 generate bindings -ts -i -names
    if ($LASTEXITCODE -ne 0) { throw 'Wails 接口生成失败' }
    npm --prefix frontend run build
    if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }
    go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0 generate syso -manifest build/app.manifest -icon build/icon.ico -arch amd64
    if ($LASTEXITCODE -ne 0) { throw 'Windows 资源生成失败' }
    New-Item -ItemType Directory -Force -Path bin | Out-Null
    go build -tags production -trimpath -ldflags '-H windowsgui -s -w' -o bin/Trylet.exe .
    if ($LASTEXITCODE -ne 0) { throw '桌面应用构建失败' }
    Write-Host ('已生成：' + (Join-Path $PSScriptRoot 'bin\Trylet.exe'))
    if ($Run) { Start-Process -FilePath (Join-Path $PSScriptRoot 'bin\Trylet.exe') -WindowStyle Hidden }
} finally { Set-Location -LiteralPath $practicePreviousDirectory }
