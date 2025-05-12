// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				AWSSessionSettings: awsutil.AWSSessionSettings{
					NumberOfWorkers:       8,
					Endpoint:              "",
					RequestTimeoutSeconds: 30,
					MaxRetries:            2,
					NoVerifySSL:           false,
					ProxyAddress:          "",
					Region:                "us-west-2",
					RoleARN:               "arn:aws:iam::123456789:role/monitoring-EKS-NodeInstanceRole",
				},
				LogGroupName:          "",
				LogStreamName:         "",
				DimensionRollupOption: "ZeroAndSingleDimensionRollup",
				OutputDestination:     "cloudwatch",
				Version:               "1",
				logger:                zap.NewNop(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "resource_attr_to_label"),
			expected: &Config{
				AWSSessionSettings: awsutil.AWSSessionSettings{
					NumberOfWorkers:       8,
					Endpoint:              "",
					RequestTimeoutSeconds: 30,
					MaxRetries:            2,
					NoVerifySSL:           false,
					ProxyAddress:          "",
					Region:                "",
					RoleARN:               "",
				},
				LogGroupName:                "",
				LogStreamName:               "",
				DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
				OutputDestination:           "cloudwatch",
				Version:                     "1",
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
				logger:                      zap.NewNop(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metric_descriptors"),
			expected: &Config{
				AWSSessionSettings: awsutil.AWSSessionSettings{
					NumberOfWorkers:       8,
					Endpoint:              "",
					RequestTimeoutSeconds: 30,
					MaxRetries:            2,
					NoVerifySSL:           false,
					ProxyAddress:          "",
					Region:                "",
					RoleARN:               "",
				},
				LogGroupName:          "",
				LogStreamName:         "",
				DimensionRollupOption: "ZeroAndSingleDimensionRollup",
				OutputDestination:     "cloudwatch",
				Version:               "1",
				MetricDescriptors: []MetricDescriptor{{
					MetricName: "memcached_current_items",
					Unit:       "Count",
					Overwrite:  true,
				}},
				logger: zap.NewNop(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	incorrectDescriptor := []MetricDescriptor{
		{MetricName: ""},
		{Unit: "Count", MetricName: "apiserver_total", Overwrite: true},
		{Unit: "INVALID", MetricName: "404"},
		{Unit: "Megabytes", MetricName: "memory_usage"},
	}
	cfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		MetricDescriptors:           incorrectDescriptor,
		logger:                      zap.NewNop(),
	}
	assert.NoError(t, xconfmap.Validate(cfg))

	assert.Len(t, cfg.MetricDescriptors, 2)
	assert.Equal(t, []MetricDescriptor{
		{Unit: "Count", MetricName: "apiserver_total", Overwrite: true},
		{Unit: "Megabytes", MetricName: "memory_usage"},
	}, cfg.MetricDescriptors)
}

func TestRetentionValidateCorrect(t *testing.T) {
	cfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		LogRetention:                365,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.NoError(t, xconfmap.Validate(cfg))
}

func TestRetentionValidateWrong(t *testing.T) {
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		LogRetention:                366,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.Error(t, xconfmap.Validate(wrongcfg))
}

func TestValidateTags(t *testing.T) {
	// Create *string values for tags inputs
	basicValue := "avalue"
	wrongRegexValue := "***"
	emptyValue := ""
	tooLongValue := strings.Repeat("a", 257)

	// Create a map with no items and then one with too many items for testing
	bigMap := make(map[string]string)
	for i := 0; i < 51; i++ {
		bigMap[strconv.Itoa(i)] = basicValue
	}

	tests := []struct {
		id           component.ID
		tags         map[string]string
		errorMessage string
	}{
		{
			id:   component.NewIDWithName(metadata.Type, "validate-correct"),
			tags: map[string]string{"basicKey": basicValue},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "too-little-tags"),
			tags:         make(map[string]string),
			errorMessage: "invalid amount of items. Please input at least 1 tag or remove the tag field",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "too-many-tags"),
			tags:         bigMap,
			errorMessage: "invalid amount of items. Please input at most 50 tags",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "wrong-key-regex"),
			tags:         map[string]string{"***": basicValue},
			errorMessage: "key - *** does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "wrong-value-regex"),
			tags:         map[string]string{"basicKey": wrongRegexValue},
			errorMessage: "value - " + wrongRegexValue + " does not follow the regex pattern" + `^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "key-too-short"),
			tags:         map[string]string{"": basicValue},
			errorMessage: "key -  has an invalid length. Please use keys with a length of 1 to 128 characters",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "key-too-long"),
			tags:         map[string]string{strings.Repeat("a", 129): basicValue},
			errorMessage: "key - " + strings.Repeat("a", 129) + " has an invalid length. Please use keys with a length of 1 to 128 characters",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "value-too-short"),
			tags:         map[string]string{"basicKey": emptyValue},
			errorMessage: "value - " + emptyValue + " has an invalid length. Please use values with a length of 1 to 256 characters",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "value-too-long"),
			tags:         map[string]string{"basicKey": tooLongValue},
			errorMessage: "value - " + tooLongValue + " has an invalid length. Please use values with a length of 1 to 256 characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := &Config{
				AWSSessionSettings: awsutil.AWSSessionSettings{
					RequestTimeoutSeconds: 30,
					MaxRetries:            1,
				},
				DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
				Tags:                        tt.tags,
				ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
				logger:                      zap.NewNop(),
			}
			if tt.errorMessage != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
		})
	}
}

func TestNoDimensionRollupFeatureGate(t *testing.T) {
	err := featuregate.GlobalRegistry().Set("awsemf.nodimrollupdefault", true)
	require.NoError(t, err)
	cfg := createDefaultConfig()

	assert.Equal(t, "NoDimensionRollup", cfg.(*Config).DimensionRollupOption)
	_ = featuregate.GlobalRegistry().Set("awsemf.nodimrollupdefault", false)
}

func TestIsApplicationSignalsEnabled(t *testing.T) {
	tests := []struct {
		name            string
		metricNameSpace string
		logGroupName    string
		expectedResult  bool
	}{
		{
			"validApplicationSignalsEMF",
			"ApplicationSignals",
			"/aws/application-signals/data",
			true,
		},
		{
			"invalidApplicationSignalsLogsGroup",
			"ApplicationSignals",
			"/nonaws/application-signals/eks",
			false,
		},
		{
			"invalidApplicationSignalsMetricNamespace",
			"NonApplicationSignals",
			"/aws/application-signals/data",
			false,
		},
		{
			"invalidApplicationSignalsEMF",
			"NonApplicationSignals",
			"/nonaws/application-signals/eks",
			false,
		},
		{
			"defaultConfig",
			"",
			"",
			false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			if len(tc.metricNameSpace) > 0 {
				cfg.Namespace = tc.metricNameSpace
			}
			if len(tc.logGroupName) > 0 {
				cfg.LogGroupName = tc.logGroupName
			}

			assert.Equal(t, tc.expectedResult, cfg.isAppSignalsEnabled())
		})
	}
}
