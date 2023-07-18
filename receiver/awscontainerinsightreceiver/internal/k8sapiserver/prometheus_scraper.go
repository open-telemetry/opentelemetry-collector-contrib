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
	"strings"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const (
	caFile             = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	collectionInterval = 60 * time.Second
)

var (
	controlPlaneMetricAllowList = []string{
		"apiserver_storage_oapiserver_storage_objectsbjects",
		"apiserver_request_total",
		"apiserver_request_duration_seconds.*",
		"apiserver_admission_controller_admission_duration_seconds.*",
		"rest_client_request_duration_seconds.*",
		"rest_client_requests_total",
		"etcd_request_duration_seconds.*",
		"etcd_db_total_size_in_bytes.*",
	}
)

type PrometheusScraper struct {
	ctx                 context.Context
	settings            component.TelemetrySettings
	host                component.Host
	clusterNameProvider clusterNameProvider
	prometheusReceiver  receiver.Metrics
	leaderElection      *LeaderElection
	running             bool
}

type PrometheusScraperOpts struct {
	Ctx                 context.Context
	TelemetrySettings   component.TelemetrySettings
	Endpoint            string
	Consumer            consumer.Metrics
	Host                component.Host
	ClusterNameProvider clusterNameProvider
	LeaderElection      *LeaderElection
	BearerToken         string
}

func NewPrometheusScraper(opts PrometheusScraperOpts) (*PrometheusScraper, error) {
	if opts.Consumer == nil {
		return nil, errors.New("consumer cannot be nil")
	}
	if opts.Host == nil {
		return nil, errors.New("host cannot be nil")
	}
	if opts.LeaderElection == nil {
		return nil, errors.New("leader election cannot be nil")
	}
	if opts.ClusterNameProvider == nil {
		return nil, errors.New("cluster name provider cannot be nil")
	}

	controlPlaneMetricsAllowRegex := ""
	for _, item := range controlPlaneMetricAllowList {
		controlPlaneMetricsAllowRegex += item + "|"
	}
	controlPlaneMetricsAllowRegex = strings.TrimSuffix(controlPlaneMetricsAllowRegex, "|")

	scrapeConfig := &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				CAFile:             caFile,
				InsecureSkipVerify: false,
			},
		},
		ScrapeInterval:  model.Duration(collectionInterval),
		ScrapeTimeout:   model.Duration(collectionInterval),
		JobName:         fmt.Sprintf("%s/%s", "containerInsightsKubeAPIServerScraper", opts.Endpoint),
		HonorTimestamps: true,
		Scheme:          "https",
		MetricsPath:     "/metrics",
		ServiceDiscoveryConfigs: discovery.Configs{
			&discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						{
							model.AddressLabel: model.LabelValue(opts.Endpoint),
							"ClusterName":      model.LabelValue(opts.ClusterNameProvider.GetClusterName()),
							"Version":          model.LabelValue("0"),
							"Sources":          model.LabelValue("[\"apiserver\"]"),
							"NodeName":         model.LabelValue(os.Getenv("HOST_NAME")),
							"Type":             model.LabelValue("control_plane"),
						},
					},
				},
			},
		},
		MetricRelabelConfigs: []*relabel.Config{
			{
				// allow list filter for the control plane metrics we care about
				SourceLabels: model.LabelNames{"__name__"},
				Regex:        relabel.MustNewRegexp(controlPlaneMetricsAllowRegex),
				Action:       relabel.Keep,
			},
		},
	}

	if opts.BearerToken != "" {
		scrapeConfig.HTTPClientConfig.BearerToken = configutil.Secret(opts.BearerToken)
	} else {
		opts.TelemetrySettings.Logger.Warn("bearer token is not set, control plane metrics will not be published")
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	params := receiver.CreateSettings{
		TelemetrySettings: opts.TelemetrySettings,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetricsReceiver(opts.Ctx, params, &promConfig, opts.Consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	return &PrometheusScraper{
		ctx:                 opts.Ctx,
		settings:            opts.TelemetrySettings,
		host:                opts.Host,
		clusterNameProvider: opts.ClusterNameProvider,
		prometheusReceiver:  promReceiver,
		leaderElection:      opts.LeaderElection,
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
		err := ps.prometheusReceiver.Start(ps.ctx, ps.host)
		if err != nil {
			ps.settings.Logger.Error("Unable to start PrometheusReceiver", zap.Error(err))
		}
		ps.running = err == nil
	}
	return nil
}
func (ps *PrometheusScraper) Shutdown() {
	if ps.running {
		err := ps.prometheusReceiver.Shutdown(ps.ctx)
		if err != nil {
			ps.settings.Logger.Error("Unable to shutdown PrometheusReceiver", zap.Error(err))
		}
		ps.running = false
	}
}
