// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Deprecated: [v0.92.0] This package is deprecated and will be removed in a future release.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187
package docsgen // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/docsgen/docsgen"

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"go.opentelemetry.io/collector/otelcol"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

const mdFileName = "config.md"

// CLI is the entrypoint for this package's functionality. It handles command-
// line arguments for the docsgen executable and produces config documentation
// for the specified components.
// Deprecated: [v0.92.0] This package is deprecated and will be removed in a future release.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187
func CLI(factories otelcol.Factories, dr configschema.DirResolver) {
	tableTmpl, err := tableTemplate()
	if err != nil {
		panic(err)
	}

	handleCLI(factories, dr, tableTmpl, os.WriteFile, os.Stdout, os.Args...)
}

func handleCLI(
	factories otelcol.Factories,
	dr configschema.DirResolver,
	tableTmpl *template.Template,
	writeFile writeFileFunc,
	wr io.Writer,
	args ...string,
) {
	if !(len(args) == 2 || len(args) == 3) {
		printLines(wr, "usage:", "docsgen all", "docsgen component-type component-name")
		return
	}

	componentType := args[1]
	if componentType == "all" {
		allComponents(dr, tableTmpl, factories, writeFile)
		return
	}

	singleComponent(dr, tableTmpl, factories, componentType, args[2], writeFile)
}

func printLines(wr io.Writer, lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(wr, line)
	}
}

func allComponents(
	dr configschema.DirResolver,
	tableTmpl *template.Template,
	factories otelcol.Factories,
	writeFile writeFileFunc,
) {
	configs := configschema.GetAllCfgInfos(factories)
	for _, cfg := range configs {
		writeConfigDoc(tableTmpl, dr, cfg, writeFile)
	}
}

func singleComponent(
	dr configschema.DirResolver,
	tableTmpl *template.Template,
	factories otelcol.Factories,
	componentType, componentName string,
	writeFile writeFileFunc,
) {
	cfg, err := configschema.GetCfgInfo(factories, componentType, componentName)
	if err != nil {
		panic(err)
	}

	writeConfigDoc(tableTmpl, dr, cfg, writeFile)
}

type writeFileFunc func(filename string, data []byte, perm os.FileMode) error

func writeConfigDoc(
	tableTmpl *template.Template,
	dr configschema.DirResolver,
	ci configschema.CfgInfo,
	writeFile writeFileFunc,
) {
	v := reflect.ValueOf(ci.CfgInstance)
	f, err := configschema.ReadFields(v, dr)
	if err != nil {
		panic(err)
	}

	mdBytes := renderHeader(string(ci.Type), ci.Group, f.Doc)

	f.Name = typeToName(f.Type)

	tableBytes, err := renderTable(tableTmpl, f)
	if err != nil {
		panic(err)
	}
	mdBytes = append(mdBytes, tableBytes...)

	if hasTimeDuration(f) {
		mdBytes = append(mdBytes, durationBlock...)
	}

	dir := dr.ReflectValueToProjectPath(v)
	if dir == "" {
		log.Printf("writeConfigDoc: skipping, local path not found for component: %s %s", ci.Group, ci.Type)
		return
	}
	err = writeFile(filepath.Join(dir, mdFileName), mdBytes, 0644)
	if err != nil {
		panic(err)
	}
}

func typeToName(typ string) string {
	idx := strings.IndexRune(typ, '.')
	return typ[:idx]
}
