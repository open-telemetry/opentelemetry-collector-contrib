// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"go/ast"
	"strings"
)

func ExprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", ExprToString(e.Key), ExprToString(e.Value))
	case *ast.ArrayType:
		return fmt.Sprintf("[%s]%s", ExprToString(e.Len), ExprToString(e.Elt))
	case *ast.StructType:
		var fields []string
		for _, f := range e.Fields.List {
			fields = append(fields, ExprToString(f.Type))
		}
		return fmt.Sprintf("{%s}", strings.Join(fields, ","))
	case *ast.InterfaceType:
		var methods []string
		for _, f := range e.Methods.List {
			methods = append(methods, "func "+ExprToString(f.Type))
		}
		return fmt.Sprintf("{%s}", strings.Join(methods, ","))
	case *ast.ChanType:
		return fmt.Sprintf("chan(%s)", ExprToString(e.Value))
	case *ast.FuncType:
		var results []string
		for _, r := range e.Results.List {
			results = append(results, ExprToString(r.Type))
		}
		var params []string
		for _, r := range e.Params.List {
			params = append(params, ExprToString(r.Type))
		}
		return fmt.Sprintf("func(%s) %s", strings.Join(params, ","), strings.Join(results, ","))
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", ExprToString(e.X), e.Sel.Name)
	case *ast.Ident:
		return e.Name
	case nil:
		return ""
	case *ast.StarExpr:
		return fmt.Sprintf("*%s", ExprToString(e.X))
	case *ast.Ellipsis:
		return fmt.Sprintf("%s...", ExprToString(e.Elt))
	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", ExprToString(e.X), ExprToString(e.Index))
	case *ast.BasicLit:
		return e.Value
	default:
		panic(fmt.Sprintf("Unsupported expr type: %#v", expr))
	}
}
