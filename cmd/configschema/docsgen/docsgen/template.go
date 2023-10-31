// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
