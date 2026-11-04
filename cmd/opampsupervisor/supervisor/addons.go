// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
)

// addonConfigs extracts addon configuration from a remote config. An entry
// configures an addon when its role is addonConfigRole; its key is the addon's
// package name, "<type>:<name>", so the same key identifies the addon in
// PackagesAvailable, PackageStatuses and the config map.
//
// The role, not the key's shape, decides what is addon configuration. Config
// map keys are free-form, and a Collector config entry whose key happened to
// look like "file:base.yaml" must not be taken for addon configuration.
func addonConfigs(remoteConfig *protobufs.AgentRemoteConfig) map[string][]byte {
	configs := map[string][]byte{}
	for name, entry := range remoteConfig.GetConfig().GetConfigMap() {
		if entry.GetRole() == addonConfigRole {
			configs[name] = entry.GetBody()
		}
	}
	return configs
}

// reconcileAddons configures and starts installed addons against the latest
// remote config, returning whether any of them needs the Collector restarted.
func (s *Supervisor) reconcileAddons(ctx context.Context) bool {
	if s.addons == nil {
		return false
	}

	effect, err := s.addons.Reconcile(ctx, addonConfigs(s.remoteConfig.Load()))
	if err != nil {
		s.telemetrySettings.Logger.Error("Could not reconcile all addons", zap.Error(err))
	}
	return effect.RestartAgent
}

// syncPackages processes a PackagesAvailable offer in the background and
// reconciles addons once it completes.
//
// restartPending is true when the same ServerToAgent message also changed the
// Collector's configuration. That restart is held back until the packages are
// in place, so that one message causes one Collector restart that observes
// both changes, instead of one restart with the old addons followed by a
// second one with the new.
func (s *Supervisor) syncPackages(ctx context.Context, syncer types.PackagesSyncer, restartPending bool) {
	if s.packages == nil {
		s.telemetrySettings.Logger.Warn("Received a package offer but the packages capability is disabled. Ignoring.")
		if restartPending {
			s.signalAgentRestart()
		}
		return
	}

	if err := syncer.Sync(ctx); err != nil {
		s.telemetrySettings.Logger.Error("Could not sync packages", zap.Error(err))
		if restartPending {
			s.signalAgentRestart()
		}
		return
	}

	go func() {
		// Wait for Done only. The syncer closes it on every path, including a
		// failed or cancelled sync. ctx must not end the wait: over WebSocket it
		// is the connection's context, so a dropped connection would otherwise
		// skip the reconciliation and lose the held-back Collector restart.
		<-syncer.Done()

		// The remote config may have arrived with this message or before the
		// packages it configures were installed. Reconciling now applies it.
		// It uses the Supervisor's context for the same reason as above.
		if s.reconcileAddons(s.runCtx) || restartPending {
			s.signalAgentRestart()
		}
	}()
}

// signalAgentRestart asks the agent loop to restart the Collector. The signal
// is coalesced, so concurrent requests cause a single restart.
func (s *Supervisor) signalAgentRestart() {
	select {
	case s.hasNewConfig <- struct{}{}:
	default:
	}
}

// startAddons loads the addons installed by a previous run and reconciles
// them against the persisted remote config. It runs before the Collector is
// first started, so addons the Collector reads are in place when it starts and
// their restart requests can be ignored.
func (s *Supervisor) startAddons(ctx context.Context) {
	if s.packages == nil {
		return
	}
	if err := s.packages.loadInstalledAddons(ctx); err != nil {
		s.telemetrySettings.Logger.Error("Could not load previously installed addons", zap.Error(err))
	}
	_ = s.reconcileAddons(ctx)
}

// stopAddons shuts down every addon. Called during shutdown so that the
// Supervisor does not leave orphaned children behind, which is the failure
// mode that made in-Collector subprocess management unworkable.
func (s *Supervisor) stopAddons(ctx context.Context) {
	if s.addons == nil {
		return
	}
	if err := s.addons.Shutdown(ctx); err != nil {
		s.telemetrySettings.Logger.Error("Could not shut down all addons", zap.Error(err))
	}
}
