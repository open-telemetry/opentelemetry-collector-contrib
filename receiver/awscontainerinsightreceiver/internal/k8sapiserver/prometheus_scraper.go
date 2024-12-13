// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	// needs to start with "containerInsightsKubeAPIServerScraper" for histogram deltas in the emf exporter
	jobName = "containerInsightsKubeAPIServerScraper"
)

var controlPlaneMetricAllowList = []string{
	"^apiserver_admission_controller_admission_duration_seconds_(bucket|sum|count)$",
	"^apiserver_admission_step_admission_duration_seconds_(bucket|sum|count)$",
	"^apiserver_admission_webhook_admission_duration_seconds_(bucket|sum|count)$",
	"^apiserver_current_inflight_requests$",
	"^apiserver_current_inqueue_requests$",
	"^apiserver_flowcontrol_rejected_requests_total$",
	"^apiserver_flowcontrol_request_concurrency_limit$",
	"^apiserver_longrunning_requests$",
	"^apiserver_request_duration_seconds_(bucket|sum|count)$",
	"^apiserver_request_total$",
	"^apiserver_requested_deprecated_apis$",
	"^apiserver_storage_list_duration_seconds_(bucket|sum|count)$",
	"^apiserver_storage_objects$",
	"^apiserver_storage_db_total_size_in_bytes$",
	"^apiserver_storage_size_bytes$",
	"^etcd_db_total_size_in_bytes$",
	"^etcd_request_duration_seconds_(bucket|sum|count)$",
	"^rest_client_request_duration_seconds_(bucket|sum|count)$",
	"^rest_client_requests_total$",
}

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
		ScrapeProtocols: config.DefaultScrapeProtocols,
		JobName:         fmt.Sprintf("%s/%s", jobName, opts.Endpoint),
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
							"Type":             model.LabelValue("ControlPlane"),
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
			// type conflicts with the log Type in the container insights output format, it needs to be replaced and dropped
			{
				Regex:       relabel.MustNewRegexp("^type$"),
				Replacement: "kubernetes_type",
				Action:      relabel.LabelMap,
			},
			{
				Regex:  relabel.MustNewRegexp("^type$"),
				Action: relabel.LabelDrop,
			},
		},
	}

	if opts.BearerToken != "" {
		scrapeConfig.HTTPClientConfig.BearerToken = configutil.Secret(opts.BearerToken)
	} else {
		opts.TelemetrySettings.Logger.Warn("bearer token is not set, control plane metrics will not be published")
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &prometheusreceiver.PromConfig{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	params := receiver.Settings{
		ID:                component.MustNewID(jobName),
		TelemetrySettings: opts.TelemetrySettings,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetrics(opts.Ctx, params, &promConfig, opts.Consumer)
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
