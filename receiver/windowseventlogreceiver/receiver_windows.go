// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/discovery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/sidcache"
)

// getDomainControllersRemoteConfig is the function used to discover domain controllers.
// It is a variable to allow mocking in tests.
var getDomainControllersRemoteConfig = discovery.GetJoinedDomainControllersRemoteConfig

// createLogsReceiver creates a logs receiver with SID enrichment support
func createLogsReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	receiverCfg := cfg.(*WindowsLogConfig)

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	// Create SID cache if enabled
	var cache sidcache.Cache
	if receiverCfg.ResolveSIDs.Enabled {
		cacheConfig := sidcache.Config{
			Size: receiverCfg.ResolveSIDs.CacheSize,
			TTL:  receiverCfg.ResolveSIDs.CacheTTL,
		}

		cache, err = sidcache.New(cacheConfig)
		if err != nil {
			return nil, err
		}

		set.Logger.Info("SID resolution enabled",
			zap.Uint("cache_size", cacheConfig.Size),
			zap.Duration("cache_ttl", cacheConfig.TTL))
	}

	// Inject telemetry bridge so the operator can record per-event and per-channel metrics.
	receiverCfg.InputConfig.Telemetry = &receiverWindowsTelemetry{tb: telemetryBuilder, cfg: receiverCfg.Telemetry.Metrics}

	// Wrap the consumer with SID enrichment, then lag tracking.
	enrichedConsumer := newSIDEnrichingConsumer(nextConsumer, cache, set.Logger)
	lagConsumer, err := newLagTrackingConsumer(enrichedConsumer, telemetryBuilder, receiverCfg.InputConfig.Channel, receiverCfg.Telemetry.Metrics)
	if err != nil {
		return nil, err
	}

	stanzaFactory := adapter.NewFactory(receiverType{}, metadata.LogsStability, xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType))

	if metadata.DomainControllersAutodiscoveryFeatureGate.IsEnabled() && receiverCfg.DiscoverDomainControllers {
		remoteConfigs, err := getDomainControllersRemoteConfig(set.Logger, receiverCfg.InputConfig.Remote.Username, receiverCfg.InputConfig.Remote.Password)
		if err != nil {
			return nil, fmt.Errorf("domain controller discovery failed: %w", err)
		}

		receivers := make([]receiver.Logs, 0, len(remoteConfigs))
		for _, rc := range remoteConfigs {
			dcCfg := *receiverCfg
			dcCfg.InputConfig.Remote = windows.RemoteConfig{
				Server:   rc.Server,
				Username: rc.Username,
				Password: rc.Password,
				Domain:   rc.Domain,
			}
			r, err := stanzaFactory.CreateLogs(ctx, set, &dcCfg, lagConsumer)
			if err != nil {
				return nil, fmt.Errorf("failed to create receiver for domain controller %q: %w", rc.Server, err)
			}
			receivers = append(receivers, r)
		}
		return &multiLogsReceiver{receivers: receivers}, nil
	}

	// Create the underlying Stanza receiver with the lag-tracking consumer.
	return stanzaFactory.CreateLogs(ctx, set, cfg, lagConsumer)
}

type multiLogsReceiver struct {
	receivers []receiver.Logs
}

func (m *multiLogsReceiver) Start(ctx context.Context, host component.Host) error {
	started := make([]receiver.Logs, 0, len(m.receivers))
	for _, r := range m.receivers {
		if err := r.Start(ctx, host); err != nil {
			// Shut down already-started receivers before returning the error.
			var shutdownErrs []error
			for _, s := range started {
				if shutdownErr := s.Shutdown(ctx); shutdownErr != nil {
					shutdownErrs = append(shutdownErrs, shutdownErr)
				}
			}
			return errors.Join(append([]error{err}, shutdownErrs...)...)
		}
		started = append(started, r)
	}
	return nil
}

func (m *multiLogsReceiver) Shutdown(ctx context.Context) error {
	var errs []error
	for _, r := range m.receivers {
		if err := r.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// receiverType implements adapter.LogReceiverType
// to create a file tailing receiver
type receiverType struct{}

var _ adapter.LogReceiverType = (*receiverType)(nil)

// Type is the receiver type
func (receiverType) Type() component.Type {
	return metadata.Type
}

// CreateDefaultConfig creates a config with type and version
func (receiverType) CreateDefaultConfig() component.Config {
	return createDefaultConfig()
}

// BaseConfig gets the base config from config, for now
func (receiverType) BaseConfig(cfg component.Config) adapter.BaseConfig {
	return cfg.(*WindowsLogConfig).BaseConfig
}

// InputConfig unmarshals the input operator
func (receiverType) InputConfig(cfg component.Config) operator.Config {
	return operator.NewConfig(&cfg.(*WindowsLogConfig).InputConfig)
}
