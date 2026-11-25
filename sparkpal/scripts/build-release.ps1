# 发版构建（Windows / PowerShell）
#
# 为什么要在别的目录构建：
#   Flutter 的 AOT 编译器（gen_snapshot）在 Windows 上读不了含中文的路径，
#   项目放在 D:\项目\开源\ 下时 --release 必然失败（app.dill 读不出来）。
#   所以这里先镜像到一个纯 ASCII 目录构建，再把产物拷回来。
#
# 用法：
#   .\scripts\build-release.ps1                 # 混淆 + 分离符号表
#   .\scripts\build-release.ps1 -NoObfuscate    # 不混淆
#   .\scripts\build-release.ps1 -SkipBuild      # 只拷上次产物

param(
    [switch]$NoObfuscate,
    [string]$BuildRoot = "D:\dev\sparkpal_build",
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Write-Host "源仓库 $root" -ForegroundColor DarkGray
Write-Host "构建目录 $BuildRoot" -ForegroundColor DarkGray

if (-not $SkipBuild) {
    # ── 镜像到 ASCII 路径 ──
    Write-Host "`n[1/4] 镜像源码" -ForegroundColor Cyan
    New-Item -ItemType Directory -Force -Path $BuildRoot | Out-Null
    $rcArgs = @($root, $BuildRoot, '/MIR', '/NFL', '/NDL', '/NJH', '/NJS', '/NP', '/R:1', '/W:1')
    foreach ($d in @('build', '.dart_tool', '.gradle', '.idea', 'dist', 'node_modules', '.git')) {
        $rcArgs += '/XD'
        $rcArgs += $d
    }
    & robocopy @rcArgs | Out-Null
    if ($LASTEXITCODE -ge 8) {
        Write-Host "robocopy 输出：" -ForegroundColor Yellow
        & robocopy @rcArgs | Select-Object -Last 15
        throw "镜像失败（robocopy $LASTEXITCODE）"
    }

    # ── 构建 ──
    $env:Path = "D:\dev\flutter\bin;$env:Path"
    $env:PUB_HOSTED_URL = "https://pub.flutter-io.cn"
    $env:FLUTTER_STORAGE_BASE_URL = "https://storage.flutter-io.cn"

    Get-Process dart, java -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

    Push-Location "$BuildRoot\app"
    try {
        New-Item -ItemType Directory -Force -Path "build\symbols" | Out-Null
        if ($NoObfuscate) {
            Write-Host "`n[2/4] 构建（不混淆）" -ForegroundColor Cyan
            flutter build apk --release
        } else {
            Write-Host "`n[2/4] 构建（混淆 + 分离符号表）" -ForegroundColor Cyan
            flutter build apk --release --obfuscate --split-debug-info=build/symbols
        }
        if ($LASTEXITCODE -ne 0) { throw "构建失败" }
    } finally {
        Pop-Location
    }
} else {
    Write-Host "`n[1-2/4] 跳过构建" -ForegroundColor Yellow
}

# ── 拷回产物 ──
Write-Host "`n[3/4] 收集产物" -ForegroundColor Cyan
$apk = Join-Path $BuildRoot "app\build\app\outputs\flutter-apk\app-release.apk"
if (-not (Test-Path $apk)) { throw "没找到 APK：$apk" }

$dest = Join-Path $root $OutDir
New-Item -ItemType Directory -Force -Path $dest | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmm"

$size = [math]::Round((Get-Item $apk).Length / 1MB, 1)
$target = Join-Path $dest "sparkpal-$stamp.apk"
Copy-Item $apk $target -Force

$symZip = $null
if (-not $NoObfuscate) {
    $sym = Join-Path $BuildRoot "app\build\symbols"
    if (Test-Path $sym) {
        $symZip = Join-Path $dest "sparkpal-$stamp-symbols.zip"
        Compress-Archive -Path "$sym\*" -DestinationPath $symZip -Force
    }
}

Write-Host "`n[4/4] 完成" -ForegroundColor Green
Write-Host "  APK     $target  ($size MB)"
if ($symZip) {
    Write-Host "  符号表  $symZip"
    Write-Host "  ↑ 必须保留：线上崩溃要用它还原堆栈" -ForegroundColor Yellow
}
