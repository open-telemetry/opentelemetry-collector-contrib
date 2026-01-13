// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ observer.Notify = (*observerHandler)(nil)

const (
	// tmpSetEndpointConfigKey denotes the observerHandler (not the user) has set an "endpoint" target field
	// in resolved configuration. Used to determine if the field should be removed when the created receiver
	// doesn't expose such a field.
	tmpSetEndpointConfigKey = "<tmp.receiver.creator.automatically.set.endpoint.field>"
)

// observerHandler manages endpoint change notifications.
type observerHandler struct {
	sync.Mutex
	config *Config
	params receiver.Settings
	// receiversByEndpointID is a map of endpoint IDs to a receiver instance.
	receiversByEndpointID receiverMap
	// nextLogsConsumer is the receiver_creator's own consumer
	nextLogsConsumer consumer.Logs
	// nextMetricsConsumer is the receiver_creator's own consumer
	nextMetricsConsumer consumer.Metrics
	// nextTracesConsumer is the receiver_creator's own consumer
	nextTracesConsumer consumer.Traces
	// runner starts and stops receiver instances.
	runner runner
}

// shutdown all receivers started at runtime.
func (obs *observerHandler) shutdown() error {
	obs.Lock()
	defer obs.Unlock()

	var errs []error

	for _, rcvr := range obs.receiversByEndpointID.Values() {
		if err := obs.runner.shutdown(rcvr); err != nil {
			// TODO: Should keep track of which receiver the error is associated with
			// but require some restructuring.
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown on %d receivers failed: %w", len(errs), multierr.Combine(errs...))
	}

	return nil
}

func (obs *observerHandler) ID() observer.NotifyID {
	return observer.NotifyID(obs.params.ID.String())
}

// OnAdd responds to endpoint add notifications.
func (obs *observerHandler) OnAdd(added []observer.Endpoint) {
	obs.Lock()
	defer obs.Unlock()

	for _, e := range added {
		env, err := e.Env()
		if err != nil {
			obs.params.Logger.Error("unable to convert endpoint to environment map", zap.String("endpoint", string(e.ID)), zap.Error(err))
			continue
		}

		obs.addReceiversForEndpoint(e, env)
	}
}

// OnRemove responds to endpoint removal notifications.
func (obs *observerHandler) OnRemove(removed []observer.Endpoint) {
	obs.Lock()
	defer obs.Unlock()

	for _, e := range removed {
		obs.removeReceiversByEndpointID(e)
	}
}

// removeReceiversByEndpointID stops and removes all receivers for the given endpoint.
// Caller must hold the lock.
func (obs *observerHandler) removeReceiversByEndpointID(e observer.Endpoint) {
	// debug log the endpoint to improve usability
	if ce := obs.params.Logger.Check(zap.DebugLevel, "handling removed endpoint"); ce != nil {
		env, err := e.Env()
		fields := []zap.Field{zap.String("endpoint_id", string(e.ID))}
		if err == nil {
			fields = append(fields, zap.Any("env", env))
		}
		ce.Write(fields...)
	}

	for _, entry := range obs.receiversByEndpointID.Get(e.ID) {
		obs.params.Logger.Info("stopping receiver", zap.String("receiver", entry.id.String()), zap.String("endpoint_id", string(e.ID)))

		if err := obs.runner.shutdown(entry.receiver); err != nil {
			obs.params.Logger.Error("failed to stop receiver", zap.String("receiver", entry.id.String()), zap.Error(err))
			continue
		}
	}
	obs.receiversByEndpointID.RemoveAll(e.ID)
}

// OnChange responds to endpoint change notifications.
// It only restarts receivers if the effective config has changed.
func (obs *observerHandler) OnChange(changed []observer.Endpoint) {
	obs.Lock()
	defer obs.Unlock()

	for _, e := range changed {
		obs.handleEndpointChange(e)
	}
}

// handleEndpointChange processes a single endpoint change, only restarting
// receivers if their effective configuration would change.
// Caller must hold the lock.
func (obs *observerHandler) handleEndpointChange(e observer.Endpoint) {
	env, err := e.Env()
	if err != nil {
		obs.params.Logger.Error("unable to convert endpoint to environment map", zap.String("endpoint", string(e.ID)), zap.Error(err))
		return
	}

	existingEntries := obs.receiversByEndpointID.Get(e.ID)

	// Check each existing receiver to see if its config would change
	var entriesToKeep []receiverEntry
	var entriesToRemove []receiverEntry

	for _, entry := range existingEntries {
		// Find the template that created this receiver
		template, found := obs.findTemplateForReceiver(entry.id, env)
		if !found {
			// Template no longer matches, remove this receiver
			entriesToRemove = append(entriesToRemove, entry)
			continue
		}

		// Resolve the new config
		resolvedConfig, resolvedDiscoveredConfig, err := obs.resolveConfig(template, env, e)
		if err != nil {
			obs.params.Logger.Error("unable to resolve config for change comparison",
				zap.String("receiver", entry.id.String()), zap.Error(err))
			entriesToRemove = append(entriesToRemove, entry)
			continue
		}

		if entry.configsEqual(resolvedConfig, resolvedDiscoveredConfig) {
			// Config unchanged, keep the receiver running
			obs.params.Logger.Debug("endpoint changed but receiver config unchanged, keeping receiver",
				zap.String("receiver", entry.id.String()),
				zap.String("endpoint_id", string(e.ID)))
			entriesToKeep = append(entriesToKeep, entry)
		} else {
			// Config changed, need to restart
			obs.params.Logger.Info("endpoint changed with new config, restarting receiver",
				zap.String("receiver", entry.id.String()),
				zap.String("endpoint_id", string(e.ID)))
			entriesToRemove = append(entriesToRemove, entry)
		}
	}

	// Shutdown receivers that need to be removed or restarted
	for _, entry := range entriesToRemove {
		if err := obs.runner.shutdown(entry.receiver); err != nil {
			obs.params.Logger.Error("failed to stop receiver", zap.String("receiver", entry.id.String()), zap.Error(err))
		}
	}

	// Update the map with kept entries
	obs.receiversByEndpointID.RemoveAll(e.ID)
	for _, entry := range entriesToKeep {
		obs.receiversByEndpointID.Put(e.ID, entry)
	}

	// Re-add receivers that were removed (they'll be recreated with new config)
	// and add any new receivers from templates that now match
	obs.addReceiversForEndpoint(e, env)
}

// findTemplateForReceiver finds the template that matches the given receiver ID.
func (obs *observerHandler) findTemplateForReceiver(id component.ID, env observer.EndpointEnv) (receiverTemplate, bool) {
	template, found := obs.config.receiverTemplates[id.String()]
	if !found {
		return receiverTemplate{}, false
	}
	// Check if the rule still matches
	matches, err := template.rule.eval(env)
	if err == nil && matches {
		return template, true
	}
	return receiverTemplate{}, false
}

// resolveConfig expands the receiver template's config using the endpoint environment.
//
// It returns two configs that are later merged by the runner:
//   - resolvedUserConfig: the user's template config with backtick expressions expanded
//   - resolvedDiscoveredConfig: auto-discovered values from the endpoint (e.g., endpoint target)
//
// These are kept separate so the runner can validate whether auto-discovered fields
// (like "endpoint") are actually supported by the receiver before merging them.
func (obs *observerHandler) resolveConfig(template receiverTemplate, env observer.EndpointEnv, e observer.Endpoint) (resolvedUserConfig, resolvedDiscoveredConfig userConfigMap, err error) {
	resolvedUserConfig, err = expandConfig(template.config, env)
	if err != nil {
		return nil, nil, err
	}

	// Build the "discovered" config which contains values derived from the endpoint
	// rather than from the user's template. Currently this is just the endpoint target.
	//
	// If the user didn't explicitly set an "endpoint" field in their config, we auto-populate
	// it from the discovered endpoint's Target. The tmpSetEndpointConfigKey flag tells
	// mergeTemplatedAndDiscoveredConfigs (in runner.go) that we auto-set this value, so it
	// can validate whether the receiver actually supports an "endpoint" config field and
	// remove it if not (avoiding "unknown field" errors for receivers without that field).
	discoveredCfg := userConfigMap{}
	if _, ok := resolvedUserConfig[endpointConfigKey]; !ok {
		discoveredCfg[endpointConfigKey] = e.Target
		discoveredCfg[tmpSetEndpointConfigKey] = struct{}{}
	}

	resolvedDiscoveredConfig, err = expandConfig(discoveredCfg, env)
	if err != nil {
		return nil, nil, err
	}

	return resolvedUserConfig, resolvedDiscoveredConfig, nil
}

// addReceiversForEndpoint starts receivers for an endpoint, skipping any that already exist.
// Caller must hold the lock.
func (obs *observerHandler) addReceiversForEndpoint(e observer.Endpoint, env observer.EndpointEnv) {
	existingIDs := make(map[component.ID]bool)
	for _, entry := range obs.receiversByEndpointID.Get(e.ID) {
		existingIDs[entry.id] = true
	}

	if obs.config.Discovery.Enabled {
		builder := createK8sHintsBuilder(obs.config.Discovery, obs.params.Logger)
		subreceiverTemplate, err := builder.createReceiverTemplateFromHints(env)
		if err != nil {
			obs.params.Logger.Error("could not extract configurations from K8s hints' annotations", zap.Error(err))
			return
		}
		if subreceiverTemplate != nil {
			if !existingIDs[subreceiverTemplate.id] {
				obs.params.Logger.Debug("adding K8s hinted receiver", zap.Any("subreceiver", subreceiverTemplate))
				obs.startReceiver(*subreceiverTemplate, env, e)
			}
			return // When hints create a receiver, skip regular templates
		}
		// Fall through to process regular templates if no hint-based receiver
	}

	for _, template := range obs.config.receiverTemplates {
		if existingIDs[template.id] {
			continue // Already running
		}
		if matches, err := template.rule.eval(env); err != nil {
			obs.params.Logger.Error("failed matching rule", zap.String("rule", template.Rule), zap.Error(err))
			continue
		} else if !matches {
			continue
		}
		obs.startReceiver(template, env, e)
	}
}

func (obs *observerHandler) startReceiver(template receiverTemplate, env observer.EndpointEnv, e observer.Endpoint) {
	obs.params.Logger.Debug("expanding the following template config",
		zap.String("name", template.id.String()),
		zap.String("endpoint", e.Target),
		zap.String("endpoint_id", string(e.ID)),
		zap.Any("config", template.config))

	resolvedConfig, resolvedDiscoveredConfig, err := obs.resolveConfig(template, env, e)
	if err != nil {
		obs.params.Logger.Error("unable to resolve template config", zap.String("receiver", template.id.String()), zap.Error(err))
		return
	}

	resAttrs := map[string]string{}
	for k, v := range template.ResourceAttributes {
		strVal, ok := v.(string)
		if !ok {
			obs.params.Logger.Info(fmt.Sprintf("ignoring unsupported `resource_attributes` %q value %v", k, v))
			continue
		}
		resAttrs[k] = strVal
	}

	// Adds default and/or configured resource attributes (e.g. k8s.pod.uid) to resources
	// as telemetry is emitted.
	var consumer *enhancingConsumer
	if consumer, err = newEnhancingConsumer(
		obs.config.ResourceAttributes,
		resAttrs,
		env,
		e,
		obs.nextLogsConsumer,
		obs.nextMetricsConsumer,
		obs.nextTracesConsumer,
	); err != nil {
		obs.params.Logger.Error("failed creating resource enhancer", zap.String("receiver", template.id.String()), zap.Error(err))
		return
	}

	filterConsumerSignals(consumer, template.signals)

	// short-circuit if no consumers are set
	if consumer.metrics == nil && consumer.logs == nil && consumer.traces == nil {
		return
	}

	obs.params.Logger.Info("starting receiver",
		zap.String("name", template.id.String()),
		zap.String("endpoint", e.Target),
		zap.String("endpoint_id", string(e.ID)),
		zap.Any("config", resolvedConfig))

	var rcvr component.Component
	if rcvr, err = obs.runner.start(
		receiverConfig{
			id:         template.id,
			config:     resolvedConfig,
			endpointID: e.ID,
		},
		resolvedDiscoveredConfig,
		consumer,
	); err != nil {
		obs.params.Logger.Error("failed to start receiver", zap.String("receiver", template.id.String()), zap.Error(err))
		return
	}

	obs.receiversByEndpointID.Put(e.ID, receiverEntry{
		receiver:                 rcvr,
		id:                       template.id,
		resolvedConfig:           resolvedConfig,
		resolvedDiscoveredConfig: resolvedDiscoveredConfig,
	})
}

func filterConsumerSignals(consumer *enhancingConsumer, signals receiverSignals) {
	if !signals.metrics {
		consumer.metrics = nil
	}
	if !signals.logs {
		consumer.logs = nil
	}
	if !signals.traces {
		consumer.traces = nil
	}
}
