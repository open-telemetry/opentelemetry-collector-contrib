// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kueuescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver/internal/kueuescraper"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const (
	kmCollectionInterval = 60 * time.Second
	// kmJobName needs to be "containerInsightsKueueMetricsScraper" so metric translator tags the source as the container insights receiver
	kmJobName                   = "containerInsightsKueueMetricsScraper"
	kueueNamespace              = "kueue-system"
	kueueNameLabelSelector      = "app.kubernetes.io/name=kueue"
	kueueComponentLabelSelector = "app.kubernetes.io/component=controller"
	kueueServiceFieldSelector   = "metadata.name=kueue-controller-manager-metrics-service"
	kueueMetricsLogStream       = "kubernetes-kueue"
)

var ( // list of regular expressions for the kueue metrics this scraper is intended to capture
	kueueMetricAllowList = []string{
		"^kueue_pending_workloads$",
		"^kueue_evicted_workloads_total$",
		"^kueue_admitted_active_workloads$",
		"^kueue_cluster_queue_resource_usage$",
		"^kueue_cluster_queue_nominal_quota$",
	}
	kueueMetricsAllowRegex    = strings.Join(kueueMetricAllowList, "|")
	kueueDimensionsRelabelMap = map[string]string{
		"cluster_queue": "ClusterQueue",
		"flavor":        "Flavor",
		"reason":        "Reason",
		"resource":      "Resource",
		"status":        "Status",
	}
)

type KueuePrometheusScraper struct {
	ctx                context.Context
	settings           component.TelemetrySettings
	host               component.Host
	clusterName        string
	prometheusReceiver receiver.Metrics
	running            bool
}

type KueuePrometheusScraperOpts struct {
	Ctx               context.Context
	TelemetrySettings component.TelemetrySettings
	Consumer          consumer.Metrics
	Host              component.Host
	ClusterName       string
	BearerToken       string
}

func NewKueuePrometheusScraper(opts KueuePrometheusScraperOpts) (*KueuePrometheusScraper, error) {
	if opts.Consumer == nil {
		return nil, errors.New("consumer cannot be nil")
	}
	if opts.Host == nil {
		return nil, errors.New("host cannot be nil")
	}
	if opts.ClusterName == "" {
		return nil, errors.New("cluster name cannot be empty")
	}

	scrapeConfig := &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
		ScrapeInterval:  model.Duration(kmCollectionInterval),
		ScrapeTimeout:   model.Duration(kmCollectionInterval),
		ScrapeProtocols: config.DefaultScrapeProtocols,
		JobName:         kmJobName,
		HonorTimestamps: true,
		Scheme:          "https",
		MetricsPath:     "/metrics",
		ServiceDiscoveryConfigs: discovery.Configs{
			&kubernetes.SDConfig{
				Role: kubernetes.RoleService,
				NamespaceDiscovery: kubernetes.NamespaceDiscovery{
					Names: []string{kueueNamespace},
				},
				Selectors: []kubernetes.SelectorConfig{
					{
						Role:  kubernetes.RoleService,
						Label: fmt.Sprintf("%s,%s", kueueNameLabelSelector, kueueComponentLabelSelector),
						Field: kueueServiceFieldSelector,
					},
				},
			},
		},
		MetricRelabelConfigs: GetKueueRelabelConfigs(opts.ClusterName),
	}

	if opts.BearerToken != "" {
		scrapeConfig.HTTPClientConfig.BearerToken = configutil.Secret(opts.BearerToken)
	} else {
		opts.TelemetrySettings.Logger.Warn("bearer token is not set, kueue metrics will not be published")
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &prometheusreceiver.PromConfig{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	params := receiver.Settings{
		ID:                component.MustNewID(kmJobName),
		TelemetrySettings: opts.TelemetrySettings,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetrics(opts.Ctx, params, &promConfig, opts.Consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus receiver for kueue metrics: %w", err)
	}

	return &KueuePrometheusScraper{
		ctx:                opts.Ctx,
		settings:           opts.TelemetrySettings,
		host:               opts.Host,
		clusterName:        opts.ClusterName,
		prometheusReceiver: promReceiver,
	}, nil
}

func GetKueueRelabelConfigs(clusterName string) []*relabel.Config {
	relabelConfigs := []*relabel.Config{
		{ // filter by metric name: keep only the Kueue metrics specified via regex in `kueueMetricAllowList`
			Action:       relabel.Keep,
			Regex:        relabel.MustNewRegexp(kueueMetricsAllowRegex),
			SourceLabels: model.LabelNames{"__name__"},
		},
		// type conflicts with the log Type in the container insights output format.
		{ // add "kubernetes_type" to serve as non-conflicting name.
			Action:      relabel.LabelMap,
			Regex:       relabel.MustNewRegexp("^type$"),
			Replacement: "kubernetes_type",
		},
		{ // drop conflicting name "type"
			Action: relabel.LabelDrop,
			Regex:  relabel.MustNewRegexp("^type$"),
		},
		{ // add port to value of label "__address__" if it isn't already included.
			Action:       relabel.Replace,
			Regex:        relabel.MustNewRegexp("([^:]+)(?::\\d+)?;(\\d+)"),
			SourceLabels: model.LabelNames{"__address__", "__meta_kubernetes_service_annotation_prometheus_io_port"},
			Replacement:  "$1:$2",
			TargetLabel:  "__address__",
		},
		{ // add cluster name as a label
			Action:      relabel.Replace,
			Regex:       relabel.MustNewRegexp(".*"),
			TargetLabel: "ClusterName",
			Replacement: clusterName,
		},
	}
	// relabel configs to change casing conventions for Kueue dimensions
	for sourceLabel, targetLabel := range kueueDimensionsRelabelMap {
		relabelConfigs = append(
			relabelConfigs,
			&relabel.Config{
				Action:       relabel.Replace,
				Regex:        relabel.MustNewRegexp("(.*)"),
				SourceLabels: model.LabelNames{model.LabelName(sourceLabel)},
				TargetLabel:  targetLabel,
				Replacement:  "${1}",
			},
		)
	}
	return relabelConfigs
}

func (kps *KueuePrometheusScraper) GetMetrics() []pmetric.Metrics {
	// This method will never return metrics because the metrics are collected by the scraper.

	if !kps.running {
		kps.settings.Logger.Info("The Kueue metrics scraper is not running, starting up the scraper")
		err := kps.prometheusReceiver.Start(kps.ctx, kps.host)
		if err != nil {
			kps.settings.Logger.Error("Unable to start Kueue PrometheusReceiver", zap.Error(err))
		}
		kps.running = err == nil
	}
	return nil
}

func (kps *KueuePrometheusScraper) Shutdown() {
	if kps.running {
		err := kps.prometheusReceiver.Shutdown(kps.ctx)
		if err != nil {
			kps.settings.Logger.Error("Unable to shutdown Kueue PrometheusReceiver", zap.Error(err))
		}
		kps.running = false
	}
}
