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

	for i := 0; i < len(config); i++ {
		pair := &aggregation{
			attribute: config[i].Attribute,
			prefixes:  config[i].Prefixes,
		}
		aggregations = append(aggregations, pair)
	}

	return &aggregateAttributesProcessor{aggregations: aggregations}
}

func (proc *aggregateAttributesProcessor) processLogs(logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		err := proc.processAttributes(resourceLogs.Resource().Attributes())
		if err != nil {
			return err
		}

		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
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
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		resourceMetrics := metrics.ResourceMetrics().At(i)
		err := proc.processAttributes(resourceMetrics.Resource().Attributes())
		if err != nil {
			return err
		}

		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
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
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		resourceSpans := traces.ResourceSpans().At(i)
		err := proc.processAttributes(resourceSpans.Resource().Attributes())
		if err != nil {
			return err
		}

		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
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
	for i := 0; i < len(proc.aggregations); i++ {
		curr := proc.aggregations[i]
		names := []string{}
		attrs := []pcommon.Value{}

		for j := 0; j < len(curr.prefixes); j++ {
			prefix := curr.prefixes[j]
			// Create a new map. Unused keys will be added here,
			// so we can check them against other prefixes.
			newMap := pcommon.NewMap()
			newMap.EnsureCapacity(attributes.Len())

			for key, value := range attributes.All() {
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
			}
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

			for j := 0; j < len(names); j++ {
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
