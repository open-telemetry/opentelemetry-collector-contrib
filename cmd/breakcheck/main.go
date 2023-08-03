package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	flag.Parse()
	folder := flag.Arg(0)
	if err := run(folder); err != nil {
		log.Fatal(err)
	}
}

type function struct {
	Name        string   `json:"name"`
	Receiver    string   `json:"receiver"`
	ReturnTypes []string `json:"return_types,omitempty"`
	ParamTypes  []string `json:"param_types,omitempty"`
}

type api struct {
	Values    []string    `json:"values,omitempty"`
	Structs   []string    `json:"structs,omitempty"`
	Functions []*function `json:"functions,omitempty"`
}

func run(folder string) error {
	return filepath.Walk(folder, func(path string, info fs.FileInfo, err error) error {
		if info.Name() == "go.mod" {
			if err := walkFolder(filepath.Dir(path)); err != nil {
				return err
			}
		}
		return nil
	})
}

func walkFolder(folder string) error {
	result := &api{}
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, folder, nil, 0)
	if err != nil {
		return err
	}

	for _, pack := range packs {
		for _, f := range pack.Files {
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
							if t.Name.IsExported() {
								result.Structs = append(result.Structs, t.Name.String())
							}
						}

					}
				}
				if fn, isFn := d.(*ast.FuncDecl); isFn {
					if !fn.Name.IsExported() {
						continue
					}
					exported := false
					receiver := ""
					if fn.Recv.NumFields() == 0 && !strings.HasPrefix(fn.Name.String(), "Test") && !strings.HasPrefix(fn.Name.String(), "Benchmark") {
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
								returnTypes = append(returnTypes, exprToString(r.Type))
							}
						}
						var params []string
						if fn.Type.Params.NumFields() > 0 {
							for _, r := range fn.Type.Params.List {
								params = append(params, exprToString(r.Type))
							}
						}
						f := &function{
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
	}
	sort.Strings(result.Structs)
	sort.Strings(result.Values)
	sort.Slice(result.Functions, func(i int, j int) bool {
		return strings.Compare(result.Functions[i].Name, result.Functions[j].Name) < 0
	})
	toStore, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if len(toStore) == 2 {
		return nil
	}

	return os.WriteFile(filepath.Join(folder, ".api"), toStore, 0600)
}

func exprToString(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.MapType:
		mapExpr := expr.(*ast.MapType)
		return fmt.Sprintf("map[%s]%s", exprToString(mapExpr.Key), exprToString(mapExpr.Value))
	case *ast.ArrayType:
		arrayExpr := expr.(*ast.ArrayType)
		return fmt.Sprintf("[%s]%s", exprToString(arrayExpr.Len), exprToString(arrayExpr.Elt))
	case *ast.StructType:
		structExpr := expr.(*ast.StructType)
		var fields []string
		for _, f := range structExpr.Fields.List {
			fields = append(fields, exprToString(f.Type))
		}
		return fmt.Sprintf("{%s}", strings.Join(fields, ","))
	case *ast.InterfaceType:
		interfaceExpr := expr.(*ast.InterfaceType)
		var methods []string
		for _, f := range interfaceExpr.Methods.List {
			methods = append(methods, "func "+exprToString(f.Type))
		}
		return fmt.Sprintf("{%s}", strings.Join(methods, ","))
	case *ast.ChanType:
		chanExpr := expr.(*ast.ChanType)
		return fmt.Sprintf("chan(%s)", exprToString(chanExpr.Value))
	case *ast.FuncType:
		funcExpr := expr.(*ast.FuncType)
		var results []string
		for _, r := range funcExpr.Results.List {
			results = append(results, exprToString(r.Type))
		}
		var params []string
		for _, r := range funcExpr.Params.List {
			params = append(params, exprToString(r.Type))
		}
		return fmt.Sprintf("func(%s) %s", strings.Join(params, ","), strings.Join(results, ","))
	case *ast.SelectorExpr:
		selectorExpr := expr.(*ast.SelectorExpr)
		return fmt.Sprintf("%s.%s", exprToString(selectorExpr.X), selectorExpr.Sel.Name)
	case *ast.Ident:
		identExpr := expr.(*ast.Ident)
		return identExpr.Name
	case nil:
		return ""
	case *ast.StarExpr:
		starExpr := expr.(*ast.StarExpr)
		return fmt.Sprintf("*%s", exprToString(starExpr.X))
	case *ast.Ellipsis:
		ellipsis := expr.(*ast.Ellipsis)
		return fmt.Sprintf("%s...", exprToString(ellipsis.Elt))
	case *ast.IndexExpr:
		indexExpr := expr.(*ast.IndexExpr)
		return fmt.Sprintf("%s[%s]", exprToString(indexExpr.X), exprToString(indexExpr.Index))
	case *ast.BasicLit:
		basicLit := expr.(*ast.BasicLit)
		return basicLit.Value
	default:
		panic(fmt.Sprintf("Unsupported expr type: %#v", expr))
	}
}
