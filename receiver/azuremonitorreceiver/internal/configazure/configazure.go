// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package configauth implements the configuration settings to
// ensure authentication on incoming requests, and allows
// exporters to add authentication on outgoing requests.
package configazure

type AzureSettings struct {
	SubscriptionId                string   `mapstructure:"subscription_id"`
	TenantId                      string   `mapstructure:"tenant_id"`
	ClientId                      string   `mapstructure:"client_id"`
	ClientSecret                  string   `mapstructure:"client_secret"`
	ResourceGroups                []string `mapstructure:"resource_groups"`
	CacheResources                int64    `mapstructure:"cache_resources"`
	CacheResourcesDefinitions     int64    `mapstructure:"cache_resources_definitions"`
	MaximumNumberOfMetricsInACall int      `mapstructure:"maximum_number_of_metrics_in_a_call"`
}
