// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lightstepexporter

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines configuration options for the LightStep exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	// AccessToken is your project access token in LightStep.
	AccessToken string `mapstructure:"access_token"`
	// SatelliteHost is the satellite pool hostname.
	SatelliteHost string `mapstructure:"satellite_host"`
	// SatellitePort is the satellite pool port.
	SatellitePort int `mapstructure:"satellite_port"`
	// ServiceName is the LightStep service name for your spans.
	ServiceName string `mapstructure:"service_name"`
	// PlainText is true for plain text satellite pools.
	PlainText bool `mapstructure:"plain_text"`
}
