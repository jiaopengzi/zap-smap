# zap-smap

[![Go Version](https://img.shields.io/badge/Go-1.25.6+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

一个用于 [zap](https://github.com/uber-go/zap) 日志库**针对需要混淆**源码注入工具。自动在 `zap.L().Info/Error/Debug/Warn/...` 调用处注入 `zap.String("fl", "file:line")` 字段，让每条日志都能精确追溯到源码位置。

## 功能特性

- **自动注入**：扫描 Go 源码，在所有 zap 日志调用处注入文件名和行号字段
- **幂等操作**：重复运行不会产生重复注入，值会自动更新
- **Variadic (fields...) 支持**：正确处理 `fields...` 展开调用，使用 `append([]zap.Field{...}, fields...)...` 包裹
- **纯删除**：`-del` 参数纯删除指定字段，不会注入新字段
- **字段排序**：`-sort` 按字段键的字母顺序排列 zap 字段
- **位置控制**：`-position` 控制字段插入位置（基于 field 参数列表，跳过 msg）
- **函数名注入**：`-with-func` 在注入内容中包含函数名
- **校验模式**：`-verify` 仅校验注入是否正确，输出汇总报告
- **Dry-run 预览**：默认不修改文件，展示预览差异
- **排除路径**：`-exclude` 跳过指定目录或文件
- **单文件/目录模式**：支持处理单个文件或递归扫描目录

## 安装

### 方式一：下载预编译二进制（推荐）

从 [Releases](https://github.com/jiaopengzi/zap-smap/releases) 页面下载对应平台的二进制文件。

> 预编译版本包含完整的版本信息（Version、Commit、BuildTime）。

### 方式二：Go Install

```bash
go install github.com/jiaopengzi/zap-smap@latest
```

> 此方式安装的版本号来自 Go 模块版本（如 v0.2.0），Commit 和 BuildTime 来自 VCS 信息。

### 方式三：源码编译（完整版本信息）

```bash
git clone https://github.com/jiaopengzi/zap-smap.git
cd zap-smap

# Windows (PowerShell)
.\run.ps1
# 选择 2 - 构建 Windows 二进制

# Linux/macOS
make build-linux   # 或 build-macos
```

## 快速开始

### 1. 预览注入效果（dry-run）

```bash
zap-smap -path ./your/project
```

默认不修改文件，仅展示预览。

### 2. 写入注入

```bash
zap-smap -path ./your/project -write
```

### 3. 校验注入

```bash
zap-smap -path ./your/project -verify
```

## 注入效果示例

**注入前：**

```go
func CreateOrder() {
    zap.L().Info("order created", zap.String("order_id", "12345"))
    zap.L().Error("payment failed", zap.String("reason", "timeout"))
}
```

**注入后：**

```go
func CreateOrder() {
    zap.L().Info("order created", zap.String("fl", "order.go:2"), zap.String("order_id", "12345"))
    zap.L().Error("payment failed", zap.String("fl", "order.go:3"), zap.String("reason", "timeout"))
}
```

### Variadic (fields...) 场景

**注入前：**

```go
fields := []zap.Field{zap.String("module", "order")}
zap.L().Warn("请求信息", fields...)
```

**注入后：**

```go
fields := []zap.Field{zap.String("module", "order")}
zap.L().Warn("请求信息", append([]zap.Field{zap.String("fl", "handler.go:3")}, fields...)...)
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-path` | `.` | 要扫描的文件或目录 |
| `-field` | `fl` | 要注入的字段名 |
| `-del` | `""` | 要删除的字段名（纯删除，不注入新字段） |
| `-write` | `false` | 将修改写回文件 |
| `-with-func` | `false` | 在注入值中包含函数名 |
| `-verify` | `false` | 校验模式，输出汇总报告 |
| `-exclude` | `""` | 以逗号分隔的排除目录或文件路径 |
| `-position` | `-1` | 插入位置索引（基于 field 列表，0 = 第一个 field 之前） |
| `-sort` | `false` | 按字段键的字母顺序排列 zap 字段 |

> **注意**：`-del` 和 `-field` 不能同时使用。如需替换字段名，请先 `-del` 再 `-field` 分两步执行。

## 使用示例

### 自定义字段名

```bash
zap-smap -path ./src -field "file:line" -write
```

### 删除旧字段

```bash
zap-smap -path ./src -del "file:line" -write
```

### 替换字段名（两步操作）

```bash
# 步骤 1：删除旧字段
zap-smap -path ./src -del "file:line" -write

# 步骤 2：注入新字段
zap-smap -path ./src -field "fl" -write
```

### 注入函数名

```bash
zap-smap -path ./src -with-func -write
```

注入值格式：`file.go:7 | package.Function`

### 排序字段

```bash
zap-smap -path ./src -sort -write
```

### 排除目录

```bash
zap-smap -path ./src -exclude "vendor,testdata,mock" -write
```

### 控制插入位置

```bash
# 插入到所有 field 最前面（默认行为）
zap-smap -path ./src -position 0 -write

# 插入到第 2 个 field 之后
zap-smap -path ./src -position 2 -write
```

## 支持的 zap 方法

工具会处理以下 zap 日志方法：

- `zap.L().Debug()`
- `zap.L().Info()`
- `zap.L().Warn()`
- `zap.L().Error()`
- `zap.L().DPanic()`
- `zap.L().Panic()`
- `zap.L().Fatal()`

同时支持 `zap.L().With(...).Info(...)` 等链式调用。

## 自动排除

工具自动跳过以下路径：

- `vendor/`、`.git/`、`build/`、`node_modules/` 目录
- `_gen.go` 后缀的生成文件
- `/internal/` 路径下的文件
- 非 `.go` 文件

## 开发

### 运行测试

```bash
go test ./...
```

### Windows 脚本

项目提供了 `run.ps1` 脚本，包含编译、测试、testdata 调试等功能：

```powershell
.\run.ps1
# 按提示选择操作编号
```

### 项目结构

```
zap-smap/
├── main.go              # 入口，解析参数与模式分发
├── flag.go              # 命令行参数定义与冲突检查
├── process.go           # AST 注入/删除核心逻辑
├── walk.go              # 目录遍历与文件处理
├── verify.go            # -verify 校验逻辑
├── sort.go              # -sort 字段排序
├── preview.go           # dry-run 预览输出
├── utils.go             # 工具函数
├── types.go             # 类型与常量定义
├── Makefile             # Linux/macOS 构建
├── run.ps1              # Windows 构建与调试脚本
└── testdata/
    ├── inputs/          # 测试输入文件
    └── expected/        # 期望输出文件
        ├── del/         # -del 场景
        ├── sort/        # -sort 场景
        ├── position/    # -position 场景
        └── with_func/   # -with-func 场景
```

## License

[MIT License](LICENSE) © 2026 [焦棚子](https://jiaopengzi.com)
