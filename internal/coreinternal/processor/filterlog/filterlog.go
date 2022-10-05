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

package filterlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterlog"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

// Matcher is an interface that allows matching a log record against a
// configuration of a match.
// TODO: Modify Matcher to invoke both the include and exclude properties so
//
//	calling processors will always have the same logic.
type Matcher interface {
	MatchLogRecord(lr plog.LogRecord, resource pcommon.Resource, library pcommon.InstrumentationScope) bool
}

// propertiesMatcher allows matching a log record against various log record properties.
type propertiesMatcher struct {
	filtermatcher.PropertiesMatcher

	// log bodies to compare to.
	bodyFilters filterset.FilterSet

	// log severity texts to compare to
	severityTextFilters filterset.FilterSet

	// matcher for severity number
	severityNumberMatcher Matcher
}

// NewMatcher creates a LogRecord Matcher that matches based on the given MatchProperties.
func NewMatcher(mp *filterconfig.MatchProperties) (Matcher, error) {
	if mp == nil {
		return nil, nil
	}

	if err := mp.ValidateForLogs(); err != nil {
		return nil, err
	}

	rm, err := filtermatcher.NewMatcher(mp)
	if err != nil {
		return nil, err
	}

	var bodyFS filterset.FilterSet
	if len(mp.LogBodies) > 0 {
		bodyFS, err = filterset.CreateFilterSet(mp.LogBodies, &mp.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating log record body filters: %w", err)
		}
	}
	var severitytextFS filterset.FilterSet
	if len(mp.LogSeverityTexts) > 0 {
		severitytextFS, err = filterset.CreateFilterSet(mp.LogSeverityTexts, &mp.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating log record severity text filters: %w", err)
		}
	}

	var severityNumberMatcher Matcher
	if mp.LogSeverityNumber != nil {
		severityNumberMatcher = newSeverityNumberMatcher(mp.LogSeverityNumber.Min, mp.LogSeverityNumber.MatchUndefined)
	}

	return &propertiesMatcher{
		PropertiesMatcher:     rm,
		bodyFilters:           bodyFS,
		severityTextFilters:   severitytextFS,
		severityNumberMatcher: severityNumberMatcher,
	}, nil
}

// MatchLogRecord matches a log record to a set of properties.
// There are 3 sets of properties to match against.
// The log record names are matched, if specified.
// The log record bodies are matched, if specified.
// The attributes are then checked, if specified.
// At least one of log record names or attributes must be specified. It is
// supported to have more than one of these specified, and all specified must
// evaluate to true for a match to occur.
func (mp *propertiesMatcher) MatchLogRecord(lr plog.LogRecord, resource pcommon.Resource, library pcommon.InstrumentationScope) bool {
	if lr.Body().Type() == pcommon.ValueTypeStr && mp.bodyFilters != nil && !mp.bodyFilters.Matches(lr.Body().Str()) {
		return false
	}
	if mp.severityTextFilters != nil && !mp.severityTextFilters.Matches(lr.SeverityText()) {
		return false
	}
	if mp.severityNumberMatcher != nil && !mp.severityNumberMatcher.MatchLogRecord(lr, resource, library) {
		return false
	}

	return mp.PropertiesMatcher.Match(lr.Attributes(), resource, library)
}
