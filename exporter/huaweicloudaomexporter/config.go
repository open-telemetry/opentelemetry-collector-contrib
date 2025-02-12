// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudaomexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter"

import "go.opentelemetry.io/collector/config/configopaque" // Config defines configuration for HuaweiCloud AOM exporter.

type Config struct {
	// Service's Endpoint
	Endpoint string `mapstructure:"endpoint"`
	// Service's RegionName
	RegionId string `mapstructure:"region_id"`
	// Service's Project Name
	ProjectId string `mapstructure:"project_id"`
	// Service's LogGroupId
	LogGroupId string `mapstructure:"log_group_id"`
	// Service's LogGroupId
	LogStreamId string `mapstructure:"log_stream_id"`
	// HuaweiCloud access key id
	AccessKeyID string `mapstructure:"access_key_id"`
	// HuaweiCloud access key secret
	AccessKeySecret configopaque.String `mapstructure:"access_key_secret"`
	// Upload data through a proxy.
	ProxyAddress string `mapstructure:"proxy_address"`
	// Set HuaweiCloud ECS ram role if you are using K8S
	ECSRamRole string `mapstructure:"ecs_ram_role"`
	// Set Token File Path if you are using K8S
	TokenFilePath string `mapstructure:"token_file_path"`
}
