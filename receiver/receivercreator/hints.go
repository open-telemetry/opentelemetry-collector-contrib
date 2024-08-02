// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	otelHints                      = "io.opentelemetry.collector.receiver-creator"
	metricsHint                    = "metrics"
	hintsMetricsReceiver           = "receiver"
	hintsMetricsEndpoint           = "endpoint"
	hintsMetricsCollectionInterval = "collection_interval"
	hintsMetricsTimeout            = "timeout"
	hintsMetricsUsername           = "username"
	hintsMetricsPassword           = "password"
)

// HintsTemplatesBuilder creates configuration templates from provided hints.
type HintsTemplatesBuilder interface {
	createReceiverTemplatesFromHints() ([]receiverTemplate, error)
}

// K8sHintsBuilder creates configurations from hints provided as Pod's annotations.
type K8sHintsBuilder struct {
	logger *zap.Logger
	config K8sHintsConfig
}

// createReceiverTemplateFromHints creates a receiver configuration based on the provided hints.
// Hints are extracted from Pod's annotations.
// Metrics configurations are only created for Port Endpoints.
// TODO: Logs configurations are only created for Pod Container Endpoints.
func (builder *K8sHintsBuilder) createReceiverTemplateFromHints(env observer.EndpointEnv) (*receiverTemplate, error) {
	var endpointType string
	var podUID string
	var annotations map[string]string

	builder.logger.Debug("handling hints for added endpoint", zap.Any("env", env))

	if pod, ok := env["pod"]; ok {
		endpointPod, ok := pod.(observer.EndpointEnv)
		if !ok {
			return nil, fmt.Errorf("could not extract endpoint's pod: %v", zap.Any("endpointPod", pod))
		}
		ann := endpointPod["annotations"]
		if ann != nil {
			annotations, ok = ann.(map[string]string)
			if !ok {
				return nil, fmt.Errorf("could not extract annotations: %v", zap.Any("annotations", ann))
			}
		}
		podUID = endpointPod["uid"].(string)
	} else {
		return nil, nil
	}

	if valType, ok := env["type"]; ok {
		endpointType, ok = valType.(string)
		if !ok {
			return nil, fmt.Errorf("could not extract endpointType: %v", zap.Any("endpointType", valType))
		}
	} else {
		return nil, fmt.Errorf("could not get endpoint type: %v", zap.Any("env", env))
	}

	if len(annotations) > 0 {
		if endpointType == string(observer.PortType) && builder.config.Metrics.Enabled {
			// Only handle Endpoints of type port for metrics
			return builder.createMetricsReceiver(annotations, env, podUID)
		}
	}
	return nil, nil
}

func (builder *K8sHintsBuilder) createMetricsReceiver(
	annotations map[string]string,
	env observer.EndpointEnv,
	podUID string) (*receiverTemplate, error) {

	var port uint16

	portName := env["name"].(string)
	subreceiverKey := getHintAnnotation(annotations, metricsHint, hintsMetricsReceiver, portName)

	if subreceiverKey == "" {
		// no metrics hints detected
		return nil, nil
	}
	builder.logger.Debug("handling added hinted receiver", zap.Any("subreceiverKey", subreceiverKey))

	userConfMap := createMetricsConfig(annotations, env, portName)

	if p, ok := env["port"]; ok {
		port = p.(uint16)
		if port == 0 {
			return nil, fmt.Errorf("could not extract port: %v", zap.Any("env", env))
		}
	} else {
		return nil, fmt.Errorf("could not extract port: %v", zap.Any("env", env))
	}
	subreceiver, err := newReceiverTemplate(fmt.Sprintf("%v/%v_%v", subreceiverKey, podUID, port), userConfMap)
	if err != nil {
		builder.logger.Error("error adding subreceiver", zap.Any("err", err))
		return nil, err
	}

	builder.logger.Debug("adding hinted receiver", zap.Any("subreceiver", subreceiver))
	return &subreceiver, nil

}

func createMetricsConfig(annotations map[string]string, env observer.EndpointEnv, portName string) userConfigMap {
	confMap := map[string]any{}

	defaultEndpoint := env["endpoint"]
	// get endpoint directly from the Port endpoint
	if defaultEndpoint != "" {
		confMap["endpoint"] = defaultEndpoint
	}

	subreceiverEndpoint := getHintAnnotation(annotations, metricsHint, hintsMetricsEndpoint, portName)
	if subreceiverEndpoint != "" {
		confMap["endpoint"] = subreceiverEndpoint
	}
	subreceiverColInterval := getHintAnnotation(annotations, metricsHint, hintsMetricsCollectionInterval, portName)
	if subreceiverColInterval != "" {
		confMap["collection_interval"] = subreceiverColInterval
	}
	subreceiverTimeout := getHintAnnotation(annotations, metricsHint, hintsMetricsTimeout, portName)
	if subreceiverTimeout != "" {
		confMap["timeout"] = subreceiverTimeout
	}
	subreceiverUsername := getHintAnnotation(annotations, metricsHint, hintsMetricsUsername, portName)
	if subreceiverUsername != "" {
		confMap["username"] = subreceiverUsername
	}
	subreceiverPassword := getHintAnnotation(annotations, metricsHint, hintsMetricsPassword, portName)
	if subreceiverPassword != "" {
		confMap["password"] = subreceiverPassword
	}
	return confMap
}

func getHintAnnotation(annotations map[string]string, hintType string, hintKey string, suffix string) string {
	// try to scope the hint more on container level by suffixing with .<port_name>
	containerLevelHint := annotations[fmt.Sprintf("%s.%s.%s/%s", otelHints, hintType, suffix, hintKey)]
	if containerLevelHint != "" {
		return containerLevelHint
	}

	// if there is no container level hint defined try to use the Pod level hint
	podHintKey := fmt.Sprintf("%s.%s/%s", otelHints, hintType, hintKey)
	podLevelHint := annotations[podHintKey]
	if podLevelHint != "" {
		return podLevelHint
	}
	return ""
}
