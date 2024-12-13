// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/neuron"

import (
	"os"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"
)

const (
	caFile                    = "/etc/amazon-cloudwatch-observability-agent-cert/tls-ca.crt"
	collectionInterval        = 60 * time.Second
	jobName                   = "containerInsightsNeuronMonitorScraper"
	scraperMetricsPath        = "/metrics"
	scraperK8sServiceSelector = "k8s-app=neuron-monitor-service"
)

func GetNeuronScrapeConfig(hostinfo prometheusscraper.HostInfoProvider) *config.ScrapeConfig {
	return &config.ScrapeConfig{
		ScrapeProtocols: config.DefaultScrapeProtocols,
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
		MetricRelabelConfigs: GetNeuronMetricRelabelConfigs(hostinfo),
	}
}

func GetNeuronMetricRelabelConfigs(hostinfo prometheusscraper.HostInfoProvider) []*relabel.Config {
	return []*relabel.Config{
		{
			SourceLabels: model.LabelNames{"__name__"},
			Regex:        relabel.MustNewRegexp("neuron.*|system_.*|execution_.*|hardware_ecc_.*"),
			Action:       relabel.Keep,
		},
		{
			SourceLabels: model.LabelNames{"neuroncore"},
			TargetLabel:  "NeuronCore",
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"instance_id"},
			TargetLabel:  ci.NodeNameKey,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  os.Getenv("HOST_NAME"),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"neuron_device_index"},
			TargetLabel:  "NeuronDevice",
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		// hacky way to inject static values (clusterName) to label set without additional processor
		// could be removed since these labels are now set by localNode decorator
		{
			SourceLabels: model.LabelNames{"instance_id"},
			TargetLabel:  ci.ClusterNameKey,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  hostinfo.GetClusterName(),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"instance_id"},
			TargetLabel:  ci.InstanceID,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  hostinfo.GetInstanceID(),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"instance_type"},
			TargetLabel:  ci.InstanceType,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  hostinfo.GetInstanceType(),
			Action:       relabel.Replace,
		},
	}
}
