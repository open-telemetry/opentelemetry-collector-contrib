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

package cfgschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/cfgschema"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

const (
	receiverGroup  = "receiver"
	extensionGroup = "extension"
	processorGroup = "processor"
	exporterGroup  = "exporter"
)

// cfgInfo contains a component config instance, as well as its group name and
// type.
type cfgInfo struct {
	// the name of the component group, e.g. "receiver"
	Group string
	// the component type, e.g. "otlpreceiver.Config"
	Type component.Type
	// an instance of the component's configuration struct
	CfgInstance interface{}
}

// getAllCfgInfos accepts a Factories struct, then creates and returns a cfgInfo
// for each of its components.
func getAllCfgInfos(components otelcol.Factories) []cfgInfo {
	var out []cfgInfo
	for _, f := range components.Receivers {
		out = append(out, cfgInfo{
			Type:        f.Type(),
			Group:       receiverGroup,
			CfgInstance: f.CreateDefaultConfig(),
		})
	}
	for _, f := range components.Extensions {
		out = append(out, cfgInfo{
			Type:        f.Type(),
			Group:       extensionGroup,
			CfgInstance: f.CreateDefaultConfig(),
		})
	}
	for _, f := range components.Processors {
		out = append(out, cfgInfo{
			Type:        f.Type(),
			Group:       processorGroup,
			CfgInstance: f.CreateDefaultConfig(),
		})
	}
	for _, f := range components.Exporters {
		out = append(out, cfgInfo{
			Type:        f.Type(),
			Group:       exporterGroup,
			CfgInstance: f.CreateDefaultConfig(),
		})
	}
	return out
}
