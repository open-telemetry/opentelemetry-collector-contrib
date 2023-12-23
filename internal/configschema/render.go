// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"

import (
	"bytes"
	"fmt"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func renderHeader(typ, doc string) []byte {
	caser := cases.Title(language.English)
	return []byte(fmt.Sprintf(
		"# %s Reference\n\n%s\n\n",
		caser.String(typ),
		doc,
	))
}

func renderTable(tmpl *template.Template, field *Field) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := executeTableTemplate(tmpl, field, buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func executeTableTemplate(tmpl *template.Template, field *Field, buf *bytes.Buffer) error {
	err := tmpl.Execute(buf, field)
	if err != nil {
		return err
	}
	for _, subField := range field.Fields {
		if subField.Fields == nil {
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

func hasTimeDuration(f *Field) bool {
	if f.Type == "time.Duration" {
		return true
	}
	for _, sub := range f.Fields {
		if hasTimeDuration(sub) {
			return true
		}
	}
	return false
}
