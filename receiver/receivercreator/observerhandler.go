// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	_ observer.Notify = (*observerHandler)(nil)
)

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
	params receiver.CreateSettings
	// receiversByEndpointID is a map of endpoint IDs to a receiver instance.
	receiversByEndpointID receiverMap
	// nextConsumer is the receiver_creator's own consumer
	nextConsumer consumer.Metrics
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
			obs.params.TelemetrySettings.Logger.Error("unable to convert endpoint to environment map", zap.String("endpoint", string(e.ID)), zap.Error(err))
			continue
		}

		obs.params.TelemetrySettings.Logger.Debug("handling added endpoint", zap.Any("env", env))

		for _, template := range obs.config.receiverTemplates {
			if matches, err := template.rule.eval(env); err != nil {
				obs.params.TelemetrySettings.Logger.Error("failed matching rule", zap.String("rule", template.Rule), zap.Error(err))
				continue
			} else if !matches {
				continue
			}

			obs.params.TelemetrySettings.Logger.Info("starting receiver",
				zap.String("name", template.id.String()),
				zap.String("endpoint", e.Target),
				zap.String("endpoint_id", string(e.ID)))

			resolvedConfig, err := expandConfig(template.config, env)
			if err != nil {
				obs.params.TelemetrySettings.Logger.Error("unable to resolve template config", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
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
				obs.params.TelemetrySettings.Logger.Error("unable to resolve discovered config", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
			}

			resAttrs := map[string]string{}
			for k, v := range template.ResourceAttributes {
				strVal, ok := v.(string)
				if !ok {
					obs.params.TelemetrySettings.Logger.Info(fmt.Sprintf("ignoring unsupported `resource_attributes` %q value %v", k, v))
					continue
				}
				resAttrs[k] = strVal
			}

			// Adds default and/or configured resource attributes (e.g. k8s.pod.uid) to resources
			// as telemetry is emitted.
			resourceEnhancer, err := newResourceEnhancer(
				obs.config.ResourceAttributes,
				resAttrs,
				env,
				e,
				obs.nextConsumer,
			)

			if err != nil {
				obs.params.TelemetrySettings.Logger.Error("failed creating resource enhancer", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
			}

			rcvr, err := obs.runner.start(
				receiverConfig{
					id:         template.id,
					config:     resolvedConfig,
					endpointID: e.ID,
				},
				discoveredConfig,
				resourceEnhancer,
			)

			if err != nil {
				obs.params.TelemetrySettings.Logger.Error("failed to start receiver", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
			}

			obs.receiversByEndpointID.Put(e.ID, rcvr)
		}
	}
}

// OnRemove responds to endpoint removal notifications.
func (obs *observerHandler) OnRemove(removed []observer.Endpoint) {
	obs.Lock()
	defer obs.Unlock()

	for _, e := range removed {
		// debug log the endpoint to improve usability
		if ce := obs.params.TelemetrySettings.Logger.Check(zap.DebugLevel, "handling removed endpoint"); ce != nil {
			env, err := e.Env()
			fields := []zap.Field{zap.String("endpoint_id", string(e.ID))}
			if err == nil {
				fields = append(fields, zap.Any("env", env))
			}
			ce.Write(fields...)
		}

		for _, rcvr := range obs.receiversByEndpointID.Get(e.ID) {
			obs.params.TelemetrySettings.Logger.Info("stopping receiver", zap.Reflect("receiver", rcvr), zap.String("endpoint_id", string(e.ID)))

			if err := obs.runner.shutdown(rcvr); err != nil {
				obs.params.TelemetrySettings.Logger.Error("failed to stop receiver", zap.Reflect("receiver", rcvr), zap.Error(err))
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
