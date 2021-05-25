// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"regexp"

	"github.com/golang/groupcache/lru"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const defaultCacheSize = 128

type stringAttributeFilter struct {
	key    string
	logger *zap.Logger
	// matcher defines the func to match the attribute values in strict string
	// or in regular expression
	matcher func(string) bool
}

type regexStrSetting struct {
	matchedAttrs *lru.Cache
	filterList   []*regexp.Regexp
}

var _ PolicyEvaluator = (*stringAttributeFilter)(nil)

// NewStringAttributeFilter creates a policy evaluator that samples all traces with
// the given attribute in the given numeric range.
func NewStringAttributeFilter(logger *zap.Logger, key string, values []string, regexMatchEnabled bool, evictSize int) PolicyEvaluator {
	// initialize regex filter rules and LRU cache for matched results
	if regexMatchEnabled {
		if evictSize <= 0 {
			evictSize = defaultCacheSize
		}
		filterList := addFilters(values)
		regexStrSetting := &regexStrSetting{
			matchedAttrs: lru.New(evictSize),
			filterList:   filterList,
		}

		return &stringAttributeFilter{
			key:    key,
			logger: logger,
			// matcher returns true if the given string matches the regex rules defined in string attribute filters
			matcher: func(toMatch string) bool {
				if v, ok := regexStrSetting.matchedAttrs.Get(toMatch); ok {
					return v.(bool)
				}

				for _, r := range regexStrSetting.filterList {
					if r.MatchString(toMatch) {
						regexStrSetting.matchedAttrs.Add(toMatch, true)
						return true
					}
				}

				regexStrSetting.matchedAttrs.Add(toMatch, false)
				return false
			},
		}
	}

	// initialize the exact value map
	valuesMap := make(map[string]struct{})
	for _, value := range values {
		if value != "" {
			valuesMap[value] = struct{}{}
		}
	}
	return &stringAttributeFilter{
		key:    key,
		logger: logger,
		// matcher returns true if the given string matches any of the string attribute filters
		matcher: func(toMatch string) bool {
			_, matched := valuesMap[toMatch]
			return matched
		},
	}
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (saf *stringAttributeFilter) OnLateArrivingSpans(Decision, []*pdata.Span) error {
	saf.logger.Debug("Triggering action for late arriving spans in string-tag filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
// The SamplingDecision is made by comparing the attribute values with the matching values,
// which might be static strings or regular expressions.
func (saf *stringAttributeFilter) Evaluate(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	saf.logger.Debug("Evaluting spans in string-tag filter")
	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()
	for _, batch := range batches {
		rspans := batch.ResourceSpans()

		for i := 0; i < rspans.Len(); i++ {
			rs := rspans.At(i)
			resource := rs.Resource()
			if v, ok := resource.Attributes().Get(saf.key); ok {
				if ok := saf.matcher(v.StringVal()); ok {
					return Sampled, nil
				}
			}

			ilss := rs.InstrumentationLibrarySpans()
			for j := 0; j < ilss.Len(); j++ {
				ils := ilss.At(j)
				for k := 0; k < ils.Spans().Len(); k++ {
					span := ils.Spans().At(k)
					if v, ok := span.Attributes().Get(saf.key); ok {
						truncableStr := v.StringVal()
						if len(truncableStr) > 0 {
							if ok := saf.matcher(v.StringVal()); ok {
								return Sampled, nil
							}
						}
					}

				}
			}
		}
	}
	return NotSampled, nil
}

// addFilters compiles all the given filters and stores them as regexes.
// All regexes are automatically anchored to enforce full string matches.
func addFilters(exprs []string) []*regexp.Regexp {
	list := make([]*regexp.Regexp, 0, len(exprs))
	for _, entry := range exprs {
		rule := regexp.MustCompile(entry)
		list = append(list, rule)
	}
	return list
}
