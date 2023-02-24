// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
	"go.opentelemetry.io/otel/schema/v1.0/types"
)

func TestNewRevision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		in       ast.VersionDef
		expect   Revision
	}{
		{
			scenario: "empty version def",
			in:       ast.VersionDef{},
			expect: &revision{
				ver:      &Version{1, 0, 0},
				all:      NopModifier{},
				resource: NopModifier{},
				logs:     NopModifier{},
				spans:    NopModifier{},
				events:   NopModifier{},
				metrics:  NopModifier{},
			},
		},
		{
			scenario: "All definitions & Resources",
			in: ast.VersionDef{
				All: ast.Attributes{
					Changes: []ast.AttributeChange{
						{
							RenameAttributes: &ast.AttributeMap{
								"docker.io": "moby.io",
							},
						},
						{
							RenameAttributes: &ast.AttributeMap{
								"kubernetes": "k8s",
							},
						},
					},
				},
				Resources: ast.Attributes{
					Changes: []ast.AttributeChange{
						{
							RenameAttributes: &ast.AttributeMap{
								"opentelemetry": "otel",
							},
						},
					},
				},
			},
			expect: &revision{
				ver: &Version{1, 0, 0},
				all: &modify{
					names:     make(map[string]string),
					appliesTo: make(map[string]struct{}),
					attrs: map[string]string{
						"docker.io":  "moby.io",
						"kubernetes": "k8s",
					},
				},
				resource: &modify{
					names:     make(map[string]string),
					appliesTo: make(map[string]struct{}),
					attrs: map[string]string{
						"opentelemetry": "otel",
					},
				},
				logs:    NopModifier{},
				spans:   NopModifier{},
				events:  NopModifier{},
				metrics: NopModifier{},
			},
		},
		{
			scenario: "Only update logs",
			in: ast.VersionDef{
				Logs: ast.Logs{
					Changes: []ast.LogsChange{
						{
							RenameAttributes: &ast.RenameAttributes{
								AttributeMap: ast.AttributeMap{
									"logger.implementation": "logger.package",
								},
							},
						},
					},
				},
			},
			expect: &revision{
				ver:      &Version{1, 0, 0},
				all:      NopModifier{},
				resource: NopModifier{},
				logs: &modify{
					names:     make(map[string]string),
					appliesTo: make(map[string]struct{}),
					attrs: map[string]string{
						"logger.implementation": "logger.package",
					},
				},
				spans:   NopModifier{},
				events:  NopModifier{},
				metrics: NopModifier{},
			},
		},
		{
			scenario: "Metrics modifications",
			in: ast.VersionDef{
				Metrics: ast.Metrics{
					Changes: []ast.MetricsChange{
						{
							RenameMetrics: map[types.MetricName]types.MetricName{
								"uptime": "system.uptime",
							},
							RenameAttributes: &ast.AttributeMapForMetrics{
								ApplyToMetrics: []types.MetricName{
									"uptime",
								},
								AttributeMap: ast.AttributeMap{
									"host": "system.name",
								},
							},
						},
						{
							RenameMetrics: map[types.MetricName]types.MetricName{
								"network.io": "host.network.io",
							},
							RenameAttributes: &ast.AttributeMapForMetrics{
								AttributeMap: ast.AttributeMap{
									"ethernet": "interface",
								},
							},
						},
					},
				},
			},
			expect: &revision{
				ver:      &Version{1, 0, 0},
				all:      NopModifier{},
				resource: NopModifier{},
				logs:     NopModifier{},
				spans:    NopModifier{},
				events:   NopModifier{},
				metrics: modifications{
					{
						names: map[string]string{
							"uptime": "system.uptime",
						},
						appliesTo: map[string]struct{}{
							"uptime": {},
						},
						attrs: map[string]string{
							"host": "system.name",
						},
					},
					{
						names: map[string]string{
							"network.io": "host.network.io",
						},
						appliesTo: make(map[string]struct{}),
						attrs: map[string]string{
							"ethernet": "interface",
						},
					},
				},
			},
		},
		{
			scenario: "Span modifications",
			in: ast.VersionDef{
				Spans: ast.Spans{
					Changes: []ast.SpansChange{
						{
							RenameAttributes: &ast.AttributeMapForSpans{
								ApplyToSpans: []types.SpanName{
									"Operation Unknown",
								},
								AttributeMap: ast.AttributeMap{
									"username": "user.name",
									"userID":   "user.id",
								},
							},
						},
					},
				},
			},
			expect: &revision{
				ver:      &Version{1, 0, 0},
				all:      NopModifier{},
				resource: NopModifier{},
				logs:     NopModifier{},
				events:   NopModifier{},
				metrics:  NopModifier{},
				spans: modifications{
					{
						names: make(map[string]string),
						appliesTo: map[string]struct{}{
							"Operation Unknown": {},
						},
						attrs: map[string]string{
							"username": "user.name",
							"userID":   "user.id",
						},
					},
				},
			},
		},
		{
			scenario: "Span Event modifications",
			in: ast.VersionDef{
				SpanEvents: ast.SpanEvents{
					Changes: []ast.SpanEventsChange{
						{
							RenameEvents: &ast.RenameSpanEvents{
								EventNameMap: map[string]string{
									"database.tranaction": "db.transaction",
									"database.fault":      "db.fault",
								},
							},
							RenameAttributes: &ast.RenameSpanEventAttributes{
								ApplyToSpans: []types.SpanName{
									"Operation Unknown",
								},
								ApplyToEvents: []types.EventName{
									"transaction.failure",
									"db.failure",
								},
								AttributeMap: ast.AttributeMap{
									"cloud.technology": "cloud.provider",
								},
							},
						},
					},
				},
			},
			expect: &revision{
				ver:      &Version{1, 0, 0},
				all:      NopModifier{},
				resource: NopModifier{},
				logs:     NopModifier{},
				spans:    NopModifier{},
				metrics:  NopModifier{},
				events: modifications{
					{
						names: map[string]string{
							"database.tranaction": "db.transaction",
							"database.fault":      "db.fault",
						},
						appliesTo: map[string]struct{}{
							"Operation Unknown":   {},
							"transaction.failure": {},
							"db.failure":          {},
						},
						attrs: map[string]string{
							"cloud.technology": "cloud.provider",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			rev := newRevision(&Version{1, 0, 0}, tc.in)
			assert.EqualValues(t, tc.expect, rev)
		})
	}
}
