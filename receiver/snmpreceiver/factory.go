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

package snmpreceiver

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr = "snmp"

	defaultCollectionInterval = 60 // In seconds
	defaultEndpoint           = "udp://localhost:161"
	defaultVersion            = "v2c"
	defaultCommunity          = "public"
)

func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createReceiver))
}

func createDefaultConfig() config.Receiver {
	scs := scraperhelper.DefaultScraperControllerSettings(typeStr)
	scs.CollectionInterval = defaultCollectionInterval * time.Second
	return &Config{
		ScraperControllerSettings: scs,
		Endpoint:                  defaultEndpoint,
		Version:                   defaultVersion,
		Community:                 defaultCommunity,
	}
}

func createReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	config config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	snmpConfig := config.(*internal.Config)

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	snmpReceiver, err := NewSNMPReceiver(ctx, params, snmpConfig, consumer)
	if err != nil {
		return nil, err
	}

	return snmpReceiver, nil
}
