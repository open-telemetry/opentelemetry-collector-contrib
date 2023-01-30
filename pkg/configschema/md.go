// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/configschema"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"go.opentelemetry.io/collector/otelcol"
)

func generateMDFile(factories otelcol.Factories, readComments readCommentsFunc, writeFile writeFileFunc, componentType, componentName string) error {
	tableTmpl, err := tableTemplate()
	if err != nil {
		return err
	}
	ci, err := getCfgInfo(factories, componentType, componentName)
	if err != nil {
		return err
	}
	return writeMarkdown(tableTmpl, ci, writeFile, readComments)
}

func generateMDFiles(factories otelcol.Factories, readComments readCommentsFunc, writeFile writeFileFunc) error {
	tableTmpl, err := tableTemplate()
	if err != nil {
		return err
	}
	infos := getAllCfgInfos(factories)
	for _, ci := range infos {
		err = writeMarkdown(tableTmpl, ci, writeFile, readComments)
		if err != nil {
			return fmt.Errorf("failed to write markdown for type: %s: %w", ci.Type, err)
		}
	}
	return nil
}

type writeFileFunc func(v reflect.Value, data []byte) error

func writeMarkdown(tableTmpl *template.Template, ci cfgInfo, writeFile writeFileFunc, readComments readCommentsFunc) error {
	v := reflect.ValueOf(ci.CfgInstance)
	f, err := readFields(v, readComments)
	if err != nil {
		return fmt.Errorf("failed to read fields for type: %s: %w", ci.Type, err)
	}
	mdBytes := renderHeader(string(ci.Type), ci.Group, f.Doc)
	f.Name = typeToName(f.Type)
	tableBytes, err := renderTable(tableTmpl, f)
	if err != nil {
		return fmt.Errorf("failed to render table for type: %s: %w", ci.Type, err)
	}
	mdBytes = append(mdBytes, tableBytes...)
	if hasTimeDuration(f) {
		mdBytes = append(mdBytes, durationBlock...)
	}
	err = writeFile(v, mdBytes)
	if err != nil {
		return fmt.Errorf("failed to write file for type: %s: %w", ci.Type, err)
	}
	return nil
}

func typeToName(typ string) string {
	idx := strings.IndexRune(typ, '.')
	return typ[:idx]
}

type mdFileWriter struct {
	dr dirResolver
}

func (w mdFileWriter) writeFile(v reflect.Value, bytes []byte) error {
	dir := w.dr.reflectValueToProjectPath(v)
	if dir == "" {
		return errors.New("writeFile: skipping, local path not found for component")
	}
	fname := filepath.Join(dir, "config.md")
	fmt.Printf("writing file: %s\n", fname)
	return os.WriteFile(fname, bytes, 0600)
}
