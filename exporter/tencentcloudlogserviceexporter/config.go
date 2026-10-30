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
	//  others set ap-{region}.cls.tencentcs.com, eg ap-beijing.cls.tencentcs.com.
	// Optional if Endpoint is set.
	Region string `mapstructure:"region"`
	// LogService's Endpoint, https://cloud.tencent.com/document/product/614/18940
	// for TencentCloud Kubernetes(or CVM), set ap-{region}.cls.tencentyun.com, eg ap-beijing.cls.tencentyun.com;
	//  others set ap-{region}.cls.tencentcs.com, eg ap-beijing.cls.tencentcs.com.
	// Takes precedence over Region when set.
	Endpoint string `mapstructure:"endpoint"`
	// LogService's LogSet Name
	LogSet string `mapstructure:"logset"`
	// LogService's Topic Name
	Topic string `mapstructure:"topic"`
	// TencentCloud access key id
	SecretID string `mapstructure:"secret_id"`
	// TencentCloud access key secret
	SecretKey configopaque.String `mapstructure:"secret_key"`
	// Whether to use the TencentCloud intranet endpoint, only used when Endpoint is empty
	IsIntranet bool `mapstructure:"is_intranet"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg == nil || cfg.LogSet == "" || cfg.Topic == "" {
		return errors.New("missing tencentcloudlogservice params: LogSet, Topic")
	}
	if cfg.Region == "" && cfg.Endpoint == "" {
		return errors.New("missing tencentcloudlogservice param: Region or Endpoint")
	}
	return nil
}
