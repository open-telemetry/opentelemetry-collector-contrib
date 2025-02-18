// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type NestingProcessorConfig struct {
	Separator          string   `mapstructure:"separator"`
	Enabled            bool     `mapstructure:"enabled"`
	Include            []string `mapstructure:"include"`
	Exclude            []string `mapstructure:"exclude"`
	SquashSingleValues bool     `mapstructure:"squash_single_values"`
}

type NestingProcessor struct {
	separator          string
	enabled            bool
	allowlist          []string
	denylist           []string
	squashSingleValues bool
}

func newNestingProcessor(config *NestingProcessorConfig) *NestingProcessor {
	proc := &NestingProcessor{
		separator:          config.Separator,
		enabled:            config.Enabled,
		allowlist:          config.Include,
		denylist:           config.Exclude,
		squashSingleValues: config.SquashSingleValues,
	}

	return proc
}

func (proc *NestingProcessor) processLogs(logs plog.Logs) error {
	if !proc.enabled {
		return nil
	}

	for i := range logs.ResourceLogs().Len() {
		rl := logs.ResourceLogs().At(i)

		if err := proc.processAttributes(rl.Resource().Attributes()); err != nil {
			return err
		}

		for j := range rl.ScopeLogs().Len() {
			logsRecord := rl.ScopeLogs().At(j).LogRecords()

			for k := range logsRecord.Len() {
				if err := proc.processAttributes(logsRecord.At(k).Attributes()); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (proc *NestingProcessor) processMetrics(metrics pmetric.Metrics) error {
	if !proc.enabled {
		return nil
	}

	for i := range metrics.ResourceMetrics().Len() {
		rm := metrics.ResourceMetrics().At(i)

		if err := proc.processAttributes(rm.Resource().Attributes()); err != nil {
			return err
		}

		for j := range rm.ScopeMetrics().Len() {
			metricsSlice := rm.ScopeMetrics().At(j).Metrics()

			for k := range metricsSlice.Len() {
				if err := processMetricLevelAttributes(proc, metricsSlice.At(k)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (proc *NestingProcessor) processTraces(traces ptrace.Traces) error {
	if !proc.enabled {
		return nil
	}

	for i := range traces.ResourceSpans().Len() {
		rs := traces.ResourceSpans().At(i)

		if err := proc.processAttributes(rs.Resource().Attributes()); err != nil {
			return err
		}

		for j := range rs.ScopeSpans().Len() {
			spans := rs.ScopeSpans().At(j).Spans()

			for k := range spans.Len() {
				if err := proc.processAttributes(spans.At(k).Attributes()); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (proc *NestingProcessor) processAttributes(attributes pcommon.Map) error {
	newMap := pcommon.NewMap()

	attributes.Range(func(k string, v pcommon.Value) bool {
		// If key is not on allow list or is on deny list, skip translating it.
		if !proc.shouldTranslateKey(k) {
			v.CopyTo(newMap.PutEmpty(k))
			return true
		}

		keys := strings.Split(k, proc.separator)
		if len(keys) == 0 {
			// Split returns empty slice only if both string and separator are empty
			// set map[""] = v and return
			newVal := newMap.PutEmpty(k)
			v.CopyTo(newVal)
			return true
		}

		prevValue := pcommon.NewValueMap()
		nextMap := prevValue.Map()
		newMap.CopyTo(nextMap)

		for i := range keys {
			if prevValue.Type() != pcommon.ValueTypeMap {
				// If previous value was not a map, change it into a map.
				// The former value will be set under the key "".
				tempMap := pcommon.NewValueMap()
				prevValue.CopyTo(tempMap.Map().PutEmpty(""))
				tempMap.CopyTo(prevValue)
			}

			newValue, ok := prevValue.Map().Get(keys[i])
			if ok {
				prevValue = newValue
			} else {
				if i == len(keys)-1 {
					// If we're checking the last key, insert empty value, to which v will be copied.
					prevValue = prevValue.Map().PutEmpty(keys[i])
				} else {
					// If we're not checking the last key, put a map.
					prevValue = prevValue.Map().PutEmpty(keys[i])
					prevValue.SetEmptyMap()
				}
			}
		}

		if prevValue.Type() == pcommon.ValueTypeMap {
			// Now check the value we want to copy. If it is a map, we should merge both maps.
			// Else, just place the value under the key "".
			if v.Type() == pcommon.ValueTypeMap {
				v.Map().Range(func(k string, val pcommon.Value) bool {
					val.CopyTo(prevValue.Map().PutEmpty(k))
					return true
				})
			} else {
				v.CopyTo(prevValue.Map().PutEmpty(""))
			}
		} else {
			v.CopyTo(prevValue)
		}

		nextMap.CopyTo(newMap)
		return true
	})

	if proc.squashSingleValues {
		newMap = proc.squash(newMap)
	}

	newMap.CopyTo(attributes)

	return nil
}

// Checks if given key fulfills the following conditions:
// - has a prefix that exists in the allowlist (if it's not empty)
// - does not have a prefix that exists in the denylist
func (proc *NestingProcessor) shouldTranslateKey(k string) bool {
	if len(proc.allowlist) > 0 {
		isOk := false
		for i := range len(proc.allowlist) {
			if strings.HasPrefix(k, proc.allowlist[i]) {
				isOk = true
				break
			}
		}
		if !isOk {
			return false
		}
	}

	if len(proc.denylist) > 0 {
		for i := range len(proc.denylist) {
			if strings.HasPrefix(k, proc.denylist[i]) {
				return false
			}
		}
	}

	return true
}

// Squashes maps that have single values, eg. map {"a": {"b": {"c": "C", "d": "D"}}}}
// gets squashes into {"a.b": {"c": "C", "d": "D"}}}
func (proc *NestingProcessor) squash(attributes pcommon.Map) pcommon.Map {
	newMap := pcommon.NewValueMap()
	attributes.CopyTo(newMap.Map())
	key := proc.squashAttribute(newMap)

	if key != "" {
		retMap := pcommon.NewMap()
		newMap.Map().CopyTo(retMap.PutEmptyMap(key))
		return retMap
	}

	return newMap.Map()
}

// A function that squashes keys in a value.
// If this value contained a map with one element, it gets squished and its key gets returned.
//
// If this value contained a map with many elements, this function is called on these elements,
// and the key gets replaced if needed, "" is returned.
//
// Else, nothing happens and "" is returned.
func (proc *NestingProcessor) squashAttribute(value pcommon.Value) string {
	if value.Type() != pcommon.ValueTypeMap {
		return ""
	}

	m := value.Map()
	if m.Len() == 1 {
		// If the map contains only one key-value pair, squash it.
		key := ""
		val := pcommon.NewValueEmpty()
		// This will iterate only over one value (the only one)
		m.Range(func(k string, v pcommon.Value) bool {
			keySuffix := proc.squashAttribute(v)
			key = proc.squashKey(k, keySuffix)
			val = v
			return false
		})

		val.CopyTo(value)
		return key
	}

	// This map doesn't get squashed, but its content might have keys replaced.
	newMap := pcommon.NewMap()
	m.Range(func(k string, v pcommon.Value) bool {
		keySuffix := proc.squashAttribute(v)
		// If "" was returned, the value was not a one-element map and did not get squashed.
		if keySuffix == "" {
			v.CopyTo(newMap.PutEmpty(k))
		} else {
			v.CopyTo(newMap.PutEmpty(proc.squashKey(k, keySuffix)))
		}

		return true
	})
	newMap.CopyTo(value.Map())

	return ""
}

func (proc *NestingProcessor) squashKey(key string, keySuffix string) string {
	if keySuffix == "" {
		return key
	}
	return key + proc.separator + keySuffix
}

func (proc *NestingProcessor) isEnabled() bool {
	return proc.enabled
}

func (*NestingProcessor) ConfigPropertyName() string {
	return "nest_attributes"
}
