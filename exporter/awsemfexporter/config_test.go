// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.uber.org/zap"
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
			id:       component.NewIDWithName(typeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "1"),
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
			id: component.NewIDWithName(typeStr, "resource_attr_to_label"),
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
			id: component.NewIDWithName(typeStr, "metric_descriptors"),
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
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
	assert.NoError(t, component.ValidateConfig(cfg))

	assert.Equal(t, 2, len(cfg.MetricDescriptors))
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
	assert.NoError(t, component.ValidateConfig(cfg))

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
	assert.Error(t, component.ValidateConfig(wrongcfg))

}

func TestTagsValidateCorrect(t *testing.T) {
	avalue := "avalue"
	cfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{"akey": &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.NoError(t, component.ValidateConfig(cfg))

}

func TestTagsValidateTooManyTags(t *testing.T) {
	m := make(map[string]*string)
	avalue := "avalue"
	for i := 0; i < 51; i++ {
		m[strconv.Itoa(i)] = &avalue
	}
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        m,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.EqualError(t, component.ValidateConfig(wrongcfg), "invalid amount of items. Please input at most 50 tags")
}

func TestTagsValidateWrongKeyRegex(t *testing.T) {
	avalue := "avalue"
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{"***": &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.EqualError(t, component.ValidateConfig(wrongcfg), "key - *** does not follow the regex pattern"+`^([\p{L}\p{Z}\p{N}_.:/=+\-@]+)$`)
}

func TestTagsValidateWrongValueRegex(t *testing.T) {
	avalue := "***"
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{"akey": &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.EqualError(t, component.ValidateConfig(wrongcfg), "value - "+avalue+" does not follow the regex pattern"+`^([\p{L}\p{Z}\p{N}_.:/=+\-@]*)$`)
}

func TestTagsValidateKeyTooShort(t *testing.T) {
	avalue := "avalue"
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{"": &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.EqualError(t, component.ValidateConfig(wrongcfg), "key -  has an invalid length. Please use keys with a length of 1 to 128 characters")
}

func TestTagsValidateKeyTooLong(t *testing.T) {
	avalue := "avalue"
	akey := ""
	for i := 0; i < 129; i++ {
		akey += "a"
	}
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{akey: &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.Error(t, component.ValidateConfig(wrongcfg), "key - "+akey+" has an invalid length. Please use keys with a length of 1 to 128 characters")
}

func TestTagsValidateValueTooShort(t *testing.T) {
	avalue := ""
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{"akey": &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.Error(t, component.ValidateConfig(wrongcfg), "value - "+avalue+" has an invalid length. Please use keys with a length of 1 to 256 characters")
}

func TestTagsValidateValueTooLong(t *testing.T) {
	avalue := ""
	for i := 0; i < 257; i++ {
		avalue += "a"
	}
	wrongcfg := &Config{
		AWSSessionSettings: awsutil.AWSSessionSettings{
			RequestTimeoutSeconds: 30,
			MaxRetries:            1,
		},
		DimensionRollupOption:       "ZeroAndSingleDimensionRollup",
		Tags:                        map[string]*string{"akey": &avalue},
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{Enabled: true},
		logger:                      zap.NewNop(),
	}
	assert.Error(t, component.ValidateConfig(wrongcfg), "value - "+avalue+" has an invalid length. Please use keys with a length of 1 to 256 characters")
}
