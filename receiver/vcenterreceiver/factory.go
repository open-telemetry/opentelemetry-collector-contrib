// Copyright  OpenTelemetry Authors
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

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

const (
	typeStr   = "vcenter"
	stability = component.StabilityLevelAlpha
)

// NewFactory returns the receiver factory for the vcenterreceiver
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver, stability),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: 2 * time.Minute,
		},
		TLSClientSetting: configtls.TLSClientSetting{},
		Metrics:          metadata.DefaultMetricsSettings(),
	}
}

var errConfigNotVcenter = errors.New("config was not an vcenter receiver config")

func logDeprecatedFeatureGateForDirection(log *zap.Logger, gate featuregate.Gate) {
	log.Warn("WARNING: The " + gate.ID + " feature gate is deprecated and will be removed in the next release. The change to remove " +
		"the direction attribute has been reverted in the specification. See https://github.com/open-telemetry/opentelemetry-specification/issues/2726 " +
		"for additional details.")
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotVcenter
	}
	vr := newVmwareVcenterScraper(params.Logger, cfg, params)

	if !vr.emitMetricsWithDirectionAttribute {
		logDeprecatedFeatureGateForDirection(vr.logger, emitMetricsWithDirectionAttributeFeatureGate)
	}

	if vr.emitMetricsWithoutDirectionAttribute {
		logDeprecatedFeatureGateForDirection(vr.logger, emitMetricsWithoutDirectionAttributeFeatureGate)
	}
	scraper, err := scraperhelper.NewScraper(
		typeStr,
		vr.scrape,
		scraperhelper.WithStart(vr.Start),
		scraperhelper.WithShutdown(vr.Shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
