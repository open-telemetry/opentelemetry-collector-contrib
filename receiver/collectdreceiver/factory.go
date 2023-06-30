// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver/internal/metadata"
)

// This file implements factory for CollectD receiver.

const (
	defaultBindEndpoint   = "localhost:8081"
	defaultTimeout        = time.Second * 30
	defaultEncodingFormat = "json"
)

// NewFactory creates a factory for collectd receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
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
