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

package docsgen // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/docsgen/docsgen"

import (
	"fmt"
	"strings"
	"text/template"
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
{{ range .Fields -}}
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
