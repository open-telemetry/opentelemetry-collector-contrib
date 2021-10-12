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

package filterprocessor

import (
	"context"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

type SpanEventsMatchType int

const (
	NameMatch      SpanEventsMatchType = iota
	AttributeMatch SpanEventsMatchType = iota
)

type filterSpanEventProcessor struct {
	cfg                *Config
	nameMatcherInclude *spanEventNameMatcher
	nameMatcherExclude *spanEventNameMatcher
	excludeAttributes  filtermatcher.AttributesMatcher
	includeAttributes  filtermatcher.AttributesMatcher
	logger             *zap.Logger
}

func newFilterSpanEventsProcessor(logger *zap.Logger, cfg *Config) (*filterSpanEventProcessor, error) {

	includeAttributes, err := createSpanEventsMatcher(cfg.SpanEvents.Include, AttributeMatch)
	if err != nil {
		logger.Error(
			"filterSpanEvent: Error creating include span events attributes matcher", zap.Error(err),
		)
		return nil, err
	}

	excludeAttributes, err := createSpanEventsMatcher(cfg.SpanEvents.Exclude, AttributeMatch)
	if err != nil {
		logger.Error(
			"filterSpanEvent: Error creating exclude span events attributes matcher", zap.Error(err),
		)
		return nil, err
	}

	nameMatcherInclude, err := newSpanEventNameMatcher(cfg.SpanEvents.Include)
	if err != nil {
		logger.Error(
			"filterSpanEvent: Error creating include span events name matcher", zap.Error(err),
		)
		return nil, err
	}

	nameMatcherExclude, err := newSpanEventNameMatcher(cfg.SpanEvents.Exclude)
	if err != nil {
		logger.Error(
			"filterSpanEvent: Error creating exclude span events name matcher", zap.Error(err),
		)
		return nil, err
	}

	return &filterSpanEventProcessor{
		cfg:                cfg,
		nameMatcherInclude: nameMatcherInclude,
		nameMatcherExclude: nameMatcherExclude,
		includeAttributes:  includeAttributes,
		excludeAttributes:  excludeAttributes,
		logger:             logger,
	}, nil
}

func createSpanEventsMatcher(sp *SpanEventMatchProperties, matchLevel SpanEventsMatchType) (filtermatcher.AttributesMatcher, error) {
	// Nothing specified in configuration
	if sp == nil {
		return nil, nil
	}
	var attributeMatcher filtermatcher.AttributesMatcher
	attributeMatcher, err := filtermatcher.NewAttributesMatcher(
		filterset.Config{
			MatchType: filterset.MatchType(sp.SpanEventMatchType),
		},
		getFilterConfigForSpanEventMatchLevel(sp, matchLevel),
	)
	if err != nil {
		return attributeMatcher, err
	}
	return attributeMatcher, nil
}

func getFilterConfigForSpanEventMatchLevel(sp *SpanEventMatchProperties, m SpanEventsMatchType) []filterconfig.Attribute {
	switch m {
	case AttributeMatch:
		return sp.EventAttributes
	default:
		return nil
	}
}

func (fsep *filterSpanEventProcessor) ProcessSpanEvents(ctx context.Context, traces pdata.Traces) (pdata.Traces, error) {
	rSpansSlice := traces.ResourceSpans()

	for i := 0; i < rSpansSlice.Len(); i++ {
		ilSpans := rSpansSlice.At(i).InstrumentationLibrarySpans()

		for j := 0; j < ilSpans.Len(); j++ {
			spanSlice := ilSpans.At(j).Spans()

			for k := 0; k < spanSlice.Len(); k++ {
				eventsSlice := spanSlice.At(k).Events()
				fsep.filterByEventAttributes(eventsSlice)
			}
		}
	}

	return traces, nil
}

func (fsep *filterSpanEventProcessor) filterByEventAttributes(events pdata.SpanEventSlice) {
	events.RemoveIf(func(se pdata.SpanEvent) bool {
		return fsep.shouldSkipEvent(se)
	})
}

// shouldSkipEvent determines if a span event should be processed.
// True is returned when a span event should be skipped.
// False is returned when a span event should not be skipped.
// The logic determining if a span event should be skipped is set in the
// attribute configuration.
func (fsep *filterSpanEventProcessor) shouldSkipEvent(se pdata.SpanEvent) bool {
	if fsep.includeAttributes != nil {

		matches := fsep.includeAttributes.Match(se.Attributes())
		if !matches {
			return true
		}
	}

	if fsep.nameMatcherInclude != nil {
		matches, err := fsep.nameMatcherInclude.MatchSpanEventName(se)
		if err != nil {
			fsep.logger.Error(
				"filterSpanEvent: Error matching include names", zap.Error(err),
			)
		}
		if !matches {
			return true
		}
	}

	if fsep.excludeAttributes != nil {
		matches := fsep.excludeAttributes.Match(se.Attributes())
		if matches {
			return true
		}
	}

	if fsep.nameMatcherExclude != nil {
		matches, err := fsep.nameMatcherExclude.MatchSpanEventName(se)
		if err != nil {
			fsep.logger.Error(
				"filterSpanEvent: Error matching exclude names", zap.Error(err),
			)
		}
		if matches {
			return true
		}
	}

	return false
}

// spanEventNameMatcher matches metrics by metric properties against prespecified values for each property.
type spanEventNameMatcher struct {
	nameFilters filterset.FilterSet
}

func newSpanEventNameMatcher(config *SpanEventMatchProperties) (*spanEventNameMatcher, error) {
	if config == nil || len(config.EventNames) == 0 {
		return nil, nil
	}

	nameFS, err := filterset.CreateFilterSet(
		config.EventNames,
		&filterset.Config{
			MatchType:    filterset.MatchType(config.SpanEventMatchType),
			RegexpConfig: config.RegexpConfig,
		},
	)
	if err != nil {
		return nil, err
	}
	return &spanEventNameMatcher{
		nameFilters: nameFS,
	}, nil
}

// MatchSpanEventName matches a span event using the span event properties configured on the spanEventNameMatcher.
// A span event only matches if every span event property configured on the spanEventNameMatcher is a match.
func (m *spanEventNameMatcher) MatchSpanEventName(spanEvent pdata.SpanEvent) (bool, error) {
	return m.nameFilters.Matches(spanEvent.Name()), nil
}
