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

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

// Config defines configuration for the exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`
	Protocol                Protocol         `mapstructure:"protocol"`
	Resolver                ResolverSettings `mapstructure:"resolver"`
}

// Protocol holds the individual protocol-specific settings. Only OTLP is supported at the moment.
type Protocol struct {
	OTLP otlpexporter.Config `mapstructure:"otlp"`
}

// ResolverSettings defines the configurations for the backend resolver
type ResolverSettings struct {
	Static *StaticResolver `mapstructure:"static"`
	DNS    *DNSResolver    `mapstructure:"dns"`
}

// StaticResolver defines the configuration for the resolver providing a fixed list of backends
type StaticResolver struct {
	Hostnames []string `mapstructure:"hostnames"`
}

// DNSResolver defines the configuration for the DNS resolver
type DNSResolver struct {
	Hostname string `mapstructure:"hostname"`
	Port     string `mapstructure:"port"`
}
