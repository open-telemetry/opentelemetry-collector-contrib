// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/gpu"

import (
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

const (
	caFile                    = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	collectionInterval        = 60 * time.Second
	jobName                   = "containerInsightsDCGMExporterScraper"
	scraperMetricsPath        = "/metrics"
	scraperK8sServiceSelector = "k8s-app=dcgm-exporter-service"
)

type hostInfoProvider interface {
	GetClusterName() string
	GetInstanceID() string
	GetInstanceType() string
}

func GetScraperConfig(hostInfoProvider hostInfoProvider) *config.ScrapeConfig {
	return &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				CAFile:             caFile,
				InsecureSkipVerify: false,
			},
		},
		ScrapeInterval:  model.Duration(collectionInterval),
		ScrapeTimeout:   model.Duration(collectionInterval),
		ScrapeProtocols: config.DefaultScrapeProtocols,
		JobName:         jobName,
		Scheme:          "https",
		MetricsPath:     scraperMetricsPath,
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
			TargetLabel:  ci.K8sNamespace,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		// hacky way to inject static values (clusterName/instanceId/instanceType)
		// could be removed since these labels are now set by localNode decorator
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
			SourceLabels: model.LabelNames{"namespace"},
			TargetLabel:  ci.InstanceType,
			Regex:        relabel.MustNewRegexp(".*"),
			Replacement:  hostInfoProvider.GetInstanceType(),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"pod"},
			TargetLabel:  ci.FullPodNameKey,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		// additional k8s podname for service name and k8s blob decoration
		{
			SourceLabels: model.LabelNames{"pod"},
			TargetLabel:  ci.PodNameKey,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"container"},
			TargetLabel:  ci.ContainerNamekey,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"device"},
			TargetLabel:  ci.GpuDevice,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
	}
}
