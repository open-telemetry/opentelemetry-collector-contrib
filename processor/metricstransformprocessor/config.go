// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"

const (
	// includeFieldName is the mapstructure field name for Include field
	includeFieldName = "include"

	// matchTypeFieldName is the mapstructure field name for matchType field
	matchTypeFieldName = "match_type"

	// actionFieldName is the mapstructure field name for Action field
	actionFieldName = "action"

	// newNameFieldName is the mapstructure field name for NewName field
	newNameFieldName = "new_name"

	// groupResourceLabelsFieldName is the mapstructure field name for GroupResourceLabels field
	groupResourceLabelsFieldName = "group_resource_labels"

	// aggregationTypeFieldName is the mapstructure field name for aggregationType field
	aggregationTypeFieldName = "aggregation_type"

	// labelFieldName is the mapstructure field name for Label field
	labelFieldName = "label"

	// newLabelFieldName is the mapstructure field name for NewLabel field
	newLabelFieldName = "new_label"

	// newValueFieldName is the mapstructure field name for NewValue field
	newValueFieldName = "new_value"

	// scaleFieldName is the mapstructure field name for Scale field
	scaleFieldName = "experimental_scale"

	// submatchCaseFieldName is the mapstructure field name for submatchCase field
	submatchCaseFieldName = "submatch_case"
)

// Config defines configuration for Resource processor.
type Config struct {
	// transform specifies a list of transforms on metrics with each transform focusing on one metric.
	Transforms []transform `mapstructure:"transforms"`

	// prevent unkeyed literal initialization
	_ struct{}
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

	// --- SPECIFY HOW TO TRANSFORM THE METRIC GENERATED AS A RESULT OF APPLYING THE ABOVE ACTION ---

	// NewName specifies the name of the new metric when inserting or updating.
	// REQUIRED only if Action is INSERT.
	NewName string `mapstructure:"new_name"`

	// GroupResourceLabels specifies resource labels that will be appended to this group's new ResourceMetrics message
	// REQUIRED only if Action is GROUP
	GroupResourceLabels map[string]string `mapstructure:"group_resource_labels"`

	// AggregationType specifies how to aggregate.
	// REQUIRED only if Action is COMBINE.
	AggregationType aggregateutil.AggregationType `mapstructure:"aggregation_type"`

	// SubmatchCase specifies what case to use for label values created from regexp submatches.
	SubmatchCase submatchCase `mapstructure:"submatch_case"`

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
	MatchLabels map[string]string `mapstructure:"experimental_match_labels"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Operation defines the specific operation performed on the selected metrics.
type Operation struct {
	// Action specifies the action performed for this operation.
	// REQUIRED
	Action operationAction `mapstructure:"action"`

	// Label identifies the exact label to operate on.
	Label string `mapstructure:"label"`

	// NewLabel determines the name to rename the identified label to.
	NewLabel string `mapstructure:"new_label"`

	// LabelSet is a list of labels to keep. All other labels are aggregated based on the AggregationType.
	LabelSet []string `mapstructure:"label_set"`

	// AggregationType specifies how to aggregate.
	AggregationType aggregateutil.AggregationType `mapstructure:"aggregation_type"`

	// AggregatedValues is a list of label values to aggregate away.
	AggregatedValues []string `mapstructure:"aggregated_values"`

	// NewValue is used to set a new label value either when the operation is `AggregatedValues` or `addLabel`.
	NewValue string `mapstructure:"new_value"`

	// ValueActions is a list of renaming actions for label values.
	ValueActions []ValueAction `mapstructure:"value_actions"`

	// Scale is a scalar to multiply the values with.
	Scale float64 `mapstructure:"experimental_scale"`

	// LabelValue identifies the exact label value to operate on
	LabelValue string `mapstructure:"label_value"`
}

// ValueAction renames label values.
type ValueAction struct {
	// Value specifies the current label value.
	Value string `mapstructure:"value"`

	// NewValue specifies the label value to rename to.
	NewValue string `mapstructure:"new_value"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ConfigAction is the enum to capture the type of action to perform on a metric.
type ConfigAction string

const (
	// Insert adds a new metric to the batch with a new name.
	Insert ConfigAction = "insert"

	// Update updates an existing metric.
	Update ConfigAction = "update"

	// Combine combines multiple metrics into a single metric.
	Combine ConfigAction = "combine"

	// Group groups multiple metrics matching the predicate into multiple ResourceMetrics messages
	Group ConfigAction = "group"
)

var actions = []ConfigAction{Insert, Update, Combine, Group}

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
	// addLabel adds a new label to an existing metric.
	// Metric has to match the FilterConfig with all its data points if used with Update ConfigAction,
	// otherwise the operation will be ignored.
	addLabel operationAction = "add_label"

	// updateLabel applies name changes to label and/or label values.
	updateLabel operationAction = "update_label"

	// deleteLabelValue deletes a label value by also removing all the points associated with this label value
	// Metric has to match the FilterConfig with all its data points if used with Update ConfigAction,
	// otherwise the operation will be ignored.
	deleteLabelValue operationAction = "delete_label_value"

	// toggleScalarDataType changes the data type from int64 to double, or vice-versa
	toggleScalarDataType operationAction = "toggle_scalar_data_type"

	// scaleValue multiplies the value by a constant scalar
	scaleValue operationAction = "experimental_scale_value"

	// aggregateLabels aggregates away all labels other than the ones in Operation.LabelSet
	// by the method indicated by Operation.AggregationType.
	// Metric has to match the FilterConfig with all its data points if used with Update ConfigAction,
	// otherwise the operation will be ignored.
	aggregateLabels operationAction = "aggregate_labels"

	// aggregateLabelValues aggregates away the values in Operation.AggregatedValues
	// by the method indicated by Operation.AggregationType.
	// Metric has to match the FilterConfig with all its data points if used with Update ConfigAction,
	// otherwise the operation will be ignored.
	aggregateLabelValues operationAction = "aggregate_label_values"
)

var operationActions = []operationAction{addLabel, updateLabel, deleteLabelValue, toggleScalarDataType, scaleValue, aggregateLabels, aggregateLabelValues}

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

// submatchCase is the enum to capture the two types of case changes to apply to submatches.
type submatchCase string

const (
	// lower is the submatchCase for lower casing the submatch.
	lower submatchCase = "lower"

	// upper is the submatchCase for upper casing the submatch.
	upper submatchCase = "upper"
)

var submatchCases = []submatchCase{lower, upper}

func (sc submatchCase) isValid() bool {
	for _, submatchCase := range submatchCases {
		if sc == submatchCase {
			return true
		}
	}

	return false
}
