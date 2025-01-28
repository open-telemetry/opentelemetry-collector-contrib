// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/leaderelector"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config is the configuration for the leader elector extension.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`
	LeaseName           string        `mapstructure:"lease_name"`
	LeaseNamespace      string        `mapstructure:"lease_namespace"`
	LeaseDuration       time.Duration `mapstructure:"lease_duration"`
	RenewDuration       time.Duration `mapstructure:"renew_deadline"`
	RetryPeriod         time.Duration `mapstructure:"retry_period"`
	makeClient          func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error)
}

func (cfg *Config) getK8sClient() (kubernetes.Interface, error) {
	if cfg.makeClient == nil {
		cfg.makeClient = k8sconfig.MakeClient
	}
	return cfg.makeClient(cfg.APIConfig)
}

var _ component.Config = (*Config)(nil)
