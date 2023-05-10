// Copyright The OpenTelemetry Authors
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
