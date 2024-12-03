// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package translation

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
	"go.opentelemetry.io/otel/schema/v1.0/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/changelist"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"
)

func TestNewRevisionV1(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		inVersion    *Version
		inDefinition ast.VersionDef
		expect       *RevisionV1
	}{
		{
			name:         "no definition defined",
			inVersion:    &Version{1, 1, 1},
			inDefinition: ast.VersionDef{},
			expect: &RevisionV1{
				ver:        &Version{1, 1, 1},
				all:        &changelist.ChangeList{Migrators: make([]migrate.Migrator, 0)},
				resources:  &changelist.ChangeList{Migrators: make([]migrate.Migrator, 0)},
				spans:      &changelist.ChangeList{Migrators: make([]migrate.Migrator, 0)},
				spanEvents: &changelist.ChangeList{Migrators: make([]migrate.Migrator, 0)},
				metrics:    &changelist.ChangeList{Migrators: make([]migrate.Migrator, 0)},
				logs:       &changelist.ChangeList{Migrators: make([]migrate.Migrator, 0)},
			},
		},
		{
			name:      "complete version definition used",
			inVersion: &Version{1, 0, 0},
			inDefinition: ast.VersionDef{
				All: ast.Attributes{
					Changes: []ast.AttributeChange{
						{
							RenameAttributes: &ast.RenameAttributes{
								AttributeMap: ast.AttributeMap{
									"state": "status",
								},
							},
						},
						{
							RenameAttributes: &ast.RenameAttributes{
								AttributeMap: ast.AttributeMap{
									"status": "state",
								},
							},
						},
					},
				},
				Resources: ast.Attributes{
					Changes: []ast.AttributeChange{
						{
							RenameAttributes: &ast.RenameAttributes{
								AttributeMap: ast.AttributeMap{
									"service_name": "service.name",
								},
							},
						},
					},
				},
				Spans: ast.Spans{
					Changes: []ast.SpansChange{
						{
							RenameAttributes: &ast.AttributeMapForSpans{
								ApplyToSpans: []types.SpanName{
									"application start",
								},
								AttributeMap: ast.AttributeMap{
									"service_version": "service.version",
								},
							},
						},
						{
							RenameAttributes: &ast.AttributeMapForSpans{
								AttributeMap: ast.AttributeMap{
									"deployment.environment": "service.deployment.environment",
								},
							},
						},
					},
				},
				SpanEvents: ast.SpanEvents{
					Changes: []ast.SpanEventsChange{
						{
							RenameEvents: &ast.RenameSpanEvents{
								EventNameMap: map[string]string{
									"started": "application started",
								},
							},
						},
						{
							RenameAttributes: &ast.RenameSpanEventAttributes{
								ApplyToSpans: []types.SpanName{
									"service running",
								},
								ApplyToEvents: []types.EventName{
									"service errored",
								},
								AttributeMap: ast.AttributeMap{
									"service.app.name": "service.name",
								},
							},
						},
					},
				},
				Logs: ast.Logs{
					Changes: []ast.LogsChange{
						{
							RenameAttributes: &ast.RenameAttributes{
								AttributeMap: ast.AttributeMap{
									"ERROR": "error",
								},
							},
						},
					},
				},
				Metrics: ast.Metrics{
					Changes: []ast.MetricsChange{
						{
							RenameMetrics: map[types.MetricName]types.MetricName{
								"service.computed.uptime": "service.uptime",
							},
						},
						{
							RenameAttributes: &ast.AttributeMapForMetrics{
								ApplyToMetrics: []types.MetricName{
									"service.runtime",
								},
								AttributeMap: ast.AttributeMap{
									"runtime": "service.language",
								},
							},
						},
					},
				},
			},
			expect: &RevisionV1{
				ver: &Version{1, 0, 0},
				all: &changelist.ChangeList{
					Migrators: []migrate.Migrator{
						transformer.AllAttributes{
							// initialize one of each transformer with the attribute set
							MetricAttributes: transformer.MetricAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}),
							},
							LogAttributes: transformer.LogAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}),
							},
							SpanAttributes: transformer.SpanAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}),
							},
							SpanEventAttributes: transformer.SpanEventAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}),
							},
							ResourceAttributes: transformer.ResourceAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}),
							},
						},
						transformer.AllAttributes{
							// initialize one of each transformer with the attribute set
							MetricAttributes: transformer.MetricAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}),
							},
							LogAttributes: transformer.LogAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}),
							},
							SpanAttributes: transformer.SpanAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}),
							},
							SpanEventAttributes: transformer.SpanEventAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}),
							},
							ResourceAttributes: transformer.ResourceAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}),
							},
						},
					},
				},
				resources: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.ResourceAttributes{AttributeChange: migrate.NewAttributeChangeSet(
						map[string]string{"service_name": "service.name"},
					)},
				}},
				spans: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.SpanConditionalAttributes{Migrator: migrate.NewConditionalAttributeSet(
						map[string]string{"service_version": "service.version"},
						"application start",
					)},
					transformer.SpanConditionalAttributes{Migrator: migrate.NewConditionalAttributeSet[string](
						map[string]string{"deployment.environment": "service.deployment.environment"},
					)},
				}},
				spanEvents: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.SpanEventSignalNameChange{
						SignalNameChange: migrate.NewSignalNameChange(map[string]string{
							"started": "application started",
						}),
					},
					transformer.SpanEventConditionalAttributes{
						MultiConditionalAttributeSet: migrate.NewMultiConditionalAttributeSet(
							map[string]string{"service.app.name": "service.name"},
							map[string][]string{
								"span.name":  {"service running"},
								"event.name": {"service errored"},
							},
						),
					},
				}},
				metrics: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.MetricSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(map[string]string{
						"service.computed.uptime": "service.uptime",
					})},
					transformer.MetricDataPointAttributes{ConditionalAttributeChange: migrate.NewConditionalAttributeSet(
						map[string]string{"runtime": "service.language"},
						"service.runtime",
					)},
				}},
				logs: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.LogAttributes{
						AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
							"ERROR": "error",
						}),
					},
				}},
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rev := NewRevision(tc.inVersion, tc.inDefinition)

			// use go-cmp to compare tc.expect and rev and fail the test if there's a difference
			if diff := cmp.Diff(tc.expect, rev, cmp.AllowUnexported(RevisionV1{}, migrate.AttributeChangeSet{}, migrate.ConditionalAttributeSet{}, migrate.SignalNameChange{}, transformer.SpanEventConditionalAttributes{}, migrate.MultiConditionalAttributeSet{})); diff != "" {
				t.Errorf("NewRevisionV1() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
