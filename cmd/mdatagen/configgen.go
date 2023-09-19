// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/atombender/go-jsonschema/pkg/schemas"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen/third_party/gojsonschemagenerator"
)

const (
	CONFIG_NAME = "config"
)

func GenerateConfig(conf any) error {
	// load config
	jsonBytes, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed loading config %w", err)
	}
	var schema schemas.Schema
	if err := json.Unmarshal(jsonBytes, &schema); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// todo: don't hardcode
	pkgName := "fileexporter"

	// init generator
	cfg := gojsonschemagenerator.Config{
		Warner: func(message string) {
			logf("Warning: %s", message)
		},
		DefaultPackageName:  pkgName,
		DefaultOutputName:   "config",
		StructNameFromTitle: true,
		Tags:                []string{"json", "yaml", "mapstructure"},
		SchemaMappings:      []gojsonschemagenerator.SchemaMapping{},
		YAMLPackage:         "gopkg.in/yaml.v3",
		YAMLExtensions:      []string{".yaml", ".yml"},
	}

	generator, err := gojsonschemagenerator.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}
	if err = generator.AddFile(CONFIG_NAME, &schema); err != nil {
		return fmt.Errorf("failed to add config: %w", err)
	}

	hasUnsupportedValidations := len(generator.NotSupportedValidations) > 0

	tplVars := struct {
		ValidatorFuncName string
	}{
		ValidatorFuncName: "Validate",
	}
	if hasUnsupportedValidations {
		tplVars.ValidatorFuncName = "ValidateHelper"
	}

	tpl := `
func (cfg *Config){{.ValidatorFuncName}}() error {
	b, err := json.Marshal(cfg)
	if err != nil {
			return err
	}
	var config Config
	if err := json.Unmarshal(b, &config); err != nil {
			return err
	}
	return nil
}`
	tmpl, err := template.New("validator").Parse(tpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	for _, source := range generator.Sources() {
		file, err := os.Create("/tmp/config.go")
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		buf := bytes.NewBufferString("")
		if err = tmpl.Execute(buf, tplVars); err != nil {
			return fmt.Errorf("failed to execute template: %w", err)
		}
		// only write custom validation if there are no unsupported validations
		// source = append(source, []byte(tpl)...)
		source = append(source, buf.Bytes()...)
		_, err = file.Write(source)

		if err != nil {
			return fmt.Errorf("failed writing file: %w", err)
		}
	}
	fmt.Println("done")
	return nil
}

func logf(format string, args ...interface{}) {
	fmt.Fprint(os.Stderr, "go-jsonschema: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprint(os.Stderr, "\n")
}

func dump(v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(jsonBytes))
}
