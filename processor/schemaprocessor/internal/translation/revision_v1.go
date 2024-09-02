// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/schema/v1.0/ast"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

// RevisionV1 represents all changes that are to be
// applied to a signal at a given version.
// todo(ankit) implement split and rest of otel schema
type RevisionV1 struct {
	ver                               *Version
	all                               *migrate.AttributeChangeSetSlice
	resources                         *migrate.AttributeChangeSetSlice
	spans                             *migrate.ConditionalAttributeSetSlice
	spanEventsRenameEvents            *migrate.SignalNameChangeSlice
	spanEventsRenameAttributesOnSpanEvent *migrate.ConditionalLambdaAttributeSetSlice[ptrace.Span]
	metricsRenameMetrics              *migrate.SignalNameChangeSlice
	metricsRenameAttributes           *migrate.ConditionalAttributeSetSlice
	logsRenameAttributes              *migrate.AttributeChangeSetSlice
}

// NewRevision processes the VersionDef and assigns the version to this revision
// to allow sorting within a slice.
// Since VersionDef uses custom types for various definitions, it isn't possible
// to cast those values into the primitives so each has to be processed together.
// Generics would be handy here.
// todo(ankit) investigate using generics
func NewRevision(ver *Version, def ast.VersionDef) *RevisionV1 {
	// todo(ankit) change logs to be an ast.Attributes type so I dont have to change this
	var logChanges ast.Attributes
	for _, change := range def.Logs.Changes {
		logChanges.Changes = append(logChanges.Changes, ast.AttributeChange{RenameAttributes: change.RenameAttributes})
	}
	return &RevisionV1{
		ver:                               ver,
		all:                               newAttributeChangeSetSliceFromChanges(def.All),
		resources:                         newAttributeChangeSetSliceFromChanges(def.Resources),
		spans:                             newSpanConditionalAttributeSlice(def.Spans),
		spanEventsRenameEvents:            newSpanEventSignalSlice(def.SpanEvents),
		spanEventsRenameAttributesOnSpanEvent: newSpanEventsRenameAttributesOnSpanEventEvent(def.SpanEvents),
		metricsRenameAttributes:           newMetricConditionalSlice(def.Metrics),
		metricsRenameMetrics:              newMetricNameSignalSlice(def.Metrics),
		logsRenameAttributes:              newAttributeChangeSetSliceFromChanges(logChanges),
	}
}

func (r RevisionV1) Version() *Version {
	return r.ver
}

func newAttributeChangeSetSliceFromChanges(attrs ast.Attributes) *migrate.AttributeChangeSetSlice {
	values := make([]*migrate.AttributeChangeSet, 0, 10)
	for _, at := range attrs.Changes {
		if renamed := at.RenameAttributes; renamed != nil {
			values = append(values, migrate.NewAttributeChangeSet(renamed.AttributeMap))
		}
	}
	return migrate.NewAttributeChangeSetSlice(values...)
}

func newSpanConditionalAttributeSlice(spans ast.Spans) *migrate.ConditionalAttributeSetSlice {
	values := make([]*migrate.ConditionalAttributeSet, 0, 10)
	for _, ch := range spans.Changes {
		if renamed := ch.RenameAttributes; renamed != nil {
			values = append(values, migrate.NewConditionalAttributeSet(
				renamed.AttributeMap,
				renamed.ApplyToSpans...,
			))
		}
	}
	return migrate.NewConditionalAttributeSetSlice(values...)
}

func newSpanEventSignalSlice(events ast.SpanEvents) *migrate.SignalNameChangeSlice {
	values := make([]*migrate.SignalNameChange, 0, 10)
	for _, ch := range events.Changes {
		if renamed := ch.RenameEvents; renamed != nil {
			values = append(values, migrate.NewSignalNameChange(renamed.EventNameMap))
		}
	}
	return migrate.NewSignalNameChangeSlice(values...)
}

func newMetricConditionalSlice(metrics ast.Metrics) *migrate.ConditionalAttributeSetSlice {
	values := make([]*migrate.ConditionalAttributeSet, 0, 10)
	for _, ch := range metrics.Changes {
		if rename := ch.RenameAttributes; rename != nil {
			values = append(values, migrate.NewConditionalAttributeSet(rename.AttributeMap, rename.ApplyToMetrics...))
		}
	}
	return migrate.NewConditionalAttributeSetSlice(values...)
}

func newMetricNameSignalSlice(metrics ast.Metrics) *migrate.SignalNameChangeSlice {
	values := make([]*migrate.SignalNameChange, 0, 10)
	for _, ch := range metrics.Changes {
		values = append(values, migrate.NewSignalNameChange(ch.RenameMetrics))
	}
	return migrate.NewSignalNameChangeSlice(values...)
}

func generateSpanEventsRenameAttributes(ch ast.SpanEventsChange) func(span ptrace.Span) bool{
	return func(span ptrace.Span) bool {
		var spanNameMatches, spanEventMatches bool
		if ch.RenameAttributes.ApplyToSpans == nil || len(ch.RenameAttributes.ApplyToSpans) == 0 {
			spanNameMatches = true
		} else {
			for _, spanName := range ch.RenameAttributes.ApplyToSpans {
				if span.Name() == string(spanName) {
					spanNameMatches = true
					break
				}
			}
		}
		if ch.RenameAttributes.ApplyToEvents == nil || len(ch.RenameAttributes.ApplyToEvents) == 0 {
			spanEventMatches = true
		} else {
			for _, eventName := range ch.RenameAttributes.ApplyToEvents {
				for i := 0; i < span.Events().Len(); i++  {
					spanEvent := span.Events().At(i)
					if spanEvent.Name() == string(eventName) {
						spanEventMatches = true
						break
					}
				}
			}
		}
		return spanNameMatches && spanEventMatches
	}
}

func newSpanEventsRenameAttributesOnSpanEventEvent(spanEvents ast.SpanEvents) *migrate.ConditionalLambdaAttributeSetSlice[ptrace.Span]{
	var conditions []*migrate.ConditionalLambdaAttributeSet[ptrace.Span]
	for _, ch := range spanEvents.Changes {
		if rename := ch.RenameAttributes; rename != nil {
			conditions = append(conditions, migrate.NewConditionalLambdaAttributeSet(
				ch.RenameAttributes.AttributeMap,
				generateSpanEventsRenameAttributes(ch),
			))
		}
	}
	return migrate.NewConditionalLambdaAttributeSetSlice(conditions...)
}