// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package translation

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/schema/v1.0/ast"
	"go.opentelemetry.io/otel/schema/v1.0/types"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

func compareFuncs(f1, f2 migrate.ResourceTestFunc[ptrace.Span]) bool {
	return fmt.Sprintf("%p", f1) == fmt.Sprintf("%p", f2)
}

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
				ver:                               &Version{1, 1, 1},
				all:                               migrate.NewAttributeChangeSetSlice(),
				resources:                         migrate.NewAttributeChangeSetSlice(),
				spans:                             migrate.NewConditionalAttributeSetSlice(),
				spanEventsRenameEvents:            migrate.NewSignalNameChangeSlice(),
				spanEventsRenameAttributesOnSpanEvent: migrate.NewConditionalLambdaAttributeSetSlice[ptrace.Span](),
				metricsRenameAttributes:           migrate.NewConditionalAttributeSetSlice(),
				metricsRenameMetrics:              migrate.NewSignalNameChangeSlice(),
				logsRenameAttributes:              migrate.NewAttributeChangeSetSlice(),
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
				all: migrate.NewAttributeChangeSetSlice(
					migrate.NewAttributeChangeSet(map[string]string{
						"state": "status",
					}),
					migrate.NewAttributeChangeSet(map[string]string{
						"status": "state",
					}),
				),
				resources: migrate.NewAttributeChangeSetSlice(
					migrate.NewAttributeChangeSet(map[string]string{
						"service_name": "service.name",
					}),
				),
				spans: migrate.NewConditionalAttributeSetSlice(
					migrate.NewConditionalAttributeSet(
						map[string]string{"service_version": "service.version"},
						"application start",
					),
					migrate.NewConditionalAttributeSet[string](
						map[string]string{"deployment.environment": "service.deployment.environment"},
					),
				),
				spanEventsRenameEvents: migrate.NewSignalNameChangeSlice(
					migrate.NewSignalNameChange(map[string]string{
						"started": "application started",
					}),
				),
				spanEventsRenameAttributesOnSpanEvent: migrate.NewConditionalLambdaAttributeSetSlice(
					migrate.NewConditionalLambdaAttributeSet[ptrace.Span](map[string]string{
							"service.app.name": "service.name",
						},
						nil,
					),
				),
				metricsRenameAttributes: migrate.NewConditionalAttributeSetSlice(
					migrate.NewConditionalAttributeSet(
						map[string]string{
							"runtime": "service.language",
						},
						"service.runtime",
					),
				),
				metricsRenameMetrics: migrate.NewSignalNameChangeSlice(
					migrate.NewSignalNameChange(map[string]string{
						"service.computed.uptime": "service.uptime",
					}),
				),
				logsRenameAttributes: migrate.NewAttributeChangeSetSlice(
					migrate.NewAttributeChangeSet(map[string]string{
						"ERROR": "error",
					}),
				),
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rev := NewRevision(tc.inVersion, tc.inDefinition)
			// set expected spanEventsRenameAttributesOnSpanEvent testFunc to the actual revision's testFunc
			if len(*tc.expect.spanEventsRenameAttributesOnSpanEvent) != 0 {
				element := (*tc.expect.spanEventsRenameAttributesOnSpanEvent)[0]
				element.TestFunc = (*rev.spanEventsRenameAttributesOnSpanEvent)[0].TestFunc
			}

			// use go-cmp to compare tc.expect and rev and fail the test if there's a difference
			if diff := cmp.Diff(tc.expect, rev, cmp.AllowUnexported(RevisionV1{}, migrate.AttributeChangeSet{}, migrate.ConditionalAttributeSet{}, migrate.SignalNameChange{}, migrate.ConditionalLambdaAttributeSet[ptrace.Span]{}), cmp.Comparer(compareFuncs)); diff != "" {
				t.Errorf("NewRevisionV1() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
