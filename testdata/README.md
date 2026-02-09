# testdata 测试数据说明

本目录包含用于调试和验证 `zap-smap` 工具的测试文件。

## 目录结构

```
testdata/
├── inputs/                     # 输入文件 (未经处理的原始 Go 源码)
│   ├── basic.go                # 基本场景: 单函数单调用
│   ├── multi_func.go           # 多函数场景: 不同函数中多个调用
│   ├── multi_level.go          # 多日志等级: Debug/Info/Warn/Error/DPanic
│   ├── with_chain.go           # 链式调用: zap.L().With(...).Info(...)
│   ├── with_existing_fields.go # 带现有字段: 已有其他 zap 字段
│   ├── anonymous_func.go       # 匿名函数: zap 调用在匿名函数内
│   ├── no_zap.go               # 无 zap 导入: 不应被修改的文件
│   ├── already_injected.go     # 已注入: 已有正确注入字段
│   ├── mismatch.go             # 值不匹配: 注入值错误 (用于 -verify)
│   ├── del_target.go           # 删除字段: 含旧字段 (用于 -del)
│   ├── sort_fields.go          # 排序字段: 字段无序 (用于 -sort)
│   ├── position_insert.go      # 位置插入: (用于 -position)
│   ├── with_func_name.go       # 函数名注入: 含方法接收者 (用于 -with-func)
│   ├── mixed_scenario.go       # 综合场景: 混合多种调用模式
│   └── fields_slice.go         # 字段切片: fields... 展开传入 (边界场景)
│
└── expected/                   # 期望输出文件
    ├── *.go                    # 默认场景: -field fl (无排序/无函数名)
    ├── with_func/              # -with-func 场景
    │   └── with_func_name.go
    ├── sort/                   # -sort 场景
    │   └── sort_fields.go
    ├── del/                    # -del "file:line" 场景
    │   └── del_target.go
    └── position/               # -position 0 场景
        └── position_insert.go
```

## 使用方式

### 1. 默认注入 (dry-run 预览)

```bash
# 预览所有输入文件的注入结果
zap-smap -path testdata/inputs -field fl
```

### 2. 写入模式

```bash
# 先复制输入文件到临时目录, 再执行写入
cp -r testdata/inputs /tmp/test_inputs
zap-smap -path /tmp/test_inputs -field fl -write

# 对比结果与 expected
diff /tmp/test_inputs testdata/expected
```

### 3. 验证模式

```bash
# 对已注入的文件进行校验
zap-smap -path testdata/expected -field fl -verify

# 对值不匹配的文件进行校验 (应报告 mismatch)
zap-smap -path testdata/inputs/mismatch.go -field fl -verify
```

### 4. 删除字段

```bash
zap-smap -path testdata/inputs/del_target.go -del "file:line"
# 对比: testdata/expected/del/del_target.go
```

### 5. 排序字段

```bash
zap-smap -path testdata/inputs/sort_fields.go -field fl -sort
# 对比: testdata/expected/sort/sort_fields.go
```

### 6. 位置插入

```bash
zap-smap -path testdata/inputs/position_insert.go -field fl -position 0
# 对比: testdata/expected/position/position_insert.go
```

### 7. 函数名注入

```bash
zap-smap -path testdata/inputs/with_func_name.go -field fl -with-func
# 对比: testdata/expected/with_func/with_func_name.go
```

## 输入文件场景说明

| 文件 | 场景 | 测试要点 |
|------|------|----------|
| `basic.go` | 最简单的单函数单调用 | 验证基本注入是否正确 |
| `multi_func.go` | 多个函数各有 zap 调用 | 验证每个调用都被正确注入 |
| `multi_level.go` | 所有日志等级 | 验证 Debug/Info/Warn/Error/DPanic 都被处理 |
| `with_chain.go` | `zap.L().With(...).Info(...)` | 验证链式调用的识别与注入 |
| `with_existing_fields.go` | 已有 `zap.String/Uint64` 字段 | 验证注入位置不影响现有字段 |
| `anonymous_func.go` | 匿名函数和 goroutine | 验证嵌套函数内的调用被处理 |
| `no_zap.go` | 无 zap 导入 | 验证不会修改无关文件 |
| `already_injected.go` | 字段已存在且值正确 | 验证幂等性(更新已有字段) |
| `mismatch.go` | 字段已存在但值错误 | 验证 `-verify` 报告 mismatch |
| `del_target.go` | 含旧键 `file:line` | 验证 `-del` 正确删除旧字段 |
| `sort_fields.go` | 多个无序字段 | 验证 `-sort` 按字母排序 |
| `position_insert.go` | 指定位置插入 | 验证 `-position` 控制插入位置 |
| `with_func_name.go` | 普通函数+方法接收者 | 验证 `-with-func` 包含函数名 |
| `mixed_scenario.go` | 综合: 多函数+匿名+方法+多字段 | 端到端集成测试 |
| `fields_slice.go` | `[]zap.Field` 切片 + `fields...` 展开 | ⚠️ 边界场景: 展开调用不应被注入 |

## expected 文件生成方式

expected 文件由以下命令生成:

```bash
# 默认场景 (expected/*.go)
cp testdata/inputs/* testdata/expected/
zap-smap -path testdata/expected -field fl -write

# with_func 场景
cp testdata/inputs/with_func_name.go testdata/expected/with_func/
zap-smap -path testdata/expected/with_func -field fl -with-func -write

# sort 场景
cp testdata/inputs/sort_fields.go testdata/expected/sort/
zap-smap -path testdata/expected/sort -field fl -sort -write

# del 场景
cp testdata/inputs/del_target.go testdata/expected/del/
zap-smap -path testdata/expected/del -del "file:line" -write

# position 场景 (position=0: 插入到参数列表最前)
cp testdata/inputs/position_insert.go testdata/expected/position/
zap-smap -path testdata/expected/position -field fl -position 0 -write
```
