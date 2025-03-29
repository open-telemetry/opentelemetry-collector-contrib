// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
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
		if e.Results != nil {
			for _, r := range e.Results.List {
				results = append(results, ExprToString(r.Type))
			}
		}
		var params []string
		if e.Params != nil {
			for _, r := range e.Params.List {
				params = append(params, ExprToString(r.Type))
			}
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
	case *ast.IndexListExpr:
		var exprs []string
		for _, e := range e.Indices {
			exprs = append(exprs, ExprToString(e))
		}
		return strings.Join(exprs, ",")
	default:
		panic(fmt.Sprintf("Unsupported expr type: %#v", expr))
	}
}

func Read(folder string, ignoredFunctions []string) (*Api, error) {
	result := &Api{}
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, folder, nil, 0)
	if err != nil {
		return nil, err
	}

	for _, pack := range packs {
		for _, f := range pack.Files {
			readFile(ignoredFunctions, f, result)
		}
	}

	return result, nil
}

func readFile(ignoredFunctions []string, f *ast.File, result *Api) {
	for _, d := range f.Decls {
		if str, isStr := d.(*ast.GenDecl); isStr {
			for _, s := range str.Specs {
				if values, ok := s.(*ast.ValueSpec); ok {
					for _, v := range values.Names {
						if v.IsExported() {
							result.Values = append(result.Values, v.Name)
						}
					}
				}
				if t, ok := s.(*ast.TypeSpec); ok {
					var fieldNames []string
					if t.TypeParams != nil {
						fieldNames = make([]string, len(t.TypeParams.List))
						for i, f := range t.TypeParams.List {
							fieldNames[i] = f.Names[0].Name
						}
					}
					result.Structs = append(result.Structs, &Apistruct{
						Name:   t.Name.String(),
						Fields: fieldNames,
					})
				}
			}
		}
		if fn, isFn := d.(*ast.FuncDecl); isFn {
			if !fn.Name.IsExported() {
				continue
			}
			exported := false
			receiver := ""
			if fn.Recv.NumFields() == 0 && !isFunctionIgnored(ignoredFunctions, fn.Name.String()) {
				exported = true
			}
			if fn.Recv.NumFields() > 0 {
				for _, t := range fn.Recv.List {
					for _, n := range t.Names {
						exported = exported || n.IsExported()
						if n.IsExported() {
							receiver = n.Name
						}
					}
				}
			}
			if exported {
				var returnTypes []string
				if fn.Type.Results.NumFields() > 0 {
					for _, r := range fn.Type.Results.List {
						returnTypes = append(returnTypes, ExprToString(r.Type))
					}
				}
				var params []string
				if fn.Type.Params.NumFields() > 0 {
					for _, r := range fn.Type.Params.List {
						params = append(params, ExprToString(r.Type))
					}
				}
				f := &Function{
					Name:        fn.Name.Name,
					Receiver:    receiver,
					ParamTypes:  params,
					ReturnTypes: returnTypes,
				}
				result.Functions = append(result.Functions, f)
			}
		}
	}
}

func isFunctionIgnored(ignoredFunctions []string, fnName string) bool {
	for _, v := range ignoredFunctions {
		reg := regexp.MustCompile(v)
		if reg.MatchString(fnName) {
			return true
		}
	}
	return false
}
