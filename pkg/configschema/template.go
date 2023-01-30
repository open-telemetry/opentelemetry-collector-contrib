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
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func tableTemplate() (*template.Template, error) {
	return template.New("table").Option("missingkey=zero").Funcs(
		template.FuncMap{
			"join":            join,
			"mkAnchor":        mkAnchor,
			"isCompoundField": isCompoundField,
			"isDuration":      isDuration,
		},
	).Parse(tableTemplateStr)
}

func isCompoundField(kind string) bool {
	return kind == "struct" || kind == "ptr"
}

func join(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}

// mkAnchor takes a name and a type (e.g. "configtls.TLSClientSetting") and
// returns a string suitable for use as a markdown anchor.
func mkAnchor(name, typ string) string {
	if isDuration(typ) {
		return "time-Duration"
	}
	idx := strings.IndexRune(typ, '.')
	// strip "configtls." from e.g. "configtls.TLSClientSetting"
	typeStripped := typ[idx+1:]
	concat := fmt.Sprintf("%s-%s", name, typeStripped)
	asterisksRemoved := strings.ReplaceAll(concat, "*", "")
	dotsToDashes := strings.ReplaceAll(asterisksRemoved, ".", "-")
	return strings.ReplaceAll(dotsToDashes, "_", "-")
}

func isDuration(s string) bool {
	return s == "time.Duration"
}

const tableTemplateStr = `### {{ mkAnchor .Name .Type }}

| Name | Field Info | Default | Docs |
| ---- | --------- | ------- | ---- |
{{ range .CfgFields -}}
| {{ .Name }} |
{{- if .Type -}}
	{{- $anchor := mkAnchor .Name .Type -}}
	{{- if isCompoundField .Kind -}}
			[{{ $anchor }}](#{{ $anchor }})
	{{- else -}}
		{{- if isDuration .Type -}}
			[{{ $anchor }}](#{{ $anchor }})
		{{- else -}}
			{{ .Type }}
		{{- end -}}
	{{- end -}}
{{- else -}}
	{{ .Kind }}
{{- end -}}
| {{ .Default }} | {{ join .Doc }} |
{{ end }}
`

func renderHeader(typ, group, doc string) []byte {
	caser := cases.Title(language.English)
	return []byte(fmt.Sprintf(
		"# %s %s Reference\n\n%s\n\n",
		caser.String(typ),
		caser.String(group),
		doc,
	))
}

func renderTable(tmpl *template.Template, field *cfgField) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := executeTableTemplate(tmpl, field, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func executeTableTemplate(tmpl *template.Template, field *cfgField, buf *bytes.Buffer) error {
	err := tmpl.Execute(buf, field)
	if err != nil {
		return err
	}
	for _, subField := range field.CfgFields {
		if subField.CfgFields == nil {
			continue
		}
		err = executeTableTemplate(tmpl, subField, buf)
		if err != nil {
			return err
		}
	}
	return nil
}

const durationBlock = "### time-Duration \n" +
	"An optionally signed sequence of decimal numbers, " +
	"each with a unit suffix, such as `300ms`, `-1.5h`, " +
	"or `2h45m`. Valid time units are `ns`, `us`, `ms`, `s`, `m`, `h`."

func hasTimeDuration(f *cfgField) bool {
	if f.Type == "time.Duration" {
		return true
	}
	for _, sub := range f.CfgFields {
		if hasTimeDuration(sub) {
			return true
		}
	}
	return false
}
