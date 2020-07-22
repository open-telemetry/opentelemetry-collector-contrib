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

package awsxrayreceiver

import (
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
)

const (
	typeStr = "aws_xray"
	version = "1.0.0"
)

// Config defines the configurations for an AWS X-Ray receiver.
type Config struct {
	// The Endpoint field in ReceiverSettings represents the UDP address
	// and port on which this receiver listens for X-Ray segment documents
	// emitted by the X-Ray SDK.
	configmodels.ReceiverSettings `mapstructure:",squash"`
	confignet.TCPAddr             `mapstructure:",squash"`

	// squash ensures fields are correctly decoded in embedded struct
	// https://godoc.org/github.com/mitchellh/mapstructure#hdr-Embedded_Structs_and_Squashing

	// ProxyServer defines configurations related to the local TCP proxy server.
	ProxyServer *proxyServer `mapstructure:"proxy_server"`
}

type proxyServer struct {
	// TCPEndpoint is the address and port on which this receiver listens for
	// calls from the X-Ray SDK and relays them to the AWS X-Ray backend to
	// get sampling rules and report sampling statistics.
	TCPEndpoint string `mapstructure:"tcp_endpoint"`

	// ProxyAddress defines the proxy address that the local TCP server
	// forwards HTTP requests to AWS X-Ray backend through.
	ProxyAddress string `mapstructure:"proxy_address"`

	// TLSSetting struct exposes TLS client configuration when forwarding
	// calls to the AWS X-Ray backend.
	TLSSetting configtls.TLSClientSetting `mapstructure:",squash"`

	// Region is the AWS region the local TCP server forwards requests to.
	Region string `mapstructure:"region"`

	// RoleARN is the IAM role used by the local TCP server when
	// communicating with the AWS X-Ray service.
	RoleARN string `mapstructure:"role_arn"`

	// AWSEndpoint is the X-Ray service endpoint which the local
	// TCP server forwards requests to.
	AWSEndpoint string `mapstructure:"aws_endpoint"`

	// LocalMode determines whether the EC2 instance metadata endpoint
	// will be called or not. Set to `true` to skip EC2 instance
	// metadata check.
	LocalMode *bool `mapstructure:"local_mode"`
}
