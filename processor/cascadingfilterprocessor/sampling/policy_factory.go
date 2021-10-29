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
	"errors"
	"regexp"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
)

type numericAttributeFilter struct {
	key                string
	minValue, maxValue int64
}

type stringAttributeFilter struct {
	key      string
	values   map[string]struct{}
	patterns []*regexp.Regexp
}

type attributeRange struct {
	minValue int64
	maxValue int64
}

type attributeFilter struct {
	key      string
	values   map[string]struct{}
	patterns []*regexp.Regexp
	ranges   []attributeRange
}

type policyEvaluator struct {
	numericAttr *numericAttributeFilter
	stringAttr  *stringAttributeFilter
	attrs       []attributeFilter

	operationRe       *regexp.Regexp
	minDuration       *time.Duration
	minNumberOfSpans  *int
	minNumberOfErrors *int

	currentSecond        int64
	maxSpansPerSecond    int32
	spansInCurrentSecond int32

	invertMatch bool

	logger *zap.Logger
}

var _ PolicyEvaluator = (*policyEvaluator)(nil)

func createNumericAttributeFilter(cfg *config.NumericAttributeCfg) *numericAttributeFilter {
	if cfg == nil {
		return nil
	}

	return &numericAttributeFilter{
		key:      cfg.Key,
		minValue: cfg.MinValue,
		maxValue: cfg.MaxValue,
	}
}

func createStringAttributeFilter(cfg *config.StringAttributeCfg) (*stringAttributeFilter, error) {
	if cfg == nil {
		return nil, nil
	}

	valuesMap := make(map[string]struct{})
	var patterns []*regexp.Regexp
	for _, value := range cfg.Values {
		if cfg.UseRegex {
			re, err := regexp.Compile(value)
			if err != nil {
				return nil, err
			}
			patterns = append(patterns, re)
		} else {
			if value != "" {
				valuesMap[value] = struct{}{}
			}
		}
	}

	return &stringAttributeFilter{
		key:      cfg.Key,
		values:   valuesMap,
		patterns: patterns,
	}, nil
}

func createAttributeFilter(cfg config.AttributeCfg) (*attributeFilter, error) {
	valuesMap := make(map[string]struct{})
	var patterns []*regexp.Regexp
	for _, value := range cfg.Values {
		if cfg.UseRegex {
			re, err := regexp.Compile(value)
			if err != nil {
				return nil, err
			}
			patterns = append(patterns, re)
		} else {
			if value != "" {
				valuesMap[value] = struct{}{}
			}
		}
	}
	var ranges []attributeRange
	for _, r := range cfg.Ranges {
		ranges = append(ranges, attributeRange{
			minValue: r.MinValue,
			maxValue: r.MaxValue,
		})
	}

	return &attributeFilter{
		key:      cfg.Key,
		values:   valuesMap,
		patterns: patterns,
		ranges:   ranges,
	}, nil
}

func createAttributesFilter(cfg []config.AttributeCfg) ([]attributeFilter, error) {
	if cfg == nil {
		return nil, nil
	}

	var filters []attributeFilter
	for _, attrCfg := range cfg {
		filter, err := createAttributeFilter(attrCfg)
		if err != nil {
			return nil, err
		}
		filters = append(filters, *filter)
	}

	return filters, nil
}

// NewProbabilisticFilter creates a policy evaluator intended for selecting samples probabilistically
func NewProbabilisticFilter(logger *zap.Logger, maxSpanRate int32) (PolicyEvaluator, error) {
	return &policyEvaluator{
		logger:               logger,
		currentSecond:        0,
		spansInCurrentSecond: 0,
		maxSpansPerSecond:    maxSpanRate,
	}, nil
}

// NewFilter creates a policy evaluator that samples all traces with the specified criteria
func NewFilter(logger *zap.Logger, cfg *config.TraceAcceptCfg) (PolicyEvaluator, error) {
	numericAttrFilter := createNumericAttributeFilter(cfg.NumericAttributeCfg)
	stringAttrFilter, err := createStringAttributeFilter(cfg.StringAttributeCfg)
	if err != nil {
		return nil, err
	}
	attrsFilter, err := createAttributesFilter(cfg.AttributeCfg)
	if err != nil {
		return nil, err
	}

	var operationRe *regexp.Regexp

	if cfg.PropertiesCfg.NamePattern != nil {
		operationRe, err = regexp.Compile(*cfg.PropertiesCfg.NamePattern)
		if err != nil {
			return nil, err
		}
	}

	if cfg.PropertiesCfg.MinDuration != nil && *cfg.PropertiesCfg.MinDuration < 0*time.Second {
		return nil, errors.New("minimum span duration must be a non-negative number")
	}

	if cfg.PropertiesCfg.MinNumberOfSpans != nil && *cfg.PropertiesCfg.MinNumberOfSpans < 1 {
		return nil, errors.New("minimum number of spans must be a positive number")
	}

	return &policyEvaluator{
		stringAttr:           stringAttrFilter,
		numericAttr:          numericAttrFilter,
		attrs:                attrsFilter,
		operationRe:          operationRe,
		minDuration:          cfg.PropertiesCfg.MinDuration,
		minNumberOfSpans:     cfg.PropertiesCfg.MinNumberOfSpans,
		minNumberOfErrors:    cfg.PropertiesCfg.MinNumberOfErrors,
		logger:               logger,
		currentSecond:        0,
		spansInCurrentSecond: 0,
		maxSpansPerSecond:    cfg.SpansPerSecond,
		invertMatch:          cfg.InvertMatch,
	}, nil
}
