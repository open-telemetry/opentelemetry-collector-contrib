// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	otelHints            = "io.opentelemetry.discovery"
	discoveryEnabledHint = "enabled"
	scraperHint          = "scraper"
	signalsHint          = "signals"
	configHint           = "config."
)

// HintsTemplatesBuilder creates configuration templates from provided hints.
type HintsTemplatesBuilder interface {
	createScraperTemplateFromHints() ([]receiverTemplate, error)
}

// K8sHintsBuilder creates configurations from hints provided as Pod's annotations.
type K8sHintsBuilder struct {
	logger *zap.Logger
	config K8sHintsConfig
}

// createScraperTemplateFromHints creates a receiver configuration based on the provided hints.
// Hints are extracted from Pod's annotations.
// Scraper configurations are only created for Port Endpoints.
// TODO: container_logs configurations are only created for Pod Container Endpoints.
func (builder *K8sHintsBuilder) createScraperTemplateFromHints(env observer.EndpointEnv) (*receiverTemplate, error) {
	var endpointType string
	var podUID string
	var annotations map[string]string

	endpointType = getStringEnv(env, "type")
	if endpointType == "" {
		return nil, fmt.Errorf("could not get endpoint type: %v", zap.Any("env", env))
	}

	if endpointType != string(observer.PortType) {
		return nil, nil
	}

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
			if len(annotations) == 0 {
				return nil, nil
			}
		}
		podUID = endpointPod["uid"].(string)
	} else {
		return nil, nil
	}

	return builder.createScraper(annotations, env, podUID)
}

func (builder *K8sHintsBuilder) createScraper(
	annotations map[string]string,
	env observer.EndpointEnv,
	podUID string) (*receiverTemplate, error) {

	var port uint16
	portName := getStringEnv(env, "name")

	if !discoveryEnabled(annotations, portName) {
		return nil, nil
	}

	subreceiverKey := getHintAnnotation(annotations, scraperHint, portName)
	if subreceiverKey == "" {
		// no scraper hint detected
		return nil, nil
	}
	builder.logger.Debug("handling added hinted receiver", zap.Any("subreceiverKey", subreceiverKey))

	defaultEndpoint := getStringEnv(env, "endpoint")
	userConfMap := getConfFromAnnotations(annotations, defaultEndpoint, portName)

	signalsAnn := getHintAnnotation(annotations, signalsHint, portName)
	signals := getSignalsConf(signalsAnn)

	if p, ok := env["port"]; ok {
		port, ok = p.(uint16)
		if !ok || port == 0 {
			return nil, fmt.Errorf("could not extract port: %v", zap.Any("env", env))
		}
	} else {
		return nil, fmt.Errorf("could not extract port: %v", zap.Any("env", env))
	}

	recTemplate, err := newReceiverTemplate(fmt.Sprintf("%v/%v_%v", subreceiverKey, podUID, port), userConfMap)
	recTemplate.signals = signals

	return &recTemplate, err

}

func getConfFromAnnotations(annotations map[string]string, defaultEndpoint string, scopeSuffix string) userConfigMap {
	var annotationPrefixScoped string
	annotationPrefix := fmt.Sprintf("%s/%s", otelHints, configHint)

	if scopeSuffix != "" {
		annotationPrefixScoped = fmt.Sprintf("%s.%s/%s", otelHints, scopeSuffix, configHint)
	}
	conf := userConfigMap{}
	if defaultEndpoint != "" {
		conf["endpoint"] = defaultEndpoint
	}

	for key, val := range annotations {
		var dst map[string]any
		var dstList []any
		if strings.HasPrefix(key, annotationPrefixScoped) && annotationPrefixScoped != "" {
			res, _ := strings.CutPrefix(key, annotationPrefixScoped)

			if err := yaml.Unmarshal([]byte(val), &dst); err == nil {
				conf[res] = dst
			} else if err = yaml.Unmarshal([]byte(val), &dstList); err == nil {
				conf[res] = dstList
			} else {
				conf[res] = val
			}
		} else if strings.HasPrefix(key, annotationPrefix) {
			res, _ := strings.CutPrefix(key, annotationPrefix)

			if _, ok := conf[res]; !ok {
				// only use top level annotation in case there is no scope level annotation already set
				if err := yaml.Unmarshal([]byte(val), &dst); err == nil {
					conf[res] = dst
				} else if err = yaml.Unmarshal([]byte(val), &dstList); err == nil {
					conf[res] = dstList
				} else {
					conf[res] = val
				}
			}
		}
	}
	return conf
}

func getHintAnnotation(annotations map[string]string, hintKey string, suffix string) string {
	// try to scope the hint more on container level by suffixing with .<port_name>
	containerLevelHint := annotations[fmt.Sprintf("%s.%s/%s", otelHints, suffix, hintKey)]
	if containerLevelHint != "" {
		return containerLevelHint
	}

	// if there is no container level hint defined try to use the Pod level hint
	podHintKey := fmt.Sprintf("%s/%s", otelHints, hintKey)
	podLevelHint := annotations[podHintKey]
	if podLevelHint != "" {
		return podLevelHint
	}
	return ""
}

func getSignalsConf(signalsStr string) receiverSignals {
	s := strings.Split(signalsStr, ",")
	if len(s) == 0 || signalsStr == "" {
		return receiverSignals{true, true, true}
	}
	signals := receiverSignals{}
	for _, signal := range s {
		if signal == "metrics" {
			signals.metrics = true
		}
		if signal == "logs" {
			signals.logs = true
		}
		if signal == "traces" {
			signals.traces = true
		}
	}
	return signals
}

func discoveryEnabled(annotations map[string]string, scopeSuffix string) bool {
	hintVal := getHintAnnotation(annotations, discoveryEnabledHint, scopeSuffix)
	if hintVal == "true" || hintVal == "" {
		return true
	}
	return false
}

func getStringEnv(env observer.EndpointEnv, key string) string {
	var valString string
	if val, ok := env[key]; ok {
		valString, ok = val.(string)
		if !ok {
			return ""
		}
	}
	return valString
}
