// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leaderelector // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/leaderelector"

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/leaderelector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
)

// createDefaultConfig returns the default configuration for the extension.
func createDefaultConfig() component.Config {
	return &Config{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
		LeaseDuration: defaultLeaseDuration,
		RenewDuration: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
	}
}

// createExtension creates the extension instance based on the configuration.
func createExtension(
	ctx context.Context,
	set extension.Settings,
	cfg component.Config,
) (extension.Extension, error) {
	baseCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config, cannot create extension leaderelector")
	}

	// Initialize k8s client in factory as doing it in extension.Start()
	// should cause race condition as http Proxy gets shared.
	client, err := baseCfg.getK8sClient()
	if err != nil {
		return nil, errors.New("failed to create k8s client")
	}

	leaseHolderID, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &leaderElectionExtension{
		config:        baseCfg,
		logger:        set.Logger,
		client:        client,
		leaseHolderId: leaseHolderID,
	}, nil
}

// NewFactory creates a new factory for your extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType(metadata.Type.String()),
		createDefaultConfig,
		createExtension,
		component.StabilityLevelDevelopment,
	)
}
