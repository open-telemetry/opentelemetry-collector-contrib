// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	kube.ClientConfig       `mapstructure:",squash"`
	confignet.TCPAddrConfig `mapstructure:",squash"`

	// ExtraMetadataLabels contains list of extra metadata that should be taken from /pods endpoint
	// and put as extra labels on metrics resource.
	// No additional metadata is fetched by default, so there are no extra calls to /pods endpoint.
	// Supported values include container.id and k8s.volume.type.
	ExtraMetadataLabels []kubelet.MetadataLabel `mapstructure:"extra_metadata_labels"`

	// MetricGroupsToCollect provides a list of metrics groups to collect metrics from.
	// "container", "pod", "node" and "volume" are the only valid groups.
	MetricGroupsToCollect []kubelet.MetricGroup `mapstructure:"metric_groups"`

	// Configuration of the Kubernetes API client.
	K8sAPIConfig *k8sconfig.APIConfig `mapstructure:"k8s_api_config"`

	// NodeName is the node name to limit the discovery of nodes.
	// For example, node name can be set using the downward API inside the collector
	// pod spec as follows:
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then set this value to ${env:K8S_NODE_NAME} in the configuration.
	NodeName string `mapstructure:"node"`

	// MetricsBuilderConfig allows customizing scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`

	// NetworkCollectAllInterfaces allows to enable collecting metrics from all network interfaces instead of default one
	// Can be set separately for Pod and Node network metrics
	NetworkCollectAllInterfaces NetworkInterfacesEnablerConfig `mapstructure:"collect_all_network_interfaces"`
}

type NetworkInterfacesEnablerConfig struct {
	PodMetrics  bool `mapstructure:"pod"`
	NodeMetrics bool `mapstructure:"node"`
}

// getReceiverOptions returns scraperOptions is the config is valid,
// otherwise it will return an error.
func (cfg *Config) getReceiverOptions() (*scraperOptions, error) {
	err := kubelet.ValidateMetadataLabelsConfig(cfg.ExtraMetadataLabels)
	if err != nil {
		return nil, err
	}

	mgs, err := getMapFromSlice(cfg.MetricGroupsToCollect)
	if err != nil {
		return nil, err
	}

	ifaces := map[kubelet.MetricGroup]bool{
		kubelet.NodeMetricGroup: cfg.NetworkCollectAllInterfaces.NodeMetrics,
		kubelet.PodMetricGroup:  cfg.NetworkCollectAllInterfaces.PodMetrics,
	}

	var k8sAPIClient kubernetes.Interface
	if cfg.K8sAPIConfig != nil {
		k8sAPIClient, err = k8sconfig.MakeClient(*cfg.K8sAPIConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create K8s API client: %w", err)
		}
	}

	return &scraperOptions{
		collectionInterval:    cfg.CollectionInterval,
		extraMetadataLabels:   cfg.ExtraMetadataLabels,
		metricGroupsToCollect: mgs,
		allNetworkInterfaces:  ifaces,
		k8sAPIClient:          k8sAPIClient,
	}, nil
}

// getMapFromSlice returns a set of kubelet.MetricGroup values from
// the provided list. Returns an err if invalid entries are encountered.
func getMapFromSlice(collect []kubelet.MetricGroup) (map[kubelet.MetricGroup]bool, error) {
	out := make(map[kubelet.MetricGroup]bool, len(collect))
	for _, c := range collect {
		if !kubelet.ValidMetricGroups[c] {
			return nil, errors.New("invalid entry in metric_groups")
		}
		out[c] = true
	}

	return out, nil
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(cfg); err != nil {
		return err
	}

	// custom unmarshalling is required to get []kubelet.MetricGroup, the default
	// unmarshaller does not correctly overwrite slices.
	if !componentParser.IsSet(metricGroupsConfig) {
		cfg.MetricGroupsToCollect = defaultMetricGroups
	}

	return nil
}

func (cfg *Config) Validate() error {
	if cfg.NodeName == "" {
		switch {
		case cfg.Metrics.K8sContainerCPUNodeUtilization.Enabled:
			return errors.New("for k8s.container.cpu.node.utilization node setting is required. Check the readme on how to set the required setting")
		case cfg.Metrics.K8sPodCPUNodeUtilization.Enabled:
			return errors.New("for k8s.pod.cpu.node.utilization node setting is required. Check the readme on how to set the required setting")
		case cfg.Metrics.K8sContainerMemoryNodeUtilization.Enabled:
			return errors.New("for k8s.container.memory.node.utilization node setting is required. Check the readme on how to set the required setting")
		case cfg.Metrics.K8sPodMemoryNodeUtilization.Enabled:
			return errors.New("for k8s.pod.memory.node.utilization node setting is required. Check the readme on how to set the required setting")
		}
	}
	return nil
}
