// Copyright 2022 OpenTelemetry Authors
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

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"fmt"
	"net"
	"strconv"

	"go.opentelemetry.io/collector/config"
)

var maxPacketSizeErr = fmt.Errorf("max_packet_size must be between 1 and 65527")
var socketBufferSizeErr = fmt.Errorf("socket_buffer_size must be > 0")
var portNumberRangeErr = fmt.Errorf("port number must be between 1 and 65535")
var formatErr = fmt.Errorf("format must be either opentelemetry or opentracing")

const (
	OPENTRACING   = "opentracing"
	OPENTELEMETRY = "opentelemetry"
)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	Address                 string `mapstructure:"endpoint"`
	MaxPacketSize           int    `mapstructure:"max_packet_size"`
	SocketBufferSize        int    `mapstructure:"socket_buffer_size"`
	Format                  string `mapstructure:"format"`
}

func (c *Config) validate() error {
	if c.MaxPacketSize > defaultMaxPacketSize || c.MaxPacketSize <= 0 {
		return maxPacketSizeErr
	}

	if c.SocketBufferSize < 0 {
		return socketBufferSizeErr
	}

	if c.Format != OPENTRACING && c.Format != OPENTELEMETRY {
		return formatErr
	}

	err := validateAddress(c.Address)
	if err != nil {
		return err
	}
	return nil
}

// extract the port number from string in "address:port" format. If the
// port number cannot be extracted returns an error.
func validateAddress(endpoint string) error {
	_, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is not formatted correctly: %s", err.Error())
	}
	port, err := strconv.ParseInt(portStr, 10, 0)
	if err != nil {
		return fmt.Errorf("endpoint port is not a number: %s", err.Error())
	}
	if port < 1 || port > 65535 {
		return portNumberRangeErr
	}
	return nil
}
