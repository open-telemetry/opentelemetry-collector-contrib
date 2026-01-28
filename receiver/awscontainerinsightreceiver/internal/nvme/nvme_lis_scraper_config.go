// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nvme // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/nvme"

import (
	"os"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	_ "github.com/prometheus/prometheus/discovery/install" // init() registers service discovery impl.
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

const (
	lisCollectionInterval        = 60 * time.Second
	lisJobName                   = "containerInsightsNVMeLISScraper"
	lisScraperMetricsPath        = "/metrics"
	lisScraperK8sServiceSelector = "app.kubernetes.io/name=instance-store-plugin"
	lisNamespaceDiscoveryName    = "kube-system"
)

func GetLisScraperConfig(hostInfoProvider hostInfoProvider) *config.ScrapeConfig {
	return &config.ScrapeConfig{
		ScrapeInterval:         model.Duration(lisCollectionInterval),
		ScrapeTimeout:          model.Duration(lisCollectionInterval),
		ScrapeProtocols:        config.DefaultScrapeProtocols,
		JobName:                lisJobName,
		Scheme:                 "http",
		MetricsPath:            lisScraperMetricsPath,
		ScrapeFallbackProtocol: config.PrometheusText0_0_4,
		ServiceDiscoveryConfigs: discovery.Configs{
			&kubernetes.SDConfig{
				Role: kubernetes.RoleService,
				NamespaceDiscovery: kubernetes.NamespaceDiscovery{
					Names: []string{lisNamespaceDiscoveryName},
				},
				Selectors: []kubernetes.SelectorConfig{
					{
						Role:  kubernetes.RoleService,
						Label: lisScraperK8sServiceSelector,
					},
				},
			},
		},
		MetricRelabelConfigs: getLisMetricRelabelConfig(hostInfoProvider),
	}
}

func getLisMetricRelabelConfig(hostInfoProvider hostInfoProvider) []*relabel.Config {
	return []*relabel.Config{
		{
			SourceLabels: model.LabelNames{"__name__"},
			Regex:        relabel.MustNewRegexp("aws_ec2_instance_store_csi_.*"),
			Action:       relabel.Keep,
		},

		// Below metrics are histogram type which are not supported for container insights yet
		{
			SourceLabels: model.LabelNames{"__name__"},
			Regex:        relabel.MustNewRegexp(".*_bucket|.*_sum|.*_count.*"),
			Action:       relabel.Drop,
		},
		// Inject static values (clusterName/instanceId/nodeName/volumeID)
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
