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
	tmplDir := filepath.Join(filepath.Dir(filename), "templates")

	codeDir := filepath.Join(ymlDir, "internal", "metadata")
	if err = os.MkdirAll(codeDir, 0700); err != nil {
		return fmt.Errorf("unable to create output directory %q: %w", codeDir, err)
	}
	if err = generateFile(filepath.Join(tmplDir, "metrics.go.tmpl"),
		filepath.Join(codeDir, "generated_metrics.go"), md); err != nil {
		return err
	}
	if err = generateFile(filepath.Join(tmplDir, "metrics_test.go.tmpl"),
		filepath.Join(codeDir, "generated_metrics_test.go"), md); err != nil {
		return err
	}
	return generateFile(filepath.Join(tmplDir, "documentation.md.tmpl"), filepath.Join(ymlDir, "documentation.md"), md)
}

func generateFile(tmplFile string, outputFile string, md metadata) error {
	tmpl := template.Must(
		template.
			New(filepath.Base(tmplFile)).
			Option("missingkey=error").
			Funcs(map[string]interface{}{
				"publicVar": func(s string) (string, error) {
					return formatIdentifier(s, true)
				},
				"attributeInfo": func(an attributeName) attribute {
					return md.Attributes[an]
				},
				"attributeName": func(an attributeName) string {
					if md.Attributes[an].NameOverride != "" {
						return md.Attributes[an].NameOverride
					}
					return string(an)
				},
				"metricInfo": func(mn metricName) metric {
					return md.Metrics[mn]
				},
				"parseImportsRequired": func(metrics map[metricName]metric) bool {
					for _, m := range metrics {
						if m.Data().HasMetricInputType() {
							return true
						}
					}
					return false
				},
				"stringsJoin": strings.Join,
			}).ParseFiles(tmplFile))

	buf := bytes.Buffer{}

	if err := tmpl.Execute(&buf, templateContext{metadata: md, Package: "metadata"}); err != nil {
		return fmt.Errorf("failed executing template: %w", err)
	}

	result := buf.Bytes()

	if strings.HasSuffix(outputFile, ".go") {
		var err error
		result, err = format.Source(buf.Bytes())
		if err != nil {
			errstr := strings.Builder{}
			_, _ = fmt.Fprintf(&errstr, "failed formatting source: %v", err)
			errstr.WriteString("--- BEGIN SOURCE ---")
			errstr.Write(buf.Bytes())
			errstr.WriteString("--- END SOURCE ---")
			return errors.New(errstr.String())
		}
	}

	if err := os.Remove(outputFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("unable to remove genererated file %q: %w", outputFile, err)
	}
	if err := os.WriteFile(outputFile, result, 0600); err != nil {
		return fmt.Errorf("failed writing %q: %w", outputFile, err)
	}

	return nil
}
