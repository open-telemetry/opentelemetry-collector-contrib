// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package translation

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ast10 "go.opentelemetry.io/otel/schema/v1.0/ast"
	"go.opentelemetry.io/otel/schema/v1.0/types"
	ast11 "go.opentelemetry.io/otel/schema/v1.1/ast"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/changelist"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"
)

func TestNewRevisionV1(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		inVersion    *Version
		inDefinition ast11.VersionDef
		expect       *RevisionV1
	}{
		{
			name:         "no definition defined",
			inVersion:    &Version{1, 1, 1},
			inDefinition: ast11.VersionDef{},
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
			inDefinition: ast11.VersionDef{
				All: ast10.Attributes{
					Changes: []ast10.AttributeChange{
						{
							RenameAttributes: &ast10.RenameAttributes{
								AttributeMap: ast10.AttributeMap{
									"state": "status",
								},
							},
						},
						{
							RenameAttributes: &ast10.RenameAttributes{
								AttributeMap: ast10.AttributeMap{
									"status": "state",
								},
							},
						},
					},
				},
				Resources: ast10.Attributes{
					Changes: []ast10.AttributeChange{
						{
							RenameAttributes: &ast10.RenameAttributes{
								AttributeMap: ast10.AttributeMap{
									"service_name": "service.name",
								},
							},
						},
					},
				},
				Spans: ast10.Spans{
					Changes: []ast10.SpansChange{
						{
							RenameAttributes: &ast10.AttributeMapForSpans{
								ApplyToSpans: []types.SpanName{
									"application start",
								},
								AttributeMap: ast10.AttributeMap{
									"service_version": "service.version",
								},
							},
						},
						{
							RenameAttributes: &ast10.AttributeMapForSpans{
								AttributeMap: ast10.AttributeMap{
									"deployment.environment": "service.deployment.environment",
								},
							},
						},
					},
				},
				SpanEvents: ast10.SpanEvents{
					Changes: []ast10.SpanEventsChange{
						{
							RenameEvents: &ast10.RenameSpanEvents{
								EventNameMap: map[string]string{
									"started": "application started",
								},
							},
						},
						{
							RenameAttributes: &ast10.RenameSpanEventAttributes{
								ApplyToSpans: []types.SpanName{
									"service running",
								},
								ApplyToEvents: []types.EventName{
									"service errored",
								},
								AttributeMap: ast10.AttributeMap{
									"service.app.name": "service.name",
								},
							},
						},
					},
				},
				Logs: ast10.Logs{
					Changes: []ast10.LogsChange{
						{
							RenameAttributes: &ast10.RenameAttributes{
								AttributeMap: ast10.AttributeMap{
									"ERROR": "error",
								},
							},
						},
					},
				},
				Metrics: ast11.Metrics{
					Changes: []ast11.MetricsChange{
						{
							RenameMetrics: map[types.MetricName]types.MetricName{
								"service.computed.uptime": "service.uptime",
							},
						},
						{
							RenameAttributes: &ast10.AttributeMapForMetrics{
								ApplyToMetrics: []types.MetricName{
									"service.runtime",
								},
								AttributeMap: ast10.AttributeMap{
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
								}, false),
							},
							LogAttributes: transformer.LogAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}, false),
							},
							SpanAttributes: transformer.SpanAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}, false),
							},
							SpanEventAttributes: transformer.SpanEventAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}, false),
							},
							ResourceAttributes: transformer.ResourceAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"state": "status",
								}, false),
							},
						},
						transformer.AllAttributes{
							// initialize one of each transformer with the attribute set
							MetricAttributes: transformer.MetricAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}, false),
							},
							LogAttributes: transformer.LogAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}, false),
							},
							SpanAttributes: transformer.SpanAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}, false),
							},
							SpanEventAttributes: transformer.SpanEventAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}, false),
							},
							ResourceAttributes: transformer.ResourceAttributes{
								AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
									"status": "state",
								}, false),
							},
						},
					},
				},
				resources: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.ResourceAttributes{AttributeChange: migrate.NewAttributeChangeSet(
						map[string]string{"service_name": "service.name"},
						false,
					)},
				}},
				spans: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.SpanConditionalAttributes{Migrator: migrate.NewConditionalAttributeSet(
						map[string]string{"service_version": "service.version"},
						false,
						"application start",
					)},
					transformer.SpanConditionalAttributes{Migrator: migrate.NewConditionalAttributeSet[string](
						map[string]string{"deployment.environment": "service.deployment.environment"},
						false,
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
							false,
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
						false,
						"service.runtime",
					)},
				}},
				logs: &changelist.ChangeList{Migrators: []migrate.Migrator{
					transformer.LogAttributes{
						AttributeChange: migrate.NewAttributeChangeSet(map[string]string{
							"ERROR": "error",
						}, false),
					},
				}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rev, err := NewRevision(tc.inVersion, tc.inDefinition, false)
			require.NoError(t, err)

			// use go-cmp to compare tc.expect and rev and fail the test if there's a difference
			if diff := cmp.Diff(tc.expect, rev, cmp.AllowUnexported(RevisionV1{}, migrate.AttributeChangeSet{}, migrate.ConditionalAttributeSet{}, migrate.SignalNameChange{}, transformer.SpanEventConditionalAttributes{}, migrate.MultiConditionalAttributeSet{})); diff != "" {
				t.Errorf("NewRevisionV1() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewRevisionInvalidSpanEvents(t *testing.T) {
	t.Parallel()

	// A span_events change with neither rename_events nor rename_attributes set
	// must return an error rather than panicking.
	_, err := NewRevision(&Version{1, 0, 0}, ast11.VersionDef{
		SpanEvents: ast10.SpanEvents{
			Changes: []ast10.SpanEventsChange{
				{}, // empty: neither field set
			},
		},
	}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "span_events")
}
