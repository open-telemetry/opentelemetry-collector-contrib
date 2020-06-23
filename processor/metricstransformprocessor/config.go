// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricstransformprocessor

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines configuration for Resource processor.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	// MetricName is used to select the metric to operate on.
	// REQUIRED
	MetricName string `mapstructure:"metric_name"`

	// Action specifies the action performed on the matched metric
	// REQUIRED
	Action ConfigAction `mapstructure:"action"`

	// NewName is used to rename metrics.
	// REQUIRED only if Action is INSERT
	NewName string `mapstructure:"new_name"`

	// Operations contain a list of operations that will be performed on the selected metrics.
	Operations []Operation `mapstructure:"operations"`
}

// Operation defines the specific operation performed on the selected metrics
type Operation struct {
	// Action specifies the action performed for this operation
	// REQUIRED
	Action OperationAction `mapstructure:"action"`

	// Label identifies the exact label to operate on
	Label string `mapstructure:"label"`

	// NewLabel determines the name to rename the identified label to
	NewLabel string `mapstructure:"new_label"`

	// LabelSet is a list of labels to keep. All other labels are aggregated based on the AggregationType
	LabelSet []string `mapstructure:"label_set"`

	// AggregationType specifies how to aggregate
	AggregationType AggregationType `mapstructure:"aggregation_type"`

	// AggregatedValues is a list of label values to aggregate away
	AggregatedValues []string `mapstructure:"aggregated_values"`

	// NewValue indicates what is the value called when the AggregatedValues are aggregated into one
	NewValue string `mapstructure:"new_value"`

	// ValueActions is a list of renaming actions for label values
	ValueActions []ValueAction `mapstructure:"value_actions"`
}

// ValueAction renames label values
type ValueAction struct {
	// Value specifies the current label value
	Value string `mapstructure:"value"`

	// NewValue specifies the label value to rename to
	NewValue string `mapstructure:"new_value"`
}

// ConfigAction is the enum to capture the two types of actions to perform on a metric
type ConfigAction string

// OperationAction is the enum to capture the thress types of actions to perform for an operation
type OperationAction string

// AggregationType os the enum to capture the three types of aggregation for the aggregation operation
type AggregationType string

const (
	// Insert adds a new metric to the batch with a new name
	Insert ConfigAction = "insert"

	// Update updates an existing metric
	Update ConfigAction = "update"

	// UpdateLabel applies name changes to label or label values
	UpdateLabel OperationAction = "update_label"

	// AggregateLabels aggregates away all labels other than the ones in Operation.LabelSet
	// by the method indicated by Operation.AggregationType
	AggregateLabels OperationAction = "aggregate_labels"

	// AggregateLabelValues aggregates away the values in Operation.AggregatedValues
	// by the method indicated by Operation.AggregationType
	AggregateLabelValues OperationAction = "aggregate_label_values"

	// Average indicates taking the average of the aggregated data
	Average AggregationType = "average"

	// Max indicates taking the max of the aggregated data
	Max AggregationType = "max"

	// Sum indicates taking the sum of the aggregated data
	Sum AggregationType = "sum"
)
