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

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"
)

const (
	caFile             = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	collectionInterval = 60 * time.Second
)

type PrometheusScraper struct {
	ctx                      context.Context
	settings                 component.TelemetrySettings
	host                     component.Host
	clusterNameProvider      clusterNameProvider
	simplePrometheusReceiver receiver.Metrics
	leaderElection           *LeaderElection
	running                  bool
}

func NewPrometheusScraper(ctx context.Context, telemetrySettings component.TelemetrySettings, endpoint string, nextConsumer consumer.Metrics, host component.Host, clusterNameProvider clusterNameProvider, leaderElection *LeaderElection) (*PrometheusScraper, error) {
	if leaderElection == nil {
		return nil, errors.New("leader election cannot be null")
	}

	spConfig := simpleprometheusreceiver.Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: endpoint,
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: caFile,
				},
				Insecure:           false,
				InsecureSkipVerify: false,
			},
		},
		MetricsPath:        "/metrics",
		CollectionInterval: collectionInterval,
		UseServiceAccount:  true,
	}

	consumer := newPrometheusConsumer(telemetrySettings.Logger, nextConsumer, clusterNameProvider.GetClusterName(), os.Getenv("HOST_NAME"))

	params := receiver.CreateSettings{
		TelemetrySettings: telemetrySettings,
	}

	spFactory := simpleprometheusreceiver.NewFactory()
	spr, err := spFactory.CreateMetricsReceiver(ctx, params, &spConfig, consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create simple prometheus receiver: %w", err)
	}

	return &PrometheusScraper{
		ctx:                      ctx,
		settings:                 telemetrySettings,
		host:                     host,
		clusterNameProvider:      clusterNameProvider,
		simplePrometheusReceiver: spr,
		leaderElection:           leaderElection,
	}, nil
}

func (ps *PrometheusScraper) GetMetrics() []pmetric.Metrics {
	// This method will never return metrics because the metrics are collected by the scraper.
	// This method will ensure the scraper is running
	if !ps.leaderElection.leading {
		return nil
	}

	// if we are leading, ensure we are running
	if !ps.running {
		ps.settings.Logger.Info("The scraper is not running, starting up the scraper")
		err := ps.simplePrometheusReceiver.Start(ps.ctx, ps.host)
		if err != nil {
			ps.settings.Logger.Error("Unable to start SimplePrometheusReceiver", zap.Error(err))
		}
		ps.running = err == nil
	}
	return nil
}
func (ps *PrometheusScraper) Shutdown() {
	if ps.running {
		err := ps.simplePrometheusReceiver.Shutdown(ps.ctx)
		if err != nil {
			ps.settings.Logger.Error("Unable to shutdown SimplePrometheusReceiver", zap.Error(err))
		}
		ps.running = false
	}
}
