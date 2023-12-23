// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"go.opentelemetry.io/collector/component"
)

const mdFileName = "config.md"

func GenerateConfigDoc(srcRoot string,
	f component.Factory,
) error {
	tableTmpl, err := tableTemplate()
	if err != nil {
		return err
	}
	cfg, err := GetCfgInfo(f)
	if err != nil {
		return err
	}

	return writeConfigDoc(tableTmpl, srcRoot, cfg, os.WriteFile)
}

type writeFileFunc func(filename string, data []byte, perm os.FileMode) error

func writeConfigDoc(
	tableTmpl *template.Template,
	srcRoot string,
	ci CfgInfo,
	writeFile writeFileFunc,
) error {
	v := reflect.ValueOf(ci.CfgInstance)
	f, err := ReadFields(v, srcRoot)
	if err != nil {
		panic(err)
	}

	mdBytes := renderHeader(string(ci.Type), f.Doc)

	f.Name = typeToName(f.Type)

	tableBytes, err := renderTable(tableTmpl, f)
	if err != nil {
		panic(err)
	}
	mdBytes = append(mdBytes, tableBytes...)

	if hasTimeDuration(f) {
		mdBytes = append(mdBytes, durationBlock...)
	}

	return writeFile(filepath.Join(srcRoot, mdFileName), mdBytes, 0644)
}

func typeToName(typ string) string {
	idx := strings.IndexRune(typ, '.')
	return typ[:idx]
}
