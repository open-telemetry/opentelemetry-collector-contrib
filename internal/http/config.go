// Copyright 2020, OpenTelemetry Authors
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
package http

import "github.com/open-telemetry/opentelemetry-collector/receiver"

// TODO: Pull in other utilities from
// https://github.com/signalfx/signalfx-agent/blob/master/pkg/core/common/httpclient/http.go.
// HTTPConfig holds config options related to HTTP(S)
type HTTPConfig struct {
	// Whether not TLS is enabled
	TLSEnabled bool      `mapstructure:"tls_enabled"`
	TLSConfig  TLSConfig `mapstructure:"tls_config"`
}

// TLSConfig holds common TLS config options
type TLSConfig struct {
	receiver.TLSCredentials `mapstructure:",squash"`
	// Path to the CA cert that has signed the TLS cert.
	CAFile string `mapstructure:"ca_file"`
	// Whether or not to verify the exporter's TLS cert.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}
