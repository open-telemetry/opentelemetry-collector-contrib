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

package influxdbexporter

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr config.Type = "influxdb"

	protocolLineProtocol = "line-protocol"
	protocolGrpc         = "grpc"
)

// Config defines configuration for the InfluxDB exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`

	// Org is the InfluxDB organization name of the destination bucket.
	Org string `mapstructure:"org"`
	// Bucket is the InfluxDB bucket name that telemetry will be written to.
	Bucket string `mapstructure:"bucket"`

	// Protocol is the method used to deliver metrics to InfluxDB. Must be one of:
	// line-protocol, grpc, json
	Protocol string `mapstructure:"protocol"`

	// LineProtocolServerURL is the destination for Protocol "line-protocol".
	// For example http://localhost:8086
	LineProtocolServerURL string `mapstructure:"line_protocol_server_url"`
	// LineProtocolAuthToken is the authentication token for Protocol "line-protocol".
	LineProtocolAuthToken string `mapstructure:"line_protocol_auth_token"`

	// TODO add gRPC, JSON
	// configgrpc.GRPCClientSettings `mapstructure:",squash"`
}
