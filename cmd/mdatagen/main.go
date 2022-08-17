// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

const (
	tmplFile   = "metrics.tmpl"
	outputFile = "generated_metrics.go"
)

func main() {
	flag.Parse()
	yml := flag.Arg(0)
	if err := run(yml); err != nil {
		log.Fatal(err)
	}
}

func run(ymlPath string) error {
	if ymlPath == "" {
		return errors.New("argument must be metadata.yaml file")
	}

	ymlDir := filepath.Dir(ymlPath)

	md, err := loadMetadata(filepath.Clean(ymlPath))
	if err != nil {
		return fmt.Errorf("failed loading %v: %w", ymlPath, err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("unable to determine filename")
	}
	thisDir := filepath.Dir(filename)

	if err = generateMetrics(ymlDir, thisDir, md); err != nil {
		return err
	}
	return generateDocumentation(ymlDir, thisDir, md)
}

func generateMetrics(ymlDir string, thisDir string, md metadata) error {
	tmpl := template.Must(
		template.
			New(tmplFile).
			Option("missingkey=error").
			Funcs(map[string]interface{}{
				"publicVar": func(s string) (string, error) {
					return formatIdentifier(s, true)
				},
				"attributeInfo": func(an attributeName) attribute {
					return md.Attributes[an]
				},
				"attributeKey": func(an attributeName) string {
					if md.Attributes[an].Value != "" {
						return md.Attributes[an].Value
					}
					return string(an)
				},
				"parseImportsRequired": func(metrics map[metricName]metric) bool {
					for _, m := range metrics {
						if m.Data().HasMetricInputType() {
							return true
						}
					}
					return false
				},
			}).ParseFiles(filepath.Join(thisDir, tmplFile)))
	buf := bytes.Buffer{}

	if err := tmpl.Execute(&buf, templateContext{metadata: md, Package: "metadata"}); err != nil {
		return fmt.Errorf("failed executing template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())

	if err != nil {
		errstr := strings.Builder{}
		_, _ = fmt.Fprintf(&errstr, "failed formatting source: %v", err)
		errstr.WriteString("--- BEGIN SOURCE ---")
		errstr.Write(buf.Bytes())
		errstr.WriteString("--- END SOURCE ---")
		return errors.New(errstr.String())
	}

	outputDir := filepath.Join(ymlDir, "internal", "metadata")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return fmt.Errorf("unable to create output directory %q: %w", outputDir, err)
	}
	outputFilepath := filepath.Join(outputDir, outputFile)
	if err := os.Remove(outputFilepath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("unable to remove genererated file %q: %w", outputFilepath, err)
	}
	if err := os.WriteFile(outputFilepath, formatted, 0600); err != nil {
		return fmt.Errorf("failed writing %q: %w", outputFilepath, err)
	}

	return nil
}

func generateDocumentation(ymlDir string, thisDir string, md metadata) error {
	tmpl := template.Must(
		template.
			New("documentation.tmpl").
			Option("missingkey=error").
			Funcs(map[string]interface{}{
				"publicVar": func(s string) (string, error) {
					return formatIdentifier(s, true)
				},
				"stringsJoin": strings.Join,
			}).ParseFiles(filepath.Join(thisDir, "documentation.tmpl")))

	buf := bytes.Buffer{}

	tmplCtx := templateContext{metadata: md, Package: "metadata"}
	if err := tmpl.Execute(&buf, tmplCtx); err != nil {
		return fmt.Errorf("failed executing template: %w", err)
	}

	outputFile := filepath.Join(ymlDir, "documentation.md")
	if err := os.WriteFile(outputFile, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed writing %q: %w", outputFile, err)
	}

	return nil
}
