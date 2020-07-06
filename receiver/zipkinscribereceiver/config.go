// Copyright 2019, OpenTelemetry Authors
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

package zipkinscribereceiver

import "go.opentelemetry.io/collector/config/configmodels"

// Config defines configuration for Zipkin-Scribe receiver.
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	// TODO: Use one of the configs from core.
	// The target endpoint.
	Endpoint string `mapstructure:"endpoint"`

	// Category is the string that will be used to identify the scribe log
	// messages that contain Zipkin spans.
	Category string `mapstructure:"category"`
}
