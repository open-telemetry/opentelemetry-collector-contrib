// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import (
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/eks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"
)

// Config defines configuration for Resource processor.
type Config struct {

	// Detectors is an ordered list of named detectors that should be
	// run to attempt to detect resource information.
	Detectors []string `mapstructure:"detectors"`
	// Override indicates whether any existing resource attributes
	// should be overridden or preserved. Defaults to true.
	Override bool `mapstructure:"override"`
	// DetectorConfig is a list of settings specific to all detectors
	DetectorConfig DetectorConfig `mapstructure:",squash"`
	// HTTP client settings for the detector
	// Timeout default is 5s
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	// Attributes is an allowlist of attributes to add.
	// If a supplied attribute is not a valid attribute of a supplied detector it will be ignored.
	// Deprecated: Please use detector's resource_attributes config instead
	Attributes []string `mapstructure:"attributes"`
}

// DetectorConfig contains user-specified configurations unique to all individual detectors
type DetectorConfig struct {
	// EC2Config contains user-specified configurations for the EC2 detector
	EC2Config ec2.Config `mapstructure:"ec2"`

	// ECSConfig contains user-specified configurations for the ECS detector
	ECSConfig ecs.Config `mapstructure:"ecs"`

	// EKSConfig contains user-specified configurations for the EKS detector
	EKSConfig eks.Config `mapstructure:"eks"`

	// Elasticbeanstalk contains user-specified configurations for the elasticbeanstalk detector
	ElasticbeanstalkConfig elasticbeanstalk.Config `mapstructure:"elasticbeanstalk"`

	// Lambda contains user-specified configurations for the lambda detector
	LambdaConfig lambda.Config `mapstructure:"lambda"`

	// Azure contains user-specified configurations for the azure detector
	AzureConfig azure.Config `mapstructure:"azure"`

	// Aks contains user-specified configurations for the aks detector
	AksConfig aks.Config `mapstructure:"aks"`

	// ConsulConfig contains user-specified configurations for the Consul detector
	ConsulConfig consul.Config `mapstructure:"consul"`

	// DockerConfig contains user-specified configurations for the docker detector
	DockerConfig docker.Config `mapstructure:"docker"`

	// GcpConfig contains user-specified configurations for the gcp detector
	GcpConfig gcp.Config `mapstructure:"gcp"`

	// HerokuConfig contains user-specified configurations for the heroku detector
	HerokuConfig heroku.Config `mapstructure:"heroku"`

	// SystemConfig contains user-specified configurations for the System detector
	SystemConfig system.Config `mapstructure:"system"`

	// OpenShift contains user-specified configurations for the Openshift detector
	OpenShiftConfig openshift.Config `mapstructure:"openshift"`
}

func detectorCreateDefaultConfig() DetectorConfig {
	return DetectorConfig{
		EC2Config:              ec2.CreateDefaultConfig(),
		ECSConfig:              ecs.CreateDefaultConfig(),
		EKSConfig:              eks.CreateDefaultConfig(),
		ElasticbeanstalkConfig: elasticbeanstalk.CreateDefaultConfig(),
		LambdaConfig:           lambda.CreateDefaultConfig(),
		AzureConfig:            azure.CreateDefaultConfig(),
		AksConfig:              aks.CreateDefaultConfig(),
		ConsulConfig:           consul.CreateDefaultConfig(),
		DockerConfig:           docker.CreateDefaultConfig(),
		GcpConfig:              gcp.CreateDefaultConfig(),
		HerokuConfig:           heroku.CreateDefaultConfig(),
		SystemConfig:           system.CreateDefaultConfig(),
		OpenShiftConfig:        openshift.CreateDefaultConfig(),
	}
}

func (d *DetectorConfig) GetConfigFromType(detectorType internal.DetectorType) internal.DetectorConfig {
	switch detectorType {
	case ec2.TypeStr:
		return d.EC2Config
	case ecs.TypeStr:
		return d.ECSConfig
	case eks.TypeStr:
		return d.EKSConfig
	case elasticbeanstalk.TypeStr:
		return d.ElasticbeanstalkConfig
	case lambda.TypeStr:
		return d.LambdaConfig
	case azure.TypeStr:
		return d.AzureConfig
	case consul.TypeStr:
		return d.ConsulConfig
	case docker.TypeStr:
		return d.DockerConfig
	case gcp.TypeStr:
		return d.GcpConfig
	case heroku.TypeStr:
		return d.HerokuConfig
	case system.TypeStr:
		return d.SystemConfig
	case openshift.TypeStr:
		return d.OpenShiftConfig
	default:
		return nil
	}
}
