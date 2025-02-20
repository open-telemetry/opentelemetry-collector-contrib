// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// aggregateAttributesProcessor
type aggregateAttributesProcessor struct {
	aggregations []*aggregation
}

type aggregation struct {
	attribute string
	prefixes  []string
}

func newAggregateAttributesProcessor(config []aggregationPair) *aggregateAttributesProcessor {
	aggregations := []*aggregation{}

	for i := range config {
		pair := &aggregation{
			attribute: config[i].Attribute,
			prefixes:  config[i].Prefixes,
		}
		aggregations = append(aggregations, pair)
	}

	return &aggregateAttributesProcessor{aggregations: aggregations}
}

func (proc *aggregateAttributesProcessor) processLogs(logs plog.Logs) error {
	for i := range logs.ResourceLogs().Len() {
		resourceLogs := logs.ResourceLogs().At(i)
		err := proc.processAttributes(resourceLogs.Resource().Attributes())
		if err != nil {
			return err
		}

		for j := range resourceLogs.ScopeLogs().Len() {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := range scopeLogs.LogRecords().Len() {
				err := proc.processAttributes(scopeLogs.LogRecords().At(k).Attributes())
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (proc *aggregateAttributesProcessor) processMetrics(metrics pmetric.Metrics) error {
	for i := range metrics.ResourceMetrics().Len() {
		resourceMetrics := metrics.ResourceMetrics().At(i)
		err := proc.processAttributes(resourceMetrics.Resource().Attributes())
		if err != nil {
			return err
		}

		for j := range resourceMetrics.ScopeMetrics().Len() {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := range scopeMetrics.Metrics().Len() {
				err := processMetricLevelAttributes(proc, scopeMetrics.Metrics().At(k))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (proc *aggregateAttributesProcessor) processTraces(traces ptrace.Traces) error {
	for i := range traces.ResourceSpans().Len() {
		resourceSpans := traces.ResourceSpans().At(i)
		err := proc.processAttributes(resourceSpans.Resource().Attributes())
		if err != nil {
			return err
		}

		for j := range resourceSpans.ScopeSpans().Len() {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			for k := range scopeSpans.Spans().Len() {
				err := proc.processAttributes(scopeSpans.Spans().At(k).Attributes())
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (proc *aggregateAttributesProcessor) isEnabled() bool {
	return len(proc.aggregations) != 0
}

func (*aggregateAttributesProcessor) ConfigPropertyName() string {
	return "aggregate_attributes"
}

func (proc *aggregateAttributesProcessor) processAttributes(attributes pcommon.Map) error {
	for i := range len(proc.aggregations) {
		curr := proc.aggregations[i]
		names := []string{}
		attrs := []pcommon.Value{}

		for j := range len(curr.prefixes) {
			prefix := curr.prefixes[j]
			// Create a new map. Unused keys will be added here,
			// so we can check them against other prefixes.
			newMap := pcommon.NewMap()
			newMap.EnsureCapacity(attributes.Len())

			attributes.Range(func(key string, value pcommon.Value) bool {
				ok, trimmedKey := getNewKey(key, prefix)
				if ok {
					// TODO: Potential name conflict to resolve, eg.:
					// pod_* matches pod_foo
					// pod2_* matches pod2_foo
					// both will be renamed to foo
					// ref: https://github.com/SumoLogic/sumologic-otel-collector/issues/1263
					names = append(names, trimmedKey)
					val := pcommon.NewValueEmpty()
					value.CopyTo(val)
					attrs = append(attrs, val)
				} else {
					value.CopyTo(newMap.PutEmpty(key))
				}
				return true
			})
			newMap.CopyTo(attributes)
		}

		if len(names) != len(attrs) {
			return fmt.Errorf(
				"internal error: number of values does not equal the number of keys; len(keys) = %d, len(values) = %d",
				len(names),
				len(attrs),
			)
		}

		// Add a new attribute only if there's anything that should be put under it.
		if len(names) > 0 {
			aggregated := attributes.PutEmptyMap(curr.attribute)

			for j := range names {
				attrs[j].CopyTo(aggregated.PutEmpty(names[j]))
			}
		}
	}

	return nil
}

// Checks if the key has given prefix and trims it if so.
func getNewKey(key string, prefix string) (bool, string) {
	if strings.HasPrefix(key, prefix) {
		return true, strings.TrimPrefix(key, prefix)
	}

	return false, ""
}
