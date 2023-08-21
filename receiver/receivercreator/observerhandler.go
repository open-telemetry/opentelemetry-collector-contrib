// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal"
)

var (
	_ observer.Notify = (*observerHandler)(nil)
)

const (
	// tmpSetEndpointConfigKey denotes the observerHandler (not the user) has set an "endpoint" target field
	// in resolved configuration. Used to determine if the field should be removed when the created receiver
	// doesn't expose such a field.
	tmpSetEndpointConfigKey = "<tmp.receiver.creator.automatically.set.endpoint.field>"
	// tmpPropertyCreatedTemplate denotes that a receiver template was created from endpoint properties alone.
	// Used to determine if a template shouldn't be processed in error conditions
	tmpPropertyCreatedTemplate = "<tmp.property.created.template>"
)

// observerHandler manages endpoint change notifications.
type observerHandler struct {
	sync.Mutex
	config *Config
	params receiver.CreateSettings
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
	logger *zap.Logger
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
		details := e.Details
		var env observer.EndpointEnv
		var err error
		if env, err = e.Env(); err != nil {
			obs.logger.Error("unable to convert endpoint to environment map", zap.String("endpoint", string(e.ID)), zap.Error(err))
			continue
		}

		obs.logger.Debug("handling added endpoint", zap.Any("env", env))

		for _, template := range obs.updatedReceiverTemplatesFromEndpointProperties(details) {
			if matches, ee := template.rule.eval(env); ee != nil {
				obs.logger.Error("failed matching rule", zap.String("rule", template.Rule), zap.Error(ee))
				continue
			} else if !matches {
				obs.logger.Debug("rule doesn't match endpoint", zap.String("endpoint", string(e.ID)), zap.String("receiver", template.id.String()), zap.String("rule", template.Rule))
				continue
			}

			obs.logger.Info("starting receiver",
				zap.String("name", template.id.String()),
				zap.String("endpoint", e.Target),
				zap.String("endpoint_id", string(e.ID)))

			resolvedConfig, err := expandConfig(template.config, env)
			if err != nil {
				obs.logger.Error("unable to resolve template config", zap.String("receiver", template.id.String()), zap.Error(err))
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
				obs.logger.Error("unable to resolve discovered config", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
			}

			resAttrs := map[string]string{}
			for k, v := range template.ResourceAttributes {
				strVal, ok := v.(string)
				if !ok {
					obs.logger.Info(fmt.Sprintf("ignoring unsupported `resource_attributes` %q value %v", k, v))
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
				obs.logger.Error("failed creating resource enhancer", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
			}

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
				obs.logger.Error("failed to start receiver", zap.String("receiver", template.id.String()), zap.Error(err))
				continue
			}

			obs.receiversByEndpointID.Put(e.ID, receiver)
		}
	}
}

// OnRemove responds to endpoint removal notifications.
func (obs *observerHandler) OnRemove(removed []observer.Endpoint) {
	obs.Lock()
	defer obs.Unlock()

	for _, e := range removed {
		// debug log the endpoint to improve usability
		if ce := obs.logger.Check(zap.DebugLevel, "handling removed endpoint"); ce != nil {
			env, err := e.Env()
			fields := []zap.Field{zap.String("endpoint_id", string(e.ID))}
			if err == nil {
				fields = append(fields, zap.Any("env", env))
			}
			ce.Write(fields...)
		}

		for _, rcvr := range obs.receiversByEndpointID.Get(e.ID) {
			obs.logger.Info("stopping receiver", zap.Reflect("receiver", rcvr), zap.String("endpoint_id", string(e.ID)))

			if err := obs.runner.shutdown(rcvr); err != nil {
				obs.logger.Error("failed to stop receiver", zap.Reflect("receiver", rcvr), zap.Error(err))
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

// templateToUpdate contains endpoint properties from an endpoint env and the user configured receiverTemplate to enhance
type templateToUpdate struct {
	propsConf *confmap.Conf
	template  *receiverTemplate
}

func (obs *observerHandler) updatedReceiverTemplatesFromEndpointProperties(details observer.EndpointDetails) []receiverTemplate {
	// final templates to return for evaluation
	var receiverTemplates []receiverTemplate
	var properties *confmap.Conf
	if obs.config.AcceptEndpointProperties {
		properties = internal.PropertyConfFromEndpointEnv(details, obs.logger)
	}
	if properties == nil {
		for _, template := range obs.config.receiverTemplates {
			receiverTemplates = append(receiverTemplates, template)
		}
		return receiverTemplates
	}

	// forward inapplicable templates without enhancement
	propertiesSM := properties.ToStringMap()
	for receiver, template := range obs.config.receiverTemplates {
		if _, ok := propertiesSM[receiver]; !ok {
			obs.logger.Debug("receiver template unchanged by endpoint properties", zap.String("receiver", receiver))
			receiverTemplates = append(receiverTemplates, template)
		}
	}

	templatesToUpdate := obs.getTemplatesToUpdateWithProperties(receiverTemplates, properties)

	for _, toUpdate := range templatesToUpdate {
		rProps := toUpdate.propsConf
		rPropsSM := rProps.ToStringMap()
		template := *toUpdate.template

		var propertyCreated bool
		if _, propertyCreated = template.config[tmpPropertyCreatedTemplate]; propertyCreated {
			delete(template.config, tmpPropertyCreatedTemplate)
		}

		if propsConf, err := rProps.Sub(configKey); err != nil {
			obs.logger.Info("failed creating conf property values", zap.String("receiver", template.id.String()), zap.Error(err))
		} else {
			templateConf := confmap.NewFromStringMap(template.config)
			if err = templateConf.Merge(propsConf); err != nil {
				obs.logger.Info("failed merging conf property values", zap.String("receiver", template.id.String()), zap.Error(err))
			} else {
				template.config = templateConf.ToStringMap()
			}
		}

		if propRule, hasRule := rPropsSM[internal.RuleType]; hasRule {
			if receiverRule, ok := propRule.(string); !ok || receiverRule == "" {
				obs.logger.Debug(
					"invalid property rule",
					zap.String("receiver", template.id.String()),
					zap.Any("rule", propRule),
					zap.String("type", fmt.Sprintf("%T", propRule)),
				)
				if propertyCreated || template.Rule == "" {
					// there's nothing we can do without a valid rule for partial or property-created templates
					continue
				}
			} else {
				if exprRule, e := newRule(receiverRule); e != nil {
					obs.logger.Info("failed determining valid rule for property-created template", zap.String("receiver", template.id.String()), zap.Error(e))
					if propertyCreated || template.Rule == "" {
						// there's nothing we can do without a valid rule for property-created templates
						continue
					}
				} else {
					template.Rule = receiverRule
					template.rule = exprRule
				}
			}
		} else if propertyCreated {
			obs.logger.Info("missing rule for property-created template", zap.String("receiver", template.id.String()))
			continue
		}

		if _, hasResourceAttrs := rPropsSM["resource_attributes"]; hasResourceAttrs {
			if propAttrConf, e := rProps.Sub("resource_attributes"); e != nil {
				obs.logger.Info("failed retrieving property-provided resource attributes", zap.String("receiver", template.id.String()), zap.Error(e))
			} else {
				for k, v := range propAttrConf.ToStringMap() {
					template.ResourceAttributes[k] = v
				}
			}
		}
		receiverTemplates = append(receiverTemplates, template)
	}
	return receiverTemplates
}

func (obs *observerHandler) getTemplatesToUpdateWithProperties(receiverTemplates []receiverTemplate, properties *confmap.Conf) []templateToUpdate {
	var templatesToUpdate []templateToUpdate
	for receiver := range properties.ToStringMap() {
		rProps, err := properties.Sub(receiver)
		if err != nil {
			obs.logger.Info("failed extracting expected receiver properties", zap.String("receiver", receiver), zap.Error(err))
			if template, ok := obs.config.receiverTemplates[receiver]; ok {
				// fallback by not modifying existing template
				receiverTemplates = append(receiverTemplates, template)
			}
			continue
		}

		var template receiverTemplate
		var ok bool
		if template, ok = obs.config.receiverTemplates[receiver]; ok {
			// copy because we will be modifying from properties for only this endpoint
			template = template.copy()
			obs.logger.Debug("existing receiver template updated by endpoint properties", zap.String("receiver", receiver))
		} else {
			if template, err = newReceiverTemplate(receiver, map[string]any{tmpPropertyCreatedTemplate: struct{}{}}); err != nil {
				obs.logger.Info("failed creating new receiver template for property values", zap.String("receiver", receiver), zap.Error(err))
				continue
			}
			obs.logger.Debug("receiver template created by endpoint properties", zap.String("receiver", receiver))
		}

		templatesToUpdate = append(templatesToUpdate, templateToUpdate{
			propsConf: rProps, template: &template,
		})
	}
	return templatesToUpdate
}
