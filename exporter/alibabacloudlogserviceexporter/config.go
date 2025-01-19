// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import "go.opentelemetry.io/collector/config/configopaque" // Config defines configuration for AlibabaCloud Log Service exporter.

type Config struct {
	// LogService's Endpoint, https://www.alibabacloud.com/help/doc-detail/29008.htm
	// for AlibabaCloud Kubernetes(or ECS), set {region-id}-intranet.log.aliyuncs.com, eg cn-hangzhou-intranet.log.aliyuncs.com;
	//  others set {region-id}.log.aliyuncs.com, eg cn-hangzhou.log.aliyuncs.com
	Endpoint string `mapstructure:"endpoint"`
	// LogService's Project Name
	Project string `mapstructure:"project"`
	// LogService's Logstore Name
	Logstore string `mapstructure:"logstore"`
	// AlibabaCloud access key id
	AccessKeyID string `mapstructure:"access_key_id"`
	// AlibabaCloud access key secret
	AccessKeySecret configopaque.String `mapstructure:"access_key_secret"`
	// Set AlibabaCLoud ECS ram role if you are using ACK
	ECSRamRole string `mapstructure:"ecs_ram_role"`
	// Set Token File Path if you are using ACK
	TokenFilePath string `mapstructure:"token_file_path"`
}
