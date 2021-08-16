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

package awsxrayreceiver

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

// Config defines the configurations for an AWS X-Ray receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	// The `NetAddr` represents the UDP address
	// and port on which this receiver listens for X-Ray segment documents
	// emitted by the X-Ray SDK.
	confignet.NetAddr `mapstructure:",squash"`

	// ProxyServer defines configurations related to the local TCP proxy server.
	ProxyServer *proxy.Config `mapstructure:"proxy_server"`
}
