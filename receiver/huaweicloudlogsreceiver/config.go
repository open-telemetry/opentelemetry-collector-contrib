// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudlogsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/huawei"
)

var (
	errMissingGroupID  = errors.New(`"log_group_id" is not specified in config`)
	errMissingStreamID = errors.New(`"log_stream_id" is not specified in config`)
)

// Config represent a configuration for the CloudWatch logs exporter.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`
	// Set of attributes used to configure huawei's LTS SDK connection
	huawei.HuaweiSessionConfig `mapstructure:",squash"`

	// ProjectID is a string to reference project where logs should be associated with.
	// If ProjectID is not filled in, the SDK will automatically call the IAM service to query the project id corresponding to the region.
	ProjectID string `mapstructure:"project_id"`

	// RegionID is the ID of the LTS region.
	RegionID string `mapstructure:"region_id"`

	// GroupID is the ID of the LTS log group.
	GroupID string `mapstructure:"log_group_id"`

	// Stream is the ID of the LTS log stream.
	StreamID string `mapstructure:"log_stream_id"`

	BackOffConfig configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

var _ component.Config = (*Config)(nil)

// Validate config
func (config *Config) Validate() error {
	var err error
	if config.ProjectID == "" {
		err = multierr.Append(err, huawei.ErrMissingProjectID)
	}
	if config.RegionID == "" {
		err = multierr.Append(err, huawei.ErrMissingRegionID)
	}
	if config.GroupID == "" {
		err = multierr.Append(err, errMissingGroupID)
	}
	if config.StreamID == "" {
		err = multierr.Append(err, errMissingStreamID)
	}

	// Validate that ProxyAddress is provided if ProxyUser or ProxyPassword is set
	if (config.ProxyUser != "" || config.ProxyPassword != "") && config.ProxyAddress == "" {
		err = multierr.Append(err, huawei.ErrInvalidProxy)
	}

	return err
}
