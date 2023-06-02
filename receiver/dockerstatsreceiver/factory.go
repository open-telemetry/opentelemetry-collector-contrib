// Copyright The OpenTelemetry Authors
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

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

const (
	typeStr   = "docker_stats"
	stability = component.StabilityLevelAlpha
)

var _ = featuregate.GlobalRegistry().MustRegister(
	"receiver.dockerstats.useScraperV2",
	featuregate.StageStable,
	featuregate.WithRegisterDescription("When enabled, the receiver will use the function ScrapeV2 to collect metrics. This allows each metric to be turned off/on via config. The new metrics are slightly different to the legacy implementation."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9794"),
	featuregate.WithRegisterToVersion("0.74.0"),
)

func NewFactory() rcvr.Factory {
	return rcvr.NewFactory(
		typeStr,
		createDefaultConfig,
		rcvr.WithMetrics(createMetricsReceiver, stability))
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultScraperControllerSettings(typeStr)
	scs.CollectionInterval = 10 * time.Second
	return &Config{
		ScraperControllerSettings: scs,
		Endpoint:                  "unix:///var/run/docker.sock",
		Timeout:                   5 * time.Second,
		DockerAPIVersion:          defaultDockerAPIVersion,
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	params rcvr.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (rcvr.Metrics, error) {
	dockerConfig := config.(*Config)
	dsr := newReceiver(params, dockerConfig)

	scrp, err := scraperhelper.NewScraper(typeStr, dsr.scrapeV2, scraperhelper.WithStart(dsr.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&dsr.config.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(scrp))
}
