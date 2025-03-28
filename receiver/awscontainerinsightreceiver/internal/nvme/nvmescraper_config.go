// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nvme // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/gpu"

import (
	"os"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

const (
	collectionInterval        = 60 * time.Second
	jobName                   = "containerInsightsNVMeExporterScraper"
	scraperMetricsPath        = "/metrics"
	scraperK8sServiceSelector = "app=ebs-csi-node"
)

type hostInfoProvider interface {
	GetClusterName() string
	GetInstanceID() string
	GetInstanceType() string
}

func GetScraperConfig(hostInfoProvider hostInfoProvider) *config.ScrapeConfig {
	return &config.ScrapeConfig{
		ScrapeInterval:  model.Duration(collectionInterval),
		ScrapeTimeout:   model.Duration(collectionInterval),
		ScrapeProtocols: config.DefaultScrapeProtocols,
		JobName:         jobName,
		Scheme:          "http",
		MetricsPath:     scraperMetricsPath,
		ServiceDiscoveryConfigs: discovery.Configs{
			&kubernetes.SDConfig{
				Role: kubernetes.RoleService,
				NamespaceDiscovery: kubernetes.NamespaceDiscovery{
					Names: []string{"kube-system"},
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
			Regex:        relabel.MustNewRegexp("aws_ebs_csi_.*"),
			Action:       relabel.Keep,
		},

		// Below metrics are histogram type which are not supported for container insights yet
		{
			SourceLabels: model.LabelNames{"__name__"},
			Regex:        relabel.MustNewRegexp(".*_bucket|.*_sum|.*_count.*"),
			Action:       relabel.Drop,
		},
		// Hacky way to inject static values (clusterName/instanceId/nodeName/volumeID)
		{
			SourceLabels: model.LabelNames{"instance_id"},
			TargetLabel:  ci.NodeNameKey,
			Regex:        relabel.MustNewRegexp(".*"),
			Replacement:  os.Getenv("HOST_NAME"),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"instance_id"},
			TargetLabel:  ci.ClusterNameKey,
			Regex:        relabel.MustNewRegexp(".*"),
			Replacement:  hostInfoProvider.GetClusterName(),
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"instance_id"},
			TargetLabel:  ci.InstanceID,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
		{
			SourceLabels: model.LabelNames{"volume_id"},
			TargetLabel:  ci.VolumeID,
			Regex:        relabel.MustNewRegexp("(.*)"),
			Replacement:  "${1}",
			Action:       relabel.Replace,
		},
	}
}
