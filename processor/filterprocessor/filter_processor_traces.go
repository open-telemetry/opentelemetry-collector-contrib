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

func newFilterSpansProcessor(logger *zap.Logger, cfg *Config) (*filterSpanProcessor, error) {
	if cfg.Spans.Include == nil && cfg.Spans.Exclude == nil {
		return nil, nil
	}

	inc, err := createSpanMatcher(cfg.Spans.Include)
	if err != nil {
		return nil, err
	}
	exc, err := createSpanMatcher(cfg.Spans.Exclude)
	if err != nil {
		return nil, err
	}

	includeMatchType := ""
	if cfg.Spans.Include != nil {
		includeMatchType = string(cfg.Spans.Include.MatchType)
	}

	excludeMatchType := ""
	if cfg.Spans.Exclude != nil {
		excludeMatchType = string(cfg.Spans.Exclude.MatchType)
	}

	logger.Info(
		"Span filtering configured",
		zap.String("[Include] match_type", includeMatchType),
		zap.Any("[Include] attributes", cfg.Spans.Include.Attributes),
		zap.Any("[Include] libraries", cfg.Spans.Include.Libraries),
		zap.Any("[Include] attributes", cfg.Spans.Include.Resources),
		zap.Strings("[Include] services", cfg.Spans.Include.Services),
		zap.Strings("[Include] span_names", cfg.Spans.Include.SpanNames),
		zap.String("[Exclude] match_type", excludeMatchType),
		zap.Any("[Exclude] attributes", cfg.Spans.Exclude.Attributes),
		zap.Any("[Exclude] libraries", cfg.Spans.Exclude.Libraries),
		zap.Any("[Exclude] resources", cfg.Spans.Exclude.Resources),
		zap.Strings("[Exclude] services", cfg.Spans.Exclude.Services),
		zap.Strings("[Exclude] span_names", cfg.Spans.Exclude.SpanNames),

	)

	return &filterSpanProcessor{
		cfg:     cfg,
		include: inc,
		exclude: exc,
		logger:  logger,
	}, nil
}

func createSpanMatcher(sp *filterconfig.MatchProperties) (filterspan.Matcher, error) {
	if sp == nil {
		return nil, nil
	}
	return filterspan.NewMatcher(sp)
}

// processTraces filters the given spans of a traces based off the filterSpanProcessor's filters.
func (fsp *filterSpanProcessor) processTraces(_ context.Context, pdt pdata.Traces) (pdata.Traces, error) {
	for i := 0; i < pdt.ResourceSpans().Len(); i++ {
		resSpan := pdt.ResourceSpans().At(i)
		for x := 0; x < resSpan.InstrumentationLibrarySpans().Len(); x++ {
			ils := resSpan.InstrumentationLibrarySpans().At(x)
			ils.Spans().RemoveIf(func(span pdata.Span) bool {
				return !fsp.shouldKeepSpan(span, resSpan.Resource(), ils.InstrumentationLibrary())
			})
		}

		// Remove empty elements, that way if we delete everything we can tell
		// the pipeline to stop processing completely (ErrSkipProcessingData)
		resSpan.InstrumentationLibrarySpans().RemoveIf(func(ilsSpans pdata.InstrumentationLibrarySpans) bool {
			return ilsSpans.Spans().Len() == 0
		})
		pdt.ResourceSpans().RemoveIf(func(res pdata.ResourceSpans) bool {
			return res.InstrumentationLibrarySpans().Len() == 0
		})
	}
	if pdt.ResourceSpans().Len() == 0 {
		return pdt, processorhelper.ErrSkipProcessingData
	}
	return pdt, nil
}

func (fsp *filterSpanProcessor) shouldKeepSpan(span pdata.Span, resource pdata.Resource, library pdata.InstrumentationLibrary) bool {
	if fsp.include != nil {
		if i := fsp.include.MatchSpan(span, resource, library); !i {
			return false
		}
	}

	if fsp.exclude != nil {
		if e := fsp.exclude.MatchSpan(span, resource, library); e {
			return false
		}
	}

	return true
}
