// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsaggregationprocessor"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"time"
)

const (
	// includeFieldName is the mapstructure field name for Include field
	includeFieldName = "include"

	// matchTypeFieldName is the mapstructure field name for matchType field
	matchTypeFieldName = "match_type"

	// actionFieldName is the mapstructure field name for Action field
	actionFieldName = "action"

	// aggregationTypeFieldName is the mapstructure field name for aggregationType field
	aggregationTypeFieldName = "aggregation_type"

	// labelFieldName is the mapstructure field name for Label field
	labelFieldName = "label"
)

// Config defines configuration for Resource processor.
type Config struct {
	// transform specifies a list of transforms on metrics with each transform focusing on one metric.
	// TODO crl: rename to aggregations
	Transforms []transform `mapstructure:"transforms"`

	Interval time.Duration `mapstructure:"interval"`
}

// transform defines the transformation applied to the specific metric
type transform struct {
	// --- SPECIFY WHICH METRIC(S) TO MATCH ---

	// MetricIncludeFilter is used to select the metric(s) to operate on.
	// REQUIRED
	MetricIncludeFilter FilterConfig `mapstructure:",squash"`

	// --- SPECIFY THE ACTION TO TAKE ON THE MATCHED METRIC(S) ---

	// Action specifies the action performed on the matched metric. Action specifies
	// if the operations (specified below) are performed on metrics in place (update),
	// on an inserted clone (insert), or on a new combined metric that includes all
	// data points from the set of matching metrics (combine).
	// REQUIRED
	Action ConfigAction `mapstructure:"action"`

	// AggregationType specifies how to aggregate.
	// REQUIRED only if Action is COMBINE.
	AggregationType aggregateutil.AggregationType `mapstructure:"aggregation_type"`

	// Operations contains a list of operations that will be performed on the resulting metric(s).
	Operations []Operation `mapstructure:"operations"`
}

type FilterConfig struct {
	// Include specifies the metric(s) to operate on.
	Include string `mapstructure:"include"`

	// MatchType determines how the Include string is matched: <strict|regexp>.
	MatchType matchType `mapstructure:"match_type"`

	// MatchLabels specifies the label set against which the metric filter will work.
	// This field is optional.
	MatchLabels map[string]string `mapstructure:"match_labels"`
}

// Operation defines the specific operation performed on the selected metrics.
type Operation struct {
	// Action specifies the action performed for this operation.
	// REQUIRED
	Action operationAction `mapstructure:"action"`

	// LabelSet is a list of labels to keep. All other labels are aggregated based on the AggregationType.
	// TODO crl: add flag for whether to inverse this set to replicate `without` concept
	LabelSet []string `mapstructure:"label_set"`

	// AggregationType specifies how to aggregate.
	AggregationType aggregateutil.AggregationType `mapstructure:"aggregation_type"`
}

// ConfigAction is the enum to capture the type of action to perform on a metric.
type ConfigAction string

const (
	// Insert adds a new metric to the batch with a new name.
	Insert ConfigAction = "insert"

	// Combine combines multiple metrics into a single metric.
	Combine ConfigAction = "combine"
)

var actions = []ConfigAction{Insert, Combine}

func (ca ConfigAction) isValid() bool {
	for _, configAction := range actions {
		if ca == configAction {
			return true
		}
	}

	return false
}

// operationAction is the enum to capture the types of actions to perform for an operation.
type operationAction string

const (
	// aggregateLabels aggregates away all labels other than the ones in Operation.LabelSet
	// by the method indicated by Operation.AggregationType.
	// Metric has to match the FilterConfig with all its data points if used with Update ConfigAction,
	// otherwise the operation will be ignored.
	aggregateLabels operationAction = "aggregate_labels"

	// aggregateLabelValues aggregates away the values in Operation.AggregatedValues
	// by the method indicated by Operation.AggregationType.
	// Metric has to match the FilterConfig with all its data points if used with Update ConfigAction,
	// otherwise the operation will be ignored.
	// TODO crl: can we eliminate this too?
	//aggregateLabelValues operationAction = "aggregate_label_values"
)

var operationActions = []operationAction{aggregateLabels}

func (oa operationAction) isValid() bool {
	for _, operationAction := range operationActions {
		if oa == operationAction {
			return true
		}
	}

	return false
}

// matchType is the enum to capture the two types of matching metric(s) that should have operations applied to them.
type matchType string

const (
	// strictMatchType is the FilterType for filtering by exact string matches.
	strictMatchType matchType = "strict"

	// regexpMatchType is the FilterType for filtering by regexp string matches.
	regexpMatchType matchType = "regexp"
)

var matchTypes = []matchType{strictMatchType, regexpMatchType}

func (mt matchType) isValid() bool {
	for _, matchType := range matchTypes {
		if mt == matchType {
			return true
		}
	}

	return false
}
