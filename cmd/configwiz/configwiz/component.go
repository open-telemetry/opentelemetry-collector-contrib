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

package configwiz

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

func serviceToComponentNames(service map[string]interface{}) map[string][]string {
	out := map[string][]string{}
	for _, v := range service {
		m := v.(map[string]interface{})
		for _, v2 := range m {
			r := v2.(rpe)
			if r.Receivers != nil {
				out["receiver"] = append(out["receiver"], r.Receivers...)
			}
			if r.Processors != nil {
				out["processor"] = append(out["processor"], r.Processors...)
			}
			if r.Exporters != nil {
				out["exporter"] = append(out["exporter"], r.Exporters...)
			}
		}
	}
	return out
}

func handleComponent(
	io Clio,
	factories component.Factories,
	m map[string]interface{},
	componentGroup string,
	names []string,
	dr configschema.DirResolver,
) {
	typeMap := map[string]interface{}{}
	m[componentGroup+"s"] = typeMap
	pr := io.newIndentingPrinter(0)
	for _, name := range names {
		cfgInfo, err := configschema.GetCfgInfo(factories, componentGroup, strings.Split(name, "/")[0])
		if err != nil {
			panic(err)
		}
		pr.println(fmt.Sprintf("%s %q\n", strings.Title(componentGroup), name))
		f, err := configschema.ReadFields(reflect.ValueOf(cfgInfo.CfgInstance), dr)
		if err != nil {
			panic(err)
		}
		typeMap[name] = componentWizard(io, 1, f)
	}
}

func componentWizard(io Clio, lvl int, f *configschema.Field) map[string]interface{} {
	out := map[string]interface{}{}
	p := io.newIndentingPrinter(lvl)
	for _, field := range f.Fields {
		switch {
		case field.Name == "squash":
			componentWizard(io, lvl, field)
		case field.Kind == "struct":
			p.println(field.Name)
			out[field.Name] = componentWizard(io, lvl+1, field)
		case field.Kind == "ptr":
			p.print(fmt.Sprintf("%s (optional) skip (Y/n)> ", field.Name))
			in := io.Read("")
			if in == "n" {
				out[field.Name] = componentWizard(io, lvl+1, field)
			}
		default:
			handleField(io, p, field, out)
		}
	}
	return out
}

func handleField(io Clio, pr indentingPrinter, field *configschema.Field, out map[string]interface{}) {
	pr.println("Field: " + field.Name)
	typ := resolveType(field)
	if typ != "" {
		typString := "Type: " + typ
		if typ == "time.Duration" {
			typString += " (examples: 1h2m3s, 5m10s, 45s)"
		}
		pr.println(typString)
	}
	if field.Doc != "" {
		pr.println("Docs: " + strings.ReplaceAll(field.Doc, "\n", " "))
	}
	defaultVal := ""
	if field.Default != nil {
		pr.println(fmt.Sprintf("Default (enter to accept): %v", field.Default))
		defaultVal = fmt.Sprintf("%v", field.Default)
	}
	pr.print("> ")
	in := io.Read(defaultVal)
	if in == "" {
		return
	}
	switch field.Kind {
	case "bool":
		out[field.Name], _ = strconv.ParseBool(in)
	case "int", "int8", "int16", "int32", "int64":
		if field.Type == "time.Duration" {
			out[field.Name], _ = time.ParseDuration(in)
		} else {
			out[field.Name], _ = strconv.Atoi(in)
		}
	case "float32", "float64":
		out[field.Name], _ = strconv.ParseFloat(in, 32)
	case "[]string":
		out[field.Name] = parseCSV(in)
	default:
		out[field.Name] = in
	}
}

func parseCSV(in string) []string {
	a := strings.Split(in, ",")
	for i, s := range a {
		a[i] = strings.TrimSpace(s)
	}
	return a
}

func resolveType(f *configschema.Field) string {
	if f.Type != "" {
		return f.Type
	}
	return f.Kind
}
