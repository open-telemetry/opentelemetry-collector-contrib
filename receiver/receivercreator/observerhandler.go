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
		var env observer.EndpointEnv
		var err error
		if env, err = e.Env(); err != nil {
			obs.params.Logger.Error("unable to convert endpoint to environment map", zap.String("endpoint", string(e.ID)), zap.Error(err))
			continue
		}

		if obs.config.Discovery.Enabled {
			builder := createK8sHintsBuilder(obs.config.Discovery, obs.params.Logger)
			subreceiverTemplate, err := builder.createReceiverTemplateFromHints(env)
			if err != nil {
				obs.params.Logger.Error("could not extract configurations from K8s hints' annotations", zap.Error(err))
				break
			}
			if subreceiverTemplate != nil {
				obs.params.Logger.Debug("adding K8s hinted receiver", zap.Any("subreceiver", subreceiverTemplate))
				obs.startReceiver(*subreceiverTemplate, env, e)
				continue
			}
		}

		for _, template := range obs.config.receiverTemplates {
			if matches, err := template.rule.eval(env); err != nil {
				obs.params.Logger.Error("failed matching rule", zap.String("rule", template.Rule), zap.Error(err))
				continue
			} else if !matches {
				continue
			}
			obs.startReceiver(template, env, e)
		}
	}
}

// OnRemove responds to endpoint removal notifications.
func (obs *observerHandler) OnRemove(removed []observer.Endpoint) {
	obs.Lock()
	defer obs.Unlock()

	for _, e := range removed {
		// debug log the endpoint to improve usability
		if ce := obs.params.Logger.Check(zap.DebugLevel, "handling removed endpoint"); ce != nil {
			env, err := e.Env()
			fields := []zap.Field{zap.String("endpoint_id", string(e.ID))}
			if err == nil {
				fields = append(fields, zap.Any("env", env))
			}
			ce.Write(fields...)
		}

		for _, rcvr := range obs.receiversByEndpointID.Get(e.ID) {
			obs.params.Logger.Info("stopping receiver", zap.Reflect("receiver", rcvr), zap.String("endpoint_id", string(e.ID)))

			if err := obs.runner.shutdown(rcvr); err != nil {
				obs.params.Logger.Error("failed to stop receiver", zap.Reflect("receiver", rcvr), zap.Error(err))
				continue
			}
		}
		obs.receiversByEndpointID.RemoveAll(e.ID)
	}
}

// OnChange responds to endpoint change notifications.
func (obs *observerHandler) OnChange(changed []observer.Endpoint) {
	// TODO: optimize to only restart if effective config has changed.
	obs.OnRemove(changed)
	obs.OnAdd(changed)
}

func (obs *observerHandler) startReceiver(template receiverTemplate, env observer.EndpointEnv, e observer.Endpoint) {
	resolvedConfig, err := expandConfig(template.config, env)
	if err != nil {
		obs.params.Logger.Error("unable to resolve template config", zap.String("receiver", template.id.String()), zap.Error(err))
		return
	}

	discoveredCfg := userConfigMap{}
	// If user didn't set endpoint set to default value as well as
	// flag indicating we've done this for later validation.
	if _, ok := resolvedConfig[endpointConfigKey]; !ok {
		discoveredCfg[endpointConfigKey] = e.Target
		discoveredCfg[tmpSetEndpointConfigKey] = struct{}{}
	}

	// Though not necessary with contrib provided observers, nothing is stopping custom
	// ones from using expr in their Target values.
	discoveredConfig, err := expandConfig(discoveredCfg, env)
	if err != nil {
		obs.params.Logger.Error("unable to resolve discovered config", zap.String("receiver", template.id.String()), zap.Error(err))
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
		zap.Any("config", template.config))

	var receiver component.Component
	if receiver, err = obs.runner.start(
		receiverConfig{
			id:         template.id,
			config:     resolvedConfig,
			endpointID: e.ID,
		},
		discoveredConfig,
		consumer,
	); err != nil {
		obs.params.Logger.Error("failed to start receiver", zap.String("receiver", template.id.String()), zap.Error(err))
		return
	}
	obs.receiversByEndpointID.Put(e.ID, receiver)
}

func filterConsumerSignals(consumer *enhancingConsumer, signals receiverSignals) {
	if !signals.metrics {
		consumer.metrics = nil
	}
	if !signals.logs {
		consumer.logs = nil
	}
	if !signals.metrics {
		consumer.traces = nil
	}
}
