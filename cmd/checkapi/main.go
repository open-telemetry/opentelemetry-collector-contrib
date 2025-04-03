// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/checkapi/internal"
)

func main() {
	folder := flag.String("folder", ".", "folder investigated for modules")
	configPath := flag.String("config", "cmd/checkapi/config.yaml", "configuration file")
	flag.Parse()
	if err := run(*folder, *configPath); err != nil {
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

type functionDescription struct {
	Name        string   `yaml:"name"`
	Parameters  []string `yaml:"parameters"`
	ReturnTypes []string `yaml:"return_types"`
}

type config struct {
	IgnoredPaths     []string              `yaml:"ignored_paths"`
	AllowedFunctions []functionDescription `yaml:"allowed_functions"`
	IgnoredFunctions []string              `yaml:"ignored_functions"`
}

func run(folder string, configPath string) error {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg config
	err = yaml.Unmarshal(configData, &cfg)
	if err != nil {
		return err
	}
	var errs []error
	err = filepath.Walk(folder, func(path string, info fs.FileInfo, _ error) error {
		if info.Name() == "go.mod" {
			base := filepath.Dir(path)
			var relativeBase string
			relativeBase, err = filepath.Rel(folder, base)
			if err != nil {
				return err
			}
			componentType := strings.Split(relativeBase, string(os.PathSeparator))[0]
			switch componentType {
			case "receiver", "processor", "exporter", "connector", "extension":
			default:
				return nil
			}

			for _, a := range cfg.IgnoredPaths {
				if filepath.Join(filepath.SplitList(a)...) == relativeBase {
					fmt.Printf("Ignoring %s per denylist\n", base)
					return nil
				}
			}
			if err = walkFolder(cfg, base, componentType); err != nil {
				errs = append(errs, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func isFunctionIgnored(cfg config, fnName string) bool {
	for _, v := range cfg.IgnoredFunctions {
		reg := regexp.MustCompile(v)
		if reg.MatchString(fnName) {
			return true
		}
	}
	return false
}

func handleFile(cfg config, f *ast.File, result *api) {
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
			if fn.Recv.NumFields() == 0 && !isFunctionIgnored(cfg, fn.Name.String()) {
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
						returnTypes = append(returnTypes, internal.ExprToString(r.Type))
					}
				}
				var params []string
				if fn.Type.Params.NumFields() > 0 {
					for _, r := range fn.Type.Params.List {
						params = append(params, internal.ExprToString(r.Type))
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

func walkFolder(cfg config, folder string, _ string) error {
	result := &api{}
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, folder, nil, 0)
	if err != nil {
		return err
	}

	for _, pack := range packs {
		for _, f := range pack.Files {
			handleFile(cfg, f, result)
		}
	}
	sort.Strings(result.Structs)
	sort.Strings(result.Values)
	sort.Slice(result.Functions, func(i int, j int) bool {
		return strings.Compare(result.Functions[i].Name, result.Functions[j].Name) < 0
	})
	fnNames := make([]string, len(result.Functions))
	for i, fn := range result.Functions {
		fnNames[i] = fn.Name
	}
	if len(result.Structs) == 0 && len(result.Values) == 0 && len(result.Functions) == 0 {
		return nil
	}

	functionsPresent := map[string]struct{}{}
OUTER:
	for _, fnDesc := range cfg.AllowedFunctions {
		for _, fn := range result.Functions {
			if fn.Name == fnDesc.Name &&
				slices.Equal(fn.ParamTypes, fnDesc.Parameters) &&
				slices.Equal(fn.ReturnTypes, fnDesc.ReturnTypes) {
				functionsPresent[fn.Name] = struct{}{}
				break OUTER
			}
		}
	}
	if len(functionsPresent) == 0 {
		return fmt.Errorf("[%s] no function matching configuration found", folder)
	}
	return nil
}
