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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"
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
				all: &changelist.ChangeList{Migrators: []migrate.Migrator{
					operator.AllOperator{
						// initialize one of each operator with the attribute set
						MetricOperator: operator.MetricAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"state": "status",
							}),
						},
						LogOperator: operator.LogAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"state": "status",
							}),
						},
						SpanOperator: operator.SpanAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"state": "status",
							}),
						},
						SpanEventOperator: operator.SpanEventAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"state": "status",
							}),
						},
						ResourceMigrator: migrate.NewAttributeChangeSet(map[string]string{
							"state": "status",
						}),
					},
					operator.AllOperator{
						// initialize one of each operator with the attribute set
						MetricOperator: operator.MetricAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"status": "state",
							}),
						},
						LogOperator: operator.LogAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"status": "state",
							}),
						},
						SpanOperator: operator.SpanAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"status": "state",
							}),
						},
						SpanEventOperator: operator.SpanEventAttributeOperator{
							AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
								"status": "state",
							}),
						},
						ResourceMigrator: migrate.NewAttributeChangeSet(map[string]string{
							"status": "state",
						}),
					},
				},
				},
				resources: &changelist.ChangeList{Migrators: []migrate.Migrator{
					migrate.NewAttributeChangeSet(map[string]string{
						"service_name": "service.name",
					}),
				}},
				spans: &changelist.ChangeList{Migrators: []migrate.Migrator{
					operator.SpanConditionalAttributeOperator{Migrator: migrate.NewConditionalAttributeSet(
						map[string]string{"service_version": "service.version"},
						"application start",
					)},
					operator.SpanConditionalAttributeOperator{Migrator: migrate.NewConditionalAttributeSet[string](
						map[string]string{"deployment.environment": "service.deployment.environment"},
					)},
				}},
				spanEvents: &changelist.ChangeList{Migrators: []migrate.Migrator{
					operator.SpanEventSignalNameChange{
						SignalNameChange: migrate.NewSignalNameChange(map[string]string{
							"started": "application started",
						}),
					},
					operator.NewSpanEventConditionalAttributeOperator(
						migrate.NewMultiConditionalAttributeSet(
							map[string]string{"service.app.name": "service.name"},
							map[string][]string{
								"span.name":  {"service running"},
								"event.name": {"service errored"},
							},
						),
					),
				}},
				metrics: &changelist.ChangeList{Migrators: []migrate.Migrator{
					operator.MetricSignalNameChange{SignalNameChange: migrate.NewSignalNameChange(map[string]string{
						"service.computed.uptime": "service.uptime",
					})},
					operator.MetricDataPointAttributeOperator{ConditionalAttributeChange: migrate.NewConditionalAttributeSet(
						map[string]string{"runtime": "service.language"},
						"service.runtime",
					)},
				}},
				logs: &changelist.ChangeList{Migrators: []migrate.Migrator{
					migrate.NewAttributeChangeSet(map[string]string{
						"ERROR": "error",
					}),
				}},
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rev := NewRevision(tc.inVersion, tc.inDefinition)

			// use go-cmp to compare tc.expect and rev and fail the test if there's a difference
			if diff := cmp.Diff(tc.expect, rev, cmp.AllowUnexported(RevisionV1{}, migrate.AttributeChangeSet{}, migrate.ConditionalAttributeSet{}, migrate.SignalNameChange{}, operator.SpanEventConditionalAttributeOperator{}, migrate.MultiConditionalAttributeSet{})); diff != "" {
				t.Errorf("NewRevisionV1() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
