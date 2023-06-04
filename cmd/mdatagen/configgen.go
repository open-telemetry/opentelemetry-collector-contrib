// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

var readMapStructure = regexp.MustCompile("`mapstructure:\"(\\w+).*`")

type configData struct {
	ConfigFields []FieldDoc
}

type FieldDoc struct {
	Name    string
	Comment string
	Tag     string
}

func generateConfig(componentDir string) error {
	configFields, err := readConfigDocs(componentDir)
	if err != nil {
		// explicitly ignore errors for now as not all components are expected to work
		return nil
	}
	return inlineConfigReplace(
		filepath.Join("templates", "config.md.tmpl"),
		filepath.Join(componentDir, "README.md"),
		configData{ConfigFields: configFields}, configStart, configEnd)
}

func readConfigDocs(folder string) ([]FieldDoc, error) {
	// Create the AST by parsing src and test.
	fset := token.NewFileSet()
	configFile := filepath.Join(folder, "config.go")
	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("config.go file not present")
	}
	fileContents, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	f, err := parser.ParseFile(fset, "config.go", string(fileContents), parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Compute package documentation with examples.
	p, err := doc.NewFromFiles(fset, []*ast.File{f}, "github.com/open-telemetry/opentelemetry-collector-contrib")
	if err != nil {
		return nil, err
	}
	var configType *doc.Type
	for _, t := range p.Types {
		if t.Name == "Config" {
			configType = t
		}
	}
	if configType == nil {
		return nil, errors.New("no Config type found")
	}
	if len(configType.Decl.Specs) != 1 {
		return nil, errors.New("no spec for the Config type")
	}
	spec := configType.Decl.Specs[0]
	typeSpec, ok := spec.(*ast.TypeSpec)
	if !ok {
		return nil, errors.New("config is not a type")
	}
	structTypeSpec, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return nil, errors.New("config is not a struct")
	}
	var fields []FieldDoc
	for _, f := range structTypeSpec.Fields.List {
		if len(f.Names) == 0 {
			continue
		}
		match := readMapStructure.FindStringSubmatch(f.Tag.Value)
		if len(match) == 2 {
			fields = append(fields, FieldDoc{
				Name:    f.Names[0].Name,
				Comment: strings.TrimSpace(f.Doc.Text()),
				Tag:     match[1],
			})
		}
	}
	return fields, nil
}

func inlineConfigReplace(tmplFile string, outputFile string, configData configData, start string, end string) error {
	var readmeContents []byte
	var err error
	if readmeContents, err = os.ReadFile(outputFile); err != nil {
		return err
	}

	var re = regexp.MustCompile(fmt.Sprintf("%s[\\s\\S]*%s", start, end))
	if !re.Match(readmeContents) {
		return nil
	}

	var tmpl *template.Template
	if tmpl, err = template.New(filepath.Base(tmplFile)).ParseFS(templateFS, strings.ReplaceAll(tmplFile, "\\", "/")); err != nil {
		return err
	}
	buf := bytes.Buffer{}

	if err := tmpl.Execute(&buf, configData); err != nil {
		return fmt.Errorf("failed executing template: %w", err)
	}

	result := buf.String()

	s := re.ReplaceAllString(string(readmeContents), result)
	if err := os.WriteFile(outputFile, []byte(s), 0600); err != nil {
		return fmt.Errorf("failed writing %q: %w", outputFile, err)
	}

	return nil
}
