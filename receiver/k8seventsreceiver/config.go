// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines configuration for kubernetes events receiver.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// List of ‘namespaces’ to collect events from.
	Namespaces []string `mapstructure:"namespaces"`

	// Storage is the ID of the storage extension to use for resource version persistence.
	// When set, the receiver will persist the latest resource version and resume from it on restart,
	// preventing duplicate events. Only valid for watch mode (which is the only mode this receiver uses).
	Storage *component.ID `mapstructure:"storage"`

	K8sLeaderElector *component.ID `mapstructure:"k8s_leader_elector"`

	// DedupInterval controls throttling of MODIFIED watch events per event UID.
	//   positive: emit MODIFIED only after interval has elapsed since the last emit for that UID
	//   0 (default): no throttling — current behavior, backward compatible
	//   negative: drop all MODIFIED events
	DedupInterval time.Duration `mapstructure:"dedup_interval"`

	// For mocking
	makeClient        func(apiConf k8sconfig.APIConfig) (k8s.Interface, error)
	makeDynamicClient func(apiConf k8sconfig.APIConfig) (dynamic.Interface, error)
}

func (cfg *Config) Validate() error {
	return cfg.APIConfig.Validate()
}

func (cfg *Config) getK8sClient() (k8s.Interface, error) {
	if cfg.makeClient == nil {
		cfg.makeClient = k8sconfig.MakeClient
	}
	return cfg.makeClient(cfg.APIConfig)
}

func (cfg *Config) getDynamicClient() (dynamic.Interface, error) {
	if cfg.makeDynamicClient == nil {
		cfg.makeDynamicClient = k8sconfig.MakeDynamicClient
	}
	return cfg.makeDynamicClient(cfg.APIConfig)
}
