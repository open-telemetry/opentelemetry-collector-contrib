// Copyright 2021, OpenTelemetry Authors
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

package tencentcloudlogserviceexporter

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
)

// Config defines configuration for TencentCloud Log Service exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`
	// LogService's Endpoint, https://cloud.tencent.com/document/product/614/18940
	// for TencentCloud Kubernetes(or CVM), set ap-{region-id}.cls.tencentyun.com, eg ap-beijing.cls.tencentyun.com;
	//  others set ap-{region-id}.cls.tencentcs.com, eg ap-beijing.cls.tencentcs.com
	confignet.TCPAddr `mapstructure:",squash"`
	// LogService's LogSet Name
	LogSet string `mapstructure:"logset"`
	// LogService's Topic Name
	Topic string `mapstructure:"topic"`
	// TencentCloud access key id
	SecretID string `mapstructure:"secret_id"`
	// TencentCloud access key secret
	SecretKey string `mapstructure:"secret_key"`
}
