// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cfg := confighttp.NewDefaultClientConfig()
	cfg.Timeout = 2 * time.Second
	openshiftConfig := detectorCreateDefaultConfig()
	openshiftConfig.OpenShiftConfig = openshift.Config{
		Address: "127.0.0.1:4444",
		Token:   "some_token",
		TLSs: configtls.ClientConfig{
			Insecure: true,
		},
		ResourceAttributes: openshift.CreateDefaultConfig().ResourceAttributes,
	}

	ec2Config := detectorCreateDefaultConfig()
	ec2Config.EC2Config = ec2.Config{
		Tags:               []string{"^tag1$", "^tag2$"},
		ResourceAttributes: ec2.CreateDefaultConfig().ResourceAttributes,
		MaxAttempts:        3,
		MaxBackoff:         20 * time.Second,
	}

	systemConfig := detectorCreateDefaultConfig()
	systemConfig.SystemConfig = system.Config{
		HostnameSources:    []string{"os"},
		ResourceAttributes: system.CreateDefaultConfig().ResourceAttributes,
	}

	resourceAttributesConfig := detectorCreateDefaultConfig()
	ec2ResourceAttributesConfig := ec2.CreateDefaultConfig()
	ec2ResourceAttributesConfig.ResourceAttributes.HostName.Enabled = false
	ec2ResourceAttributesConfig.ResourceAttributes.HostID.Enabled = false
	ec2ResourceAttributesConfig.ResourceAttributes.HostType.Enabled = false
	systemResourceAttributesConfig := system.CreateDefaultConfig()
	systemResourceAttributesConfig.ResourceAttributes.OsType.Enabled = false
	resourceAttributesConfig.EC2Config = ec2ResourceAttributesConfig
	resourceAttributesConfig.SystemConfig = systemResourceAttributesConfig

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "openshift"),
			expected: &Config{
				Detectors:      []string{"openshift"},
				DetectorConfig: openshiftConfig,
				ClientConfig:   cfg,
				Override:       false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "gcp"),
			expected: &Config{
				Detectors:      []string{"env", "gcp"},
				ClientConfig:   cfg,
				Override:       false,
				DetectorConfig: detectorCreateDefaultConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "ec2"),
			expected: &Config{
				Detectors:      []string{"env", "ec2"},
				DetectorConfig: ec2Config,
				ClientConfig:   cfg,
				Override:       false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "system"),
			expected: &Config{
				Detectors:      []string{"env", "system"},
				DetectorConfig: systemConfig,
				ClientConfig:   cfg,
				Override:       false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "heroku"),
			expected: &Config{
				Detectors:      []string{"env", "heroku"},
				ClientConfig:   cfg,
				Override:       false,
				DetectorConfig: detectorCreateDefaultConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "lambda"),
			expected: &Config{
				Detectors:      []string{"env", "lambda"},
				ClientConfig:   cfg,
				Override:       false,
				DetectorConfig: detectorCreateDefaultConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "resourceattributes"),
			expected: &Config{
				Detectors:      []string{"system", "ec2"},
				ClientConfig:   cfg,
				Override:       false,
				DetectorConfig: resourceAttributesConfig,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "refresh"),
			expected: &Config{
				Detectors:       []string{"system"},
				ClientConfig:    cfg,
				Override:        false,
				DetectorConfig:  detectorCreateDefaultConfig(),
				RefreshInterval: 5 * time.Second,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid"),
			errorMessage: "hostname_sources contains invalid value: \"invalid_source\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.EqualExportedValues(t, *tt.expected.(*Config), *cfg.(*Config))
		})
	}
}

func TestGetConfigFromType(t *testing.T) {
	herokuDetectorConfig := DetectorConfig{HerokuConfig: heroku.CreateDefaultConfig()}
	lambdaDetectorConfig := DetectorConfig{LambdaConfig: lambda.CreateDefaultConfig()}
	ec2DetectorConfig := DetectorConfig{
		EC2Config: ec2.Config{
			Tags: []string{"tag1", "tag2"},
		},
	}
	tests := []struct {
		name                string
		detectorType        internal.DetectorType
		inputDetectorConfig DetectorConfig
		expectedConfig      internal.DetectorConfig
	}{
		{
			name:                "Get EC2 Config",
			detectorType:        ec2.TypeStr,
			inputDetectorConfig: ec2DetectorConfig,
			expectedConfig:      ec2DetectorConfig.EC2Config,
		},
		{
			name:                "Get Nil Config",
			detectorType:        internal.DetectorType("invalid input"),
			inputDetectorConfig: ec2DetectorConfig,
			expectedConfig:      nil,
		},
		{
			name:         "Get System Config",
			detectorType: system.TypeStr,
			inputDetectorConfig: DetectorConfig{
				SystemConfig: system.Config{
					HostnameSources: []string{"os"},
				},
			},
			expectedConfig: system.Config{
				HostnameSources: []string{"os"},
			},
		},
		{
			name:                "Get Heroku Config",
			detectorType:        heroku.TypeStr,
			inputDetectorConfig: herokuDetectorConfig,
			expectedConfig:      herokuDetectorConfig.HerokuConfig,
		},
		{
			name:                "Get AWS Lambda Config",
			detectorType:        lambda.TypeStr,
			inputDetectorConfig: lambdaDetectorConfig,
			expectedConfig:      lambdaDetectorConfig.LambdaConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.inputDetectorConfig.GetConfigFromType(tt.detectorType)
			assert.Equal(t, tt.expectedConfig, output)
		})
	}
}

// TestGetConfigFromType_AllDetectors tests GetConfigFromType for all detector types
// to ensure complete coverage of the switch statement
func TestGetConfigFromType_AllDetectors(t *testing.T) {
	defaultConfig := detectorCreateDefaultConfig()

	tests := []struct {
		name         string
		detectorType internal.DetectorType
	}{
		{"ECS", "ecs"},
		{"EKS", "eks"},
		{"ElasticBeanstalk", "elastic_beanstalk"},
		{"Azure", "azure"},
		{"AKS", "aks"},
		{"Consul", "consul"},
		{"DigitalOcean", "digitalocean"},
		{"Docker", "docker"},
		{"GCP", "gcp"},
		{"Hetzner", "hetzner"},
		{"OpenShift", "openshift"},
		{"Nova", "nova"},
		{"OracleCloud", "oraclecloud"},
		{"K8sNode", "k8snode"},
		{"Kubeadm", "kubeadm"},
		{"Akamai", "akamai"},
		{"Scaleway", "scaleway"},
		{"Upcloud", "upcloud"},
		{"Vultr", "vultr"},
		{"AlibabaECS", "alibaba_ecs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := defaultConfig.GetConfigFromType(tt.detectorType)
			assert.NotNil(t, config, "config should not be nil for %s detector", tt.detectorType)
		})
	}
}

// TestDetectorCreateDefaultConfig tests that detectorCreateDefaultConfig creates
// valid default configurations for all detectors
func TestDetectorCreateDefaultConfig(t *testing.T) {
	config := detectorCreateDefaultConfig()

	// Verify all detector configs are initialized
	assert.NotNil(t, config.EC2Config)
	assert.NotNil(t, config.ECSConfig)
	assert.NotNil(t, config.EKSConfig)
	assert.NotNil(t, config.ElasticbeanstalkConfig)
	assert.NotNil(t, config.LambdaConfig)
	assert.NotNil(t, config.AzureConfig)
	assert.NotNil(t, config.AksConfig)
	assert.NotNil(t, config.ConsulConfig)
	assert.NotNil(t, config.DigitalOceanConfig)
	assert.NotNil(t, config.DockerConfig)
	assert.NotNil(t, config.GcpConfig)
	assert.NotNil(t, config.HerokuConfig)
	assert.NotNil(t, config.HetznerConfig)
	assert.NotNil(t, config.SystemConfig)
	assert.NotNil(t, config.OpenShiftConfig)
	assert.NotNil(t, config.OpenStackNovaConfig)
	assert.NotNil(t, config.OracleCloudConfig)
	assert.NotNil(t, config.K8SNodeConfig)
	assert.NotNil(t, config.KubeadmConfig)
	assert.NotNil(t, config.AkamaiConfig)
	assert.NotNil(t, config.ScalewayConfig)
	assert.NotNil(t, config.UpcloudConfig)
	assert.NotNil(t, config.VultrConfig)
	assert.NotNil(t, config.AlibabaECSConfig)
}
