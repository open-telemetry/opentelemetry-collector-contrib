// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

// Config defines configuration for TencentCloud Log Service exporter.
type Config struct {
	// LogService's Region, https://cloud.tencent.com/document/product/614/18940
	// for TencentCloud Kubernetes(or CVM), set ap-{region}.cls.tencentyun.com, eg ap-beijing.cls.tencentyun.com;
	//  others set ap-{region}.cls.tencentcs.com, eg ap-beijing.cls.tencentcs.com
	Region string `mapstructure:"region"`
	// LogService's LogSet Name
	LogSet string `mapstructure:"logset"`
	// LogService's Topic Name
	Topic string `mapstructure:"topic"`
	// TencentCloud access key id
	SecretID string `mapstructure:"secret_id"`
	// TencentCloud access key secret
	SecretKey configopaque.String `mapstructure:"secret_key"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg == nil || cfg.Region == "" || cfg.LogSet == "" || cfg.Topic == "" {
		return errors.New("missing tencentcloudlogservice params: Region, LogSet, Topic")
	}
	return nil
}
