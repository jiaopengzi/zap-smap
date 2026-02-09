# FilePath    : zap-smap\run.ps1
# Author      : jiaopengzi
# Blog        : https://jiaopengzi.com
# Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
# Description : 运行脚本，提供代码格式化、单元测试、go lint、构建和运行功能

# 定义可执行文件名称
$BINARY = "zap-smap"

# 显示菜单
Write-Host ""
Write-Host "请选择需要执行的命令："
Write-Host "  1 - 格式化 Go 代码并编译生成 Linux, Windows 和 macOS 二进制文件"
Write-Host "  2 - 编译 Go 代码并生成 Windows 二进制文件"
Write-Host "  3 - 编译 Go 代码并生成 Linux 二进制文件"
Write-Host "  4 - 编译 Go 代码并生成 macOS 二进制文件"
Write-Host "  5 - 编译运行 Go 代码"
Write-Host "  6 - 清理编译生成的二进制文件和缓存文件"
Write-Host "  7 - go lint"
Write-Host "  8 - 运行编译生成的 Windows 二进制文件"
Write-Host "  9 - 单元测试"
Write-Host " 10 - gopls check"
Write-Host " 11 - 格式化代码"
Write-Host ""
Write-Host "--- testdata 调试 ---"
Write-Host " 12 - testdata: 默认注入预览 (dry-run)"
Write-Host " 13 - testdata: 写入临时目录并对比 expected"
Write-Host " 14 - testdata: 验证已注入文件 (-verify)"
Write-Host " 15 - testdata: 删除字段预览 (-del)"
Write-Host " 16 - testdata: 排序字段预览 (-sort)"
Write-Host " 17 - testdata: 位置插入预览 (-position 0)"
Write-Host " 18 - testdata: 函数名注入预览 (-with-func)"
Write-Host " 19 - testdata: 重新生成所有 expected 文件"
Write-Host " 20 - testdata: 删除字段(fields slice) 预览 (-del)"
Write-Host ""

# 接收用户输入的操作编号
$choice = Read-Host "请输入编号选择对应的操作"
Write-Host ""

# 获取 Git 相关的版本信息, 用于 ldflags 注入版本信息
function getGitVersionInfo {
    $Version = "dev" # 版本号
    $Commit = "" # 提交哈希
    $BuildTime = "" # 构建时间

    # 格式化构建时间, 包含时区偏移(例如 2025-10-23 15:04:05 +08:00)
    $BuildTime = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss zzz")

    # 获取最新的 Git Commit Hash(完整)
    $Commit = (git rev-parse HEAD 2>$null).Trim()
    if (-not $Commit) {
        Write-Host "警告：无法获取 Git Commit, 可能不在 Git 仓库中。" -ForegroundColor Yellow
        $Commit = "unknown"
    }

    # 参考: https://semver.org/lang/zh-CN/
    # 获取最近的符合 1.2.3 0.1.2-beta+251113, 同时兼容带小写v前缀等格式的 Git Tag, 如果没有或不符合格式, 则为 "dev"
    $VersionTag = (git describe --tags --abbrev=0 2>$null | Select-String '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$' | Select-Object -First 1)

    if ($VersionTag) {
        $Version = $VersionTag.Line.Trim()
        Write-Host "检测到 Git Tag 版本: $Version" -ForegroundColor Green
    }
    else {
        Write-Host "未检测到符合 semver 格式的 Git Tag, 将不注入 Version" -ForegroundColor Yellow
        # Version 保持为空, 后续不注入该参数
    }

    # 返回一个 hashtable, 供外部拼接 ldflags 使用
    return @{
        Version   = $Version
        Commit    = $Commit
        BuildTime = $BuildTime
    }
}

# 根据 Git 信息生成 -ldflags 字符串
function getLdflags {
    $gitInfo = getGitVersionInfo

    $ldflags = "-s -w"  # 默认的优化参数

    # 如果 Version 非空, 则注入 Version
    if ($gitInfo.Version -and $gitInfo.Version -ne "") {
        $ldflags += " -X 'main.Version=$($gitInfo.Version)'"
    }

    # 注入 Commit
    $ldflags += " -X 'main.Commit=$($gitInfo.Commit)'"

    # 注入 BuildTime
    $ldflags += " -X 'main.BuildTime=$($gitInfo.BuildTime)'"

    Write-Host "编译参数 ldflags: $ldflags" -ForegroundColor Green

    return $ldflags
}

# 全部操作：格式化代码, 检查静态错误, 为所有平台生成二进制文件
function all {
    buildEnvInit
    goLint
    buildLinux
    buildWindows
    buildMacos
    restoreWindows
    Write-Host "✅ 全部操作执行完毕"
}

# 初始化 Go 环境变量 设置国内代理和禁用 CGO
function buildEnvInit {
    go env -w GO111MODULE=on
    go env -w CGO_ENABLED=0
    go env -w GOARCH=amd64
    go env -w GOPROXY="https://proxy.golang.com.cn,https://goproxy.cn,https://proxy.golang.org,direct"
    go mod tidy
}

# 为 Windows 系统编译 Go 代码并生成可执行文件到 bin/windows 目录下
function buildWindows {
    go env -w GOOS=windows
    $ldflags = getLdflags
    go build -trimpath -ldflags "$ldflags" -o "./bin/windows/$BINARY-windows.exe"
    Write-Host "✅ Windows 二进制文件生成完毕"
}

# 为 Windows 系统编译 Go 代码并生成可执行文件, 并将环境变量恢复到默认设置
function buildWindowsRestoreWindowsEnv {
    buildEnvInit
    buildWindows
    restoreWindows
}

# 为 Linux 系统编译 Go 代码并生成可执行文件到 bin/linux 目录下
function buildLinux {
    go env -w GOOS=linux
    $ldflags = getLdflags
    go build -trimpath -ldflags "$ldflags" -o "./bin/linux/$BINARY-linux"
    Write-Host "✅ Linux 二进制文件生成完毕"
}

# 为 linux 系统编译 Go 代码并生成可执行文件, 并将环境变量恢复到默认设置
function buildLinuxRestoreWindowsEnv {
    buildEnvInit
    buildLinux
    restoreWindows
}

# 为 macOS 系统编译 Go 代码并生成可执行文件到 bin/macos 目录下
function buildMacos {
    go env -w GOOS=darwin
    $ldflags = getLdflags
    go build -trimpath -ldflags "$ldflags" -o "./bin/macos/$BINARY-macos"
    Write-Host "✅ macOS 二进制文件生成完毕"
}

# 为 macos 系统编译 Go 代码并生成可执行文件, 并将环境变量恢复到默认设置
function buildMacosRestoreWindowsEnv {
    buildEnvInit
    buildMacos
    restoreWindows
}

# 运行编译生成的 Windows 二进制文件
function runOnly {
    & ".\bin\windows\$BINARY-windows.exe" --help
}

# 编译运行 Go 代码
function buildRun {
    $ldflags = getLdflags
    go build -trimpath -ldflags "$ldflags" -o "./bin/windows/$BINARY-windows.exe"
    runOnly
}

# 使用 golangci-lint run 命令检查代码格式和静态错误
function goLint {
    go vet ./...
    golangci-lint run
    Write-Host "✅ 代码格式和静态检查完毕"
}

# 清理编译生成的二进制文件和缓存文件
function clean {
    go clean
    if (Test-Path .\bin) {
        Remove-Item -Recurse -Force .\bin
    }
    Write-Host "✅ 编译生成的二进制文件和缓存文件已清理"
}

# 将环境变量恢复到默认设置(Windows 系统)
function restoreWindows {
    go env -w CGO_ENABLED=1
    go env -w GOOS=windows
    Write-Host "✅ 环境变量已恢复到 windows 默认设置"
}

# 单元测试
function test {
    go test -v ./...
}

# gopls check 检查代码格式和静态错误
function goplsCheck {
    # 运行前添加策略 Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
    # 这个脚本使用 gopls check 检查当前目录及其子目录中的所有 Go 文件。
    # 主要是在 gopls 升级后或者go版本升级后检查代码是否有问题.

    # 拿到当前目录下所有的 .go 文件数量
    $goFilesCount = Get-ChildItem -Path . -Filter *.go -File -Recurse | Measure-Object | Select-Object -ExpandProperty Count

    # 每分钟大约处理文件为 26 个, 计算出大概所需时间(秒)
    $estimatedTime = [Math]::Round($goFilesCount / 26 * 60)

    # 获取当前目录及其子目录中的所有 .go 文件
    $goFiles = Get-ChildItem -Recurse -Filter *.go

    # 记录开始时间
    $startTime = Get-Date

    # 设置定时器间隔
    $interval = 60

    # 初始化已检查文件数量
    $checkedFileCount = 0

    # 初始化上次输出时间
    $lastOutputTime = $startTime

    # 遍历每个 .go 文件并运行 gopls check 命令
    Write-Host "正在检查, 耗时预估 $estimatedTime 秒, 请耐心等待..." -ForegroundColor Green
    foreach ($file in $goFiles) {
        # Write-Host "正在检查 $($file.FullName)..."
        gopls check $file.FullName
        if ($LASTEXITCODE -ne 0) {
            Write-Host "检查 $($file.FullName) 时出错" -ForegroundColor Red
        } 
        $checkedFileCount++

        # 获取当前时间
        $currentTime = Get-Date
        $elapsedTime = $currentTime - $startTime

        # 检查是否已经超过了设定的时间间隔
        if (($currentTime - $lastOutputTime).TotalSeconds -ge $interval) {
            $roundedElapsedTime = [Math]::Round($elapsedTime.TotalSeconds)
            Write-Host "当前已耗时 $roundedElapsedTime 秒, 已检查文件数量: $checkedFileCount" -ForegroundColor Yellow
            # 更新上次输出时间
            $lastOutputTime = $currentTime
        }
    }

    # 记录结束时间
    $endTime = Get-Date

    # 计算耗时时间
    $elapsedTime = $endTime - $startTime

    # 显示总耗时时间和总文件数量
    $roundedElapsedTime = [Math]::Round($elapsedTime.TotalSeconds)
    Write-Host "检查结束, 总耗时 $roundedElapsedTime 秒, 总文件数量: $($goFiles.Count), 已检查文件数量: $checkedFileCount" -ForegroundColor Green
}

# 格式化代码
function formatCode {
    go fmt ./...
}

# ======================== testdata 调试函数 ========================

# 获取可执行文件路径
function getExePath {
    return ".\bin\windows\$BINARY-windows.exe"
}

# testdata: 默认注入预览 (dry-run, 不写回文件)
function testdataPreview {
    $exe = getExePath
    Write-Host "预览所有 inputs 文件的默认注入结果 (-field fl)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs -field fl
    Write-Host "✅ 预览完毕"
}

# testdata: 写入临时目录并对比 expected
function testdataWriteAndDiff {
    $exe = getExePath
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "zap-smap-testdata-$(Get-Date -Format 'yyyyMMddHHmmss')"

    # 复制 inputs 到临时目录
    Copy-Item -Path "testdata\inputs" -Destination $tmpDir -Recurse
    Write-Host "已复制 inputs 到临时目录: $tmpDir" -ForegroundColor Cyan

    # 执行写入
    & $exe -path $tmpDir -field fl -write

    # 对比每个文件
    Write-Host ""
    Write-Host "--- 对比结果 ---" -ForegroundColor Yellow
    $hasError = $false
    $expectedDir = "testdata\expected"

    Get-ChildItem -Path $tmpDir -Filter *.go | ForEach-Object {
        $inputFile = $_.FullName
        $expectedFile = Join-Path $expectedDir $_.Name

        if (Test-Path $expectedFile) {
            $inputContent = (Get-Content $inputFile -Raw).Trim()
            $expectedContent = (Get-Content $expectedFile -Raw).Trim()

            if ($inputContent -eq $expectedContent) {
                Write-Host "  ✅ $($_.Name) - 一致" -ForegroundColor Green
            } else {
                Write-Host "  ❌ $($_.Name) - 不一致" -ForegroundColor Red
                $hasError = $true
            }
        } else {
            Write-Host "  ⚠️ $($_.Name) - expected 文件不存在" -ForegroundColor Yellow
        }
    }

    if (-not $hasError) {
        Write-Host ""
        Write-Host "✅ 所有文件与 expected 一致" -ForegroundColor Green
    } else {
        Write-Host ""
        Write-Host "❌ 存在不一致的文件, 临时目录: $tmpDir" -ForegroundColor Red
    }
}

# testdata: 验证已注入文件 (-verify)
function testdataVerify {
    $exe = getExePath
    Write-Host "验证 expected 目录中已注入文件的正确性..." -ForegroundColor Cyan
    & $exe -path testdata/expected -field fl -verify -exclude "with_func,sort,del,position"

    Write-Host ""
    Write-Host "验证 mismatch 场景 (应报告 mismatch)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs/mismatch.go -field fl -verify
    Write-Host "✅ 验证完毕"
}

# testdata: 删除字段预览 (-del)
function testdataDel {
    $exe = getExePath
    Write-Host "预览删除字段 file:line (-del)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs/del_target.go -del "file:line"
    Write-Host "✅ 预览完毕"
}

# testdata: 删除字段(fields slice) 预览 (-del, ellipsis 场景)
function testdataDelFieldsSlice {
    $exe = getExePath
    Write-Host "预览删除字段 fl (fields... ellipsis 场景)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs/del_fields_slice.go -del "fl"
    Write-Host "✅ 预览完毕"
}

# testdata: 排序字段预览 (-sort)
function testdataSort {
    $exe = getExePath
    Write-Host "预览排序字段 (-sort)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs/sort_fields.go -field fl -sort
    Write-Host "✅ 预览完毕"
}

# testdata: 位置插入预览 (-position 0)
function testdataPosition {
    $exe = getExePath
    Write-Host "预览位置插入 (-position 0, 插入到参数列表最前)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs/position_insert.go -field fl -position 0
    Write-Host "✅ 预览完毕"
}

# testdata: 函数名注入预览 (-with-func)
function testdataWithFunc {
    $exe = getExePath
    Write-Host "预览函数名注入 (-with-func)..." -ForegroundColor Cyan
    & $exe -path testdata/inputs/with_func_name.go -field fl -with-func
    Write-Host "✅ 预览完毕"
}

# testdata: 重新生成所有 expected 文件
function testdataRegenExpected {
    $exe = getExePath

    Write-Host "重新生成所有 expected 文件..." -ForegroundColor Cyan

    # 1. 清理 expected 目录
    if (Test-Path "testdata\expected") {
        Remove-Item -Recurse -Force "testdata\expected"
    }
    New-Item -ItemType Directory -Path "testdata\expected" -Force | Out-Null
    New-Item -ItemType Directory -Path "testdata\expected\with_func" -Force | Out-Null
    New-Item -ItemType Directory -Path "testdata\expected\sort" -Force | Out-Null
    New-Item -ItemType Directory -Path "testdata\expected\del" -Force | Out-Null
    New-Item -ItemType Directory -Path "testdata\expected\position" -Force | Out-Null

    # 2. 默认场景: 复制 inputs 到 expected 并执行 -field fl -write
    Copy-Item -Path "testdata\inputs\*" -Destination "testdata\expected\" -Recurse
    Write-Host "  生成默认场景 (-field fl)..." -ForegroundColor Yellow
    & $exe -path testdata/expected -field fl -write

    # 3. with_func 场景
    Copy-Item "testdata\inputs\with_func_name.go" "testdata\expected\with_func\with_func_name.go" -Force
    Write-Host "  生成 with_func 场景 (-with-func)..." -ForegroundColor Yellow
    & $exe -path testdata/expected/with_func -field fl -with-func -write

    # 4. sort 场景
    Copy-Item "testdata\inputs\sort_fields.go" "testdata\expected\sort\sort_fields.go" -Force
    Write-Host "  生成 sort 场景 (-sort)..." -ForegroundColor Yellow
    & $exe -path testdata/expected/sort -field fl -sort -write

    # 5. del 场景
    Copy-Item "testdata\inputs\del_target.go" "testdata\expected\del\del_target.go" -Force
    Copy-Item "testdata\inputs\del_fields_slice.go" "testdata\expected\del\del_fields_slice.go" -Force
    Write-Host "  生成 del 场景 (del_target: -del file:line)..." -ForegroundColor Yellow
    & $exe -path testdata/expected/del/del_target.go -del "file:line" -write
    Write-Host "  生成 del 场景 (del_fields_slice: -del fl)..." -ForegroundColor Yellow
    & $exe -path testdata/expected/del/del_fields_slice.go -del "fl" -write

    # 6. position 场景
    Copy-Item "testdata\inputs\position_insert.go" "testdata\expected\position\position_insert.go" -Force
    Write-Host "  生成 position 场景 (-position 0)..." -ForegroundColor Yellow
    & $exe -path testdata/expected/position -field fl -position 0 -write

    Write-Host "✅ 所有 expected 文件已重新生成"
}

# switch 要放到最后 
# 执行用户选择的操作
switch ($choice) {
    1 { all }
    2 { buildWindowsRestoreWindowsEnv }
    3 { buildLinuxRestoreWindowsEnv }
    4 { buildMacosRestoreWindowsEnv }
    5 { buildRun }
    6 { clean }
    7 { goLint }
    8 { runOnly }
    9 { test }
    10 { goplsCheck }
    11 { formatCode }
    12 { testdataPreview }
    13 { testdataWriteAndDiff }
    14 { testdataVerify }
    15 { testdataDel }
    16 { testdataSort }
    17 { testdataPosition }
    18 { testdataWithFunc }
    19 { testdataRegenExpected }
    20 { testdataDelFieldsSlice }
    default { Write-Host "❌ 无效的选择" }
}