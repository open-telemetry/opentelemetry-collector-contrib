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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterspan"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

type filterSpanProcessor struct {
	cfg     *Config
	include filterspan.Matcher
	exclude filterspan.Matcher
	logger  *zap.Logger
}

func newFilterTracesProcessor(logger *zap.Logger, cfg *Config) (*filterSpanProcessor, error) {
	inc, err := createSpanMatcher(cfg.Spans.Include)
	if err != nil {
		return nil, err
	}

	exc, err := createSpanMatcher(cfg.Spans.Exclude)
	if err != nil {
		return nil, err
	}

	includeMatchType := ""
	var includeResourceAttributes []filterconfig.Attribute
	if cfg.Spans.Include != nil {
		includeMatchType = string(cfg.Spans.Include.MatchType)
	}

	excludeMatchType := ""
	var excludeResourceAttributes []filterconfig.Attribute
	if cfg.Metrics.Exclude != nil {
		excludeMatchType = string(cfg.Spans.Exclude.MatchType)
	}

	logger.Info(
		"Span filter configured",
		zap.String("include match_type", includeMatchType),
		zap.Any("include spans with resource attributes", includeResourceAttributes),
		zap.String("exclude match_type", excludeMatchType),
		zap.Any("exclude spans with resource attributes", excludeResourceAttributes),
	)

	return &filterSpanProcessor{
		cfg:     cfg,
		include: inc,
		exclude: exc,
		logger:  logger,
	}, nil
}

func createSpanMatcher(sp *filterconfig.MatchProperties) (filterspan.Matcher, error) {
	var matcher filterspan.Matcher
	var err error
	if sp == nil {
		panic("No Match Properties for Filter")
	}
	matcher, err = filterspan.NewMatcher(sp)
	return matcher, err
}

// processTraces filters the given spans of a traces based off the filterSpanProcessor's filters.
func (fsp *filterSpanProcessor) processTraces(_ context.Context, pdt pdata.Traces) (pdata.Traces, error) {

	for i := 0; i < pdt.ResourceSpans().Len(); i++ {
		resSpan := pdt.ResourceSpans().At(i)
		for x := 0; x < resSpan.InstrumentationLibrarySpans().Len(); x++ {
			ils := resSpan.InstrumentationLibrarySpans().At(x)
			for spanCount := 0; spanCount < ils.Spans().Len(); spanCount++ {
				ils.Spans().RemoveIf(func(span pdata.Span) bool {
					return filterspan.SkipSpan(fsp.include, fsp.exclude, span, resSpan.Resource(), ils.InstrumentationLibrary())
				})
			}
		}
		resSpan.InstrumentationLibrarySpans().RemoveIf(func(spans pdata.InstrumentationLibrarySpans) bool {
			return spans.Spans().Len() == 0
		})
	}
	if pdt.ResourceSpans().Len() == 0 {
		return pdt, processorhelper.ErrSkipProcessingData
	}
	return pdt, nil
}
