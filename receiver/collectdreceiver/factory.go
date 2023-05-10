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

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// This file implements factory for CollectD receiver.

const (
	typeStr               = "collectd"
	stability             = component.StabilityLevelAlpha
	defaultBindEndpoint   = "localhost:8081"
	defaultTimeout        = time.Second * 30
	defaultEncodingFormat = "json"
)

// NewFactory creates a factory for collectd receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}
func createDefaultConfig() component.Config {
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: defaultBindEndpoint,
		},
		Timeout:  defaultTimeout,
		Encoding: defaultEncodingFormat,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	c.Encoding = strings.ToLower(c.Encoding)
	// CollectD receiver only supports JSON encoding. We expose a config option
	// to make it explicit and obvious to the users.
	if c.Encoding != defaultEncodingFormat {
		return nil, fmt.Errorf(
			"CollectD only support JSON encoding format. %s is not supported",
			c.Encoding,
		)
	}
	return newCollectdReceiver(params.Logger, c.Endpoint, c.Timeout, c.AttributesPrefix, nextConsumer)
}
