// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"
	"errors"
	"fmt"
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

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const (
	caFile                    = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	collectionInterval        = 60 * time.Second
	jobName                   = "containerInsightsDCGMExporterScraper"
	scraperMetricsPath        = "/metrics"
	scraperK8sServiceSelector = "k8s-app=dcgm-exporter-service"
)

type DcgmScraper struct {
	ctx                context.Context
	settings           component.TelemetrySettings
	host               component.Host
	hostInfoProvider   hostInfoProvider
	prometheusReceiver receiver.Metrics
	k8sDecorator       Decorator
	running            bool
}

type DcgmScraperOpts struct {
	Ctx               context.Context
	TelemetrySettings component.TelemetrySettings
	Consumer          consumer.Metrics
	Host              component.Host
	HostInfoProvider  hostInfoProvider
	K8sDecorator      Decorator
	Logger            *zap.Logger
}

type hostInfoProvider interface {
	GetClusterName() string
	GetInstanceID() string
}

func NewDcgmScraper(opts DcgmScraperOpts) (*DcgmScraper, error) {
	if opts.Consumer == nil {
		return nil, errors.New("consumer cannot be nil")
	}
	if opts.Host == nil {
		return nil, errors.New("host cannot be nil")
	}
	if opts.HostInfoProvider == nil {
		return nil, errors.New("cluster name provider cannot be nil")
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{getScraperConfig(opts.HostInfoProvider)},
		},
	}

	params := receiver.CreateSettings{
		TelemetrySettings: opts.TelemetrySettings,
	}

	decoConsumer := decorateConsumer{
		containerOrchestrator: ci.EKS,
		nextConsumer:          opts.Consumer,
		k8sDecorator:          opts.K8sDecorator,
		logger:                opts.Logger,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetricsReceiver(opts.Ctx, params, &promConfig, &decoConsumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	return &DcgmScraper{
		ctx:                opts.Ctx,
		settings:           opts.TelemetrySettings,
		host:               opts.Host,
		hostInfoProvider:   opts.HostInfoProvider,
		prometheusReceiver: promReceiver,
		k8sDecorator:       opts.K8sDecorator,
	}, nil
}

func getScraperConfig(hostInfoProvider hostInfoProvider) *config.ScrapeConfig {
	return &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				CAFile:             caFile,
				InsecureSkipVerify: false,
			},
		},
		ScrapeInterval: model.Duration(collectionInterval),
		ScrapeTimeout:  model.Duration(collectionInterval),
		JobName:        jobName,
		Scheme:         "https",
		MetricsPath:    scraperMetricsPath,
		ServiceDiscoveryConfigs: discovery.Configs{
			&kubernetes.SDConfig{
				Role: kubernetes.RoleService,
				NamespaceDiscovery: kubernetes.NamespaceDiscovery{
					IncludeOwnNamespace: true,
				},
				Selectors: []kubernetes.SelectorConfig{
					{
						Role:  kubernetes.RoleService,
						Label: scraperK8sServiceSelector,
					},
				},
			},
		},
		MetricRelabelConfigs: getMetricRelabelConfig(hostInfoProvider),
	}
}

func getMetricRelabelConfig(hostInfoProvider hostInfoProvider) []*relabel.Config {
	return []*relabel.Config{
		{
			SourceLabels: model.LabelNames{"__name__"},
			Regex:        relabel.MustNewRegexp("DCGM_.*"),
			Action:       relabel.Keep,
		},
		{
			SourceLabels: model.LabelNames{"Hostname"},
			TargetLabel:  ci.NodeNameKey,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"namespace"},
			TargetLabel:  ci.AttributeK8sNamespace,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		// hacky way to inject static values (clusterName & instanceId) to label set without additional processor
		// relabel looks up an existing label then creates another label with given key (TargetLabel) and value (static)
		{
			SourceLabels: model.LabelNames{"namespace"},
			TargetLabel:  ci.ClusterNameKey,
			Regex:        relabel.MustNewRegexp(".*"),
			Replacement:  hostInfoProvider.GetClusterName(),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"namespace"},
			TargetLabel:  ci.InstanceID,
			Regex:        relabel.MustNewRegexp(".*"),
			Replacement:  hostInfoProvider.GetInstanceID(),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"pod"},
			TargetLabel:  ci.AttributeFullPodName,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		// additional k8s podname for service name and k8s blob decoration
		{
			SourceLabels: model.LabelNames{"pod"},
			TargetLabel:  ci.AttributeK8sPodName,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"container"},
			TargetLabel:  ci.AttributeContainerName,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"device"},
			TargetLabel:  ci.AttributeGpuDevice,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
	}
}

func (ds *DcgmScraper) GetMetrics() []pmetric.Metrics {
	// This method will never return metrics because the metrics are collected by the scraper.
	// This method will ensure the scraper is running
	if !ds.running {
		ds.settings.Logger.Info("The scraper is not running, starting up the scraper")
		err := ds.prometheusReceiver.Start(ds.ctx, ds.host)
		if err != nil {
			ds.settings.Logger.Error("Unable to start PrometheusReceiver", zap.Error(err))
		}
		ds.running = err == nil
	}

	return nil
}

func (ds *DcgmScraper) Shutdown() {
	if ds.running {
		err := ds.prometheusReceiver.Shutdown(ds.ctx)
		if err != nil {
			ds.settings.Logger.Error("Unable to shutdown PrometheusReceiver", zap.Error(err))
		}
		ds.running = false
	}

	if ds.k8sDecorator != nil {
		err := ds.k8sDecorator.Shutdown()
		if err != nil {
			ds.settings.Logger.Error("Unable to shutdown K8sDecorator", zap.Error(err))
		}
	}
}
