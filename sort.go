//
// FilePath    : zap-smap\sort.go
// Author      : jiaopengzi
// Blog        : https://jiaopengzi.com
// Copyright   : Copyright (c) 2026 by jiaopengzi, All Rights Reserved.
// Description : 对 zap 字段进行排序
//

package main

import (
	"go/ast"
	"sort"
)

// sortZapFields 将 ce 的 zap 字段按 key 的字母顺序排序, 保留第一个参数(通常是 message)，
// 其它非 zap 字段保留在尾部(原序)
func sortZapFields(ce *ast.CallExpr) {
	if len(ce.Args) <= 1 {
		return
	}

	head := ce.Args[0]

	var zapExprs []ast.Expr

	var others []ast.Expr

	for _, a := range ce.Args[1:] {
		if call, ok := isZapFieldCall(a); ok {
			zapExprs = append(zapExprs, call)
		} else {
			others = append(others, a)
		}
	}

	if len(zapExprs) <= 1 {
		// 没有或只有一个 zap 字段, 无需排序
		ce.Args = append([]ast.Expr{head}, append(zapExprs, others...)...)
		return
	}

	// 带 key 的临时切片以便排序
	type kv struct {
		expr ast.Expr
		key  string
	}

	tmp := make([]kv, 0, len(zapExprs))

	for _, e := range zapExprs {
		if c, ok := e.(*ast.CallExpr); ok {
			k := getZapFieldKey(c)
			tmp = append(tmp, kv{expr: e, key: k})
		} else {
			tmp = append(tmp, kv{expr: e, key: ""})
		}
	}

	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].key < tmp[j].key
	})

	sorted := make([]ast.Expr, 0, len(tmp))
	for _, it := range tmp {
		sorted = append(sorted, it.expr)
	}

	ce.Args = append([]ast.Expr{head}, append(sorted, others...)...)
}
