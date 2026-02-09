// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ observer.Notify = (*observerHandler)(nil)

const (
	// tmpSetEndpointConfigKey denotes the observerHandler (not the user) has set an "endpoint" target field
	// in resolved configuration. Used to determine if the field should be removed when the created exporter
	// doesn't expose such a field.
	tmpSetEndpointConfigKey = "<tmp.exporter.creator.automatically.set.endpoint.field>"
	endpointConfigKey       = "endpoint"
)

// observerHandler manages endpoint change notifications.
type observerHandler struct {
	sync.Mutex
	config              *Config
	params              exporter.Settings
	exportersByEndpoint map[observer.EndpointID]component.Component
	router              *telemetryRouter
	runner              runner
}

// ID implements observer.Notify interface.
func (oh *observerHandler) ID() observer.NotifyID {
	return observer.NotifyID(oh.params.ID.String())
}

// OnAdd responds to endpoint add notifications.
func (oh *observerHandler) OnAdd(added []observer.Endpoint) {
	oh.Lock()
	defer oh.Unlock()

	for _, e := range added {
		endpointType := "unknown"
		if e.Details != nil {
			endpointType = string(e.Details.Type())
		}
		oh.params.Logger.Info("observed resource added",
			zap.String("endpoint_id", string(e.ID)),
			zap.String("endpoint_target", e.Target),
			zap.String("endpoint_type", endpointType),
		)

		var env observer.EndpointEnv
		var err error
		if env, err = e.Env(); err != nil {
			oh.params.Logger.Error("unable to convert endpoint to environment map", zap.String("endpoint", string(e.ID)), zap.Error(err))
			continue
		}

		// Check each exporter template to see if it matches this endpoint
		matched := false
		for _, template := range oh.config.exporterTemplates {
			if matches, err := template.rule.eval(env); err != nil {
				oh.params.Logger.Error("failed matching rule", zap.String("rule", template.Rule), zap.Error(err))
				continue
			} else if !matches {
				continue
			}
			matched = true
			oh.startExporter(template, env, e)
		}

		if !matched {
			oh.params.Logger.Debug("endpoint did not match any exporter template",
				zap.String("endpoint_id", string(e.ID)),
				zap.String("endpoint_target", e.Target),
			)
		}
	}
}

// OnRemove responds to endpoint removal notifications.
func (oh *observerHandler) OnRemove(removed []observer.Endpoint) {
	oh.Lock()
	defer oh.Unlock()

	for _, e := range removed {
		endpointType := "unknown"
		if e.Details != nil {
			endpointType = string(e.Details.Type())
		}
		oh.params.Logger.Info("observed resource removed",
			zap.String("endpoint_id", string(e.ID)),
			zap.String("endpoint_target", e.Target),
			zap.String("endpoint_type", endpointType),
		)

		if exp, exists := oh.exportersByEndpoint[e.ID]; exists {
			oh.params.Logger.Info("stopping exporter", zap.String("endpoint_id", string(e.ID)))

			if err := oh.runner.shutdown(exp); err != nil {
				oh.params.Logger.Error("failed to stop exporter", zap.String("endpoint_id", string(e.ID)), zap.Error(err))
			}

			// Remove from router
			oh.router.RemoveExporter(e.ID)
			delete(oh.exportersByEndpoint, e.ID)
		} else {
			oh.params.Logger.Debug("endpoint removed but no exporter was created for it",
				zap.String("endpoint_id", string(e.ID)),
				zap.String("endpoint_target", e.Target),
			)
		}
	}
}

// OnChange responds to endpoint change notifications.
func (oh *observerHandler) OnChange(changed []observer.Endpoint) {
	oh.Lock()
	defer oh.Unlock()

	for _, e := range changed {
		endpointType := "unknown"
		if e.Details != nil {
			endpointType = string(e.Details.Type())
		}
		oh.params.Logger.Info("observed resource updated",
			zap.String("endpoint_id", string(e.ID)),
			zap.String("endpoint_target", e.Target),
			zap.String("endpoint_type", endpointType),
		)
	}

	// TODO: optimize to only restart if effective config has changed.
	oh.OnRemove(changed)
	oh.OnAdd(changed)
}

// shutdown stops all sub-exporters.
func (oh *observerHandler) shutdown() error {
	oh.Lock()
	defer oh.Unlock()

	var errs []error

	for endpointID, exp := range oh.exportersByEndpoint {
		if err := oh.runner.shutdown(exp); err != nil {
			errs = append(errs, fmt.Errorf("endpoint %q: %w", endpointID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown on %d exporters failed: %w", len(errs), multierr.Combine(errs...))
	}

	return nil
}

func (oh *observerHandler) startExporter(template exporterTemplate, env observer.EndpointEnv, e observer.Endpoint) {
	oh.params.Logger.Debug("expanding the following template config",
		zap.String("name", template.id.String()),
		zap.String("endpoint", e.Target),
		zap.String("endpoint_id", string(e.ID)),
		zap.Any("config", template.config))
	resolvedConfig, err := expandConfig(template.config, env)
	if err != nil {
		oh.params.Logger.Error("unable to resolve template config", zap.String("exporter", template.id.String()), zap.Error(err))
		return
	}
	// Remove any nil values from resolvedConfig (optional fields that don't exist in the endpoint env)
	// This handles cases where nested fields like spec["tls"]["insecure"] don't exist
	resolvedConfig = removeNilValuesFromMap(resolvedConfig)

	discoveredCfg := userConfigMap{}
	// If user didn't set endpoint, automatically set it from the discovered endpoint target.
	// This allows templates to inherit required fields from the exporter factory.
	// The mergeTemplatedAndDiscoveredConfigs function will validate if the endpoint field
	// is supported by the exporter factory and remove it if not.
	if _, ok := resolvedConfig[endpointConfigKey]; !ok {
		discoveredCfg[endpointConfigKey] = e.Target
		discoveredCfg[tmpSetEndpointConfigKey] = struct{}{}
	}

	// Though not necessary with contrib provided observers, nothing is stopping custom
	// ones from using expr in their Target values.
	discoveredConfig, err := expandConfig(discoveredCfg, env)
	if err != nil {
		oh.params.Logger.Error("unable to resolve discovered config", zap.String("exporter", template.id.String()), zap.Error(err))
		return
	}

	oh.params.Logger.Info("starting exporter",
		zap.String("name", template.id.String()),
		zap.String("endpoint_target", e.Target),
		zap.String("endpoint_id", string(e.ID)),
	)

	oh.params.Logger.Debug("starting exporter with resolved config", zap.Any("config", resolvedConfig))

	var exporterInstance component.Component
	if exporterInstance, err = oh.runner.start(
		exporterConfig{
			id:         template.id,
			config:     resolvedConfig,
			endpointID: e.ID,
		},
		discoveredConfig,
		template.signals,
	); err != nil {
		oh.params.Logger.Error("failed to start exporter", zap.String("exporter", template.id.String()), zap.Error(err))
		return
	}

	// Store the exporter and register it with the router
	oh.exportersByEndpoint[e.ID] = exporterInstance

	// Expand ResourceAttributes from the template and merge them into the env for routing
	routingEnv := make(observer.EndpointEnv)
	// Copy the original env
	for k, v := range env {
		routingEnv[k] = v
	}
	// Expand and add ResourceAttributes from the template
	if len(template.ResourceAttributes) > 0 {
		for attrKey, attrValue := range template.ResourceAttributes {
			// Convert attrValue to string if it's not already
			var attrValueStr string
			if strVal, ok := attrValue.(string); ok {
				attrValueStr = strVal
			} else {
				attrValueStr = fmt.Sprintf("%v", attrValue)
			}
			// Expand the attribute value using the endpoint env
			expandedValue, err := evalBackticksInConfigValue(attrValueStr, env)
			if err != nil {
				oh.params.Logger.Warn("failed to expand resource attribute for routing",
					zap.String("exporter", template.id.String()),
					zap.String("attribute", attrKey),
					zap.Error(err))
				continue
			}
			// Add the expanded attribute to the routing env
			// This allows routing rules to match against these attributes
			routingEnv[attrKey] = expandedValue
		}
	}

	oh.router.AddExporter(e.ID, exporterInstance, routingEnv)

	// Debug log the routing properties for this exporter
	if ce := oh.params.Logger.Check(zap.DebugLevel, "exporter registered with routing properties"); ce != nil {
		// Extract relevant routing info for debugging
		routingInfo := make(map[string]any)
		for k, v := range routingEnv {
			// Only log top-level keys and spec to avoid huge logs
			if k == "spec" || k == "type" || k == "name" || k == "endpoint" || k == "kind" {
				routingInfo[k] = v
			}
		}
		// Also log the spec.resourceAttributes if it exists for routing debugging
		if spec, ok := routingEnv["spec"].(map[string]any); ok {
			if resourceAttrs, ok := spec["resourceAttributes"].(map[string]any); ok {
				routingInfo["spec.resourceAttributes"] = resourceAttrs
				// Specifically log the generator value if it exists
				if generator, ok := resourceAttrs["generator"]; ok {
					routingInfo["spec.resourceAttributes.generator"] = generator
				}
			}
		}
		ce.Write(
			zap.String("endpoint_id", string(e.ID)),
			zap.String("exporter", template.id.String()),
			zap.Any("routing_properties", routingInfo),
		)
	}
}
