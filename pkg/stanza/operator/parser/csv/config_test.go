// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package csv

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "basic",
				Expect: func() *Config {
					p := NewConfig()
					p.Header = "id,severity,message"
					p.ParseFrom = entry.NewBodyField("message")
					return p
				}(),
			},
			{
				Name: "lazy_quotes",
				Expect: func() *Config {
					p := NewConfig()
					p.Header = "id,severity,message"
					p.LazyQuotes = true
					p.ParseFrom = entry.NewBodyField("message")
					return p
				}(),
			},
			{
				Name: "ignore_quotes",
				Expect: func() *Config {
					p := NewConfig()
					p.Header = "id,severity,message"
					p.IgnoreQuotes = true
					p.ParseFrom = entry.NewBodyField("message")
					return p
				}(),
			},
			{
				Name: "delimiter",
				Expect: func() *Config {
					p := NewConfig()
					p.Header = "id,severity,message"
					p.ParseFrom = entry.NewBodyField("message")
					p.FieldDelimiter = "\t"
					return p
				}(),
			},
			{
				Name: "header_attribute",
				Expect: func() *Config {
					p := NewConfig()
					p.HeaderAttribute = "header_field"
					p.ParseFrom = entry.NewBodyField("message")
					p.FieldDelimiter = "\t"
					return p
				}(),
			},
			{
				Name: "timestamp",
				Expect: func() *Config {
					p := NewConfig()
					p.Header = "timestamp_field,severity,message"
					newTime := helper.NewTimeParser()
					p.TimeParser = &newTime
					parseFrom := entry.NewBodyField("timestamp_field")
					p.TimeParser.ParseFrom = &parseFrom
					p.TimeParser.LayoutType = "strptime"
					p.TimeParser.Layout = "%Y-%m-%d"
					return p
				}(),
			},
			{
				Name: "parse_to_attributes",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
					return p
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
					return p
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewResourceField()}
					return p
				}(),
			},
		},
	}.Run(t)
}
