// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// eMFSupportedUnits contains the unit collection supported by CloudWatch backend service.
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
var eMFSupportedUnits = newEMFSupportedUnits()

// Config defines configuration for AWS EMF exporter.
type Config struct {
	// AWSSessionSettings contains the common configuration options
	// for creating AWS session to communicate with backend
	awsutil.AWSSessionSettings `mapstructure:",squash"`
	// LogGroupName is the name of CloudWatch log group which defines group of log streams
	// that share the same retention, monitoring, and access control settings.
	LogGroupName string `mapstructure:"log_group_name"`
	// LogStreamName is the name of CloudWatch log stream which is a sequence of log events
	// that share the same source.
	LogStreamName string `mapstructure:"log_stream_name"`
	// Namespace is a container for CloudWatch metrics.
	// Metrics in different namespaces are isolated from each other.
	Namespace string `mapstructure:"namespace"`
	// RetainInitialValueOfDeltaMetric is the flag to signal that the initial value of a metric is a valid datapoint.
	// The default behavior is that the first value occurrence of a metric is set as the baseline for the calculation of
	// the delta to the next occurrence. With this flag set to true the exporter will instead use this first value as the
	// initial delta value. This is especially useful when handling low frequency metrics.
	RetainInitialValueOfDeltaMetric bool `mapstructure:"retain_initial_value_of_delta_metric"`
	// DimensionRollupOption is the option for metrics dimension rollup. Three options are available, default option is "ZeroAndSingleDimensionRollup".
	// "ZeroAndSingleDimensionRollup" - Enable both zero dimension rollup and single dimension rollup
	// "SingleDimensionRollupOnly" - Enable single dimension rollup
	// "NoDimensionRollup" - No dimension rollup (only keep original metrics which contain all dimensions)
	DimensionRollupOption string `mapstructure:"dimension_rollup_option"`

	// LogRetention is the option to set the log retention policy for the CloudWatch Log Group. Defaults to Never Expire if not specified or set to 0
	// Possible values are 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 2192, 2557, 2922, 3288, or 3653
	LogRetention int32 `mapstructure:"log_retention"`

	// Tags is the option to set tags for the CloudWatch Log Group.  If specified, please add at most 50 tags.  Input is a string to string map like so: { 'key': 'value' }
	// Keys must be between 1-128 characters and follow the regex pattern: ^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$
	// Values must be between 1-256 characters and follow the regex pattern: ^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$
	Tags map[string]string `mapstructure:"tags"`

	// ParseJSONEncodedAttributeValues is an array of attribute keys whose corresponding values are JSON-encoded as strings.
	// Those strings will be decoded to its original json structure.
	ParseJSONEncodedAttributeValues []string `mapstructure:"parse_json_encoded_attr_values"`

	// MetricDeclarations is the list of rules to be used to set dimensions for exported metrics.
	MetricDeclarations []*MetricDeclaration `mapstructure:"metric_declarations"`

	// MetricDescriptors is the list of override metric descriptors that are sent to the CloudWatch
	MetricDescriptors []MetricDescriptor `mapstructure:"metric_descriptors"`

	// OutputDestination is an option to specify the EMFExporter output. Default option is "cloudwatch"
	// "cloudwatch" - direct the exporter output to CloudWatch backend
	// "stdout" - direct the exporter output to stdout
	// TODO: we can support directing output to a file (in the future) while customer specifies a file path here.
	OutputDestination string `mapstructure:"output_destination"`

	// EKSFargateContainerInsightsEnabled is an option to reformat certin metric labels so that they take the form of a high level object
	// The end result will make the labels look like those coming out of ECS and be more easily injected into cloudwatch
	// Note that at the moment in order to use this feature the value "kubernetes" must also be added to the ParseJSONEncodedAttributeValues array in order to be used
	EKSFargateContainerInsightsEnabled bool `mapstructure:"eks_fargate_container_insights_enabled"`

	// ResourceToTelemetrySettings is an option for converting resource attributes to telemetry attributes.
	// "Enabled" - A boolean field to enable/disable this option. Default is `false`.
	// If enabled, all the resource attributes will be converted to metric labels by default.
	ResourceToTelemetrySettings resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// DetailedMetrics is an option for retaining detailed datapoint values in exported metrics (e.g instead of exporting a quantile as a statistical value,
	// preserve the quantile's population)
	DetailedMetrics bool `mapstructure:"detailed_metrics"`

	// Version is an option for sending metrics to CloudWatchLogs with Embedded Metric Format in selected version  (with "_aws")
	// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Specification.html#CloudWatch_Embedded_Metric_Format_Specification_structure
	// Otherwise, sending metrics as Embedded Metric Format version 0 (without "_aws")
	Version string `mapstructure:"version"`

	// logger is the Logger used for writing error/warning logs
	logger *zap.Logger
}

type MetricDescriptor struct {
	// MetricName is the name of the metric
	MetricName string `mapstructure:"metric_name"`
	// Unit defines the override value of metric descriptor `unit`
	Unit string `mapstructure:"unit"`
	// Overwrite set to true means the existing metric descriptor will be overwritten or a new metric descriptor will be created; false means
	// the descriptor will only be configured if empty.
	Overwrite bool `mapstructure:"overwrite"`
}

var _ component.Config = (*Config)(nil)

// Validate filters out invalid metricDeclarations and metricDescriptors
func (config *Config) Validate() error {
	var validDeclarations []*MetricDeclaration
	for _, declaration := range config.MetricDeclarations {
		err := declaration.init(config.logger)
		if err != nil {
			config.logger.Warn("Dropped metric declaration.", zap.Error(err))
		} else {
			validDeclarations = append(validDeclarations, declaration)
		}
	}
	config.MetricDeclarations = validDeclarations

	var validDescriptors []MetricDescriptor
	for _, descriptor := range config.MetricDescriptors {
		if descriptor.MetricName == "" {
			continue
		}
		if _, ok := eMFSupportedUnits[descriptor.Unit]; ok {
			validDescriptors = append(validDescriptors, descriptor)
		} else {
			config.logger.Warn("Dropped unsupported metric descriptor.", zap.String("unit", descriptor.Unit))
		}
	}
	config.MetricDescriptors = validDescriptors

	if retErr := cwlogs.ValidateRetentionValue(config.LogRetention); retErr != nil {
		return retErr
	}

	return cwlogs.ValidateTagsInput(config.Tags)
}

func (config *Config) isAppSignalsEnabled() bool {
	if config.LogGroupName == "" || config.Namespace == "" {
		return false
	}

	if config.Namespace == appSignalsMetricNamespace && strings.HasPrefix(config.LogGroupName, appSignalsLogGroupNamePrefix) {
		return true
	}

	return false
}

func newEMFSupportedUnits() map[string]any {
	unitIndexer := map[string]any{}
	for _, unit := range []string{
		"Seconds", "Microseconds", "Milliseconds", "Bytes", "Kilobytes", "Megabytes",
		"Gigabytes", "Terabytes", "Bits", "Kilobits", "Megabits", "Gigabits", "Terabits",
		"Percent", "Count", "Bytes/Second", "Kilobytes/Second", "Megabytes/Second",
		"Gigabytes/Second", "Terabytes/Second", "Bits/Second", "Kilobits/Second",
		"Megabits/Second", "Gigabits/Second", "Terabits/Second", "Count/Second", "None",
	} {
		unitIndexer[unit] = nil
	}
	return unitIndexer
}
