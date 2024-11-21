// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

const (
	// hints prefix
	otelHints = "io.opentelemetry.discovery"

	// hint suffix for metrics
	otelMetricsHints = otelHints + ".metrics"

	// hints definitions
	discoveryEnabledHint = "enabled"
	scraperHint          = "scraper"
	configHint           = "config"
)

// hintsTemplatesBuilder creates configuration templates from provided hints.
type hintsTemplatesBuilder interface {
	createReceiverTemplateFromHints(env observer.EndpointEnv) (*receiverTemplate, error)
}

// k8sHintsBuilder creates configurations from hints provided as Pod's annotations.
type k8sHintsBuilder struct {
	logger          *zap.Logger
	ignoreReceivers map[string]bool
}

var _ hintsTemplatesBuilder = (*k8sHintsBuilder)(nil)

func createK8sHintsBuilder(config DiscoveryConfig, logger *zap.Logger) k8sHintsBuilder {
	ignoreReceivers := make(map[string]bool, len(config.IgnoreReceivers))
	for _, r := range config.IgnoreReceivers {
		ignoreReceivers[r] = true
	}
	return k8sHintsBuilder{
		logger:          logger,
		ignoreReceivers: ignoreReceivers,
	}
}

// createReceiverTemplateFromHints creates a receiver configuration based on the provided hints.
// Hints are extracted from Pod's annotations.
// Scraper configurations are only created for Port Endpoints.
// TODO: Log receiver configurations are only created for Pod Container Endpoints.
func (builder *k8sHintsBuilder) createReceiverTemplateFromHints(env observer.EndpointEnv) (*receiverTemplate, error) {
	var pod observer.Pod

	endpointType := getStringEnv(env, "type")
	if endpointType == "" {
		return nil, fmt.Errorf("could not get endpoint type: %v", zap.Any("env", env))
	}

	if endpointType != string(observer.PortType) {
		return nil, nil
	}

	builder.logger.Debug("handling hints for added endpoint", zap.Any("env", env))

	if endpointPod, ok := env["pod"]; ok {
		err := mapstructure.Decode(endpointPod, &pod)
		if err != nil {
			return nil, fmt.Errorf("could not extract endpoint's pod: %v", zap.Any("endpointPod", pod))
		}
	} else {
		return nil, nil
	}

	return builder.createScraper(pod.Annotations, env)
}

func (builder *k8sHintsBuilder) createScraper(
	annotations map[string]string,
	env observer.EndpointEnv) (*receiverTemplate, error) {
	var port uint16
	var p observer.Port
	err := mapstructure.Decode(env, &p)
	if err != nil {
		return nil, fmt.Errorf("could not extract port event: %v", zap.Any("env", env))
	}
	if p.Port == 0 {
		return nil, fmt.Errorf("could not extract port: %v", zap.Any("env", env))
	}
	port = p.Port
	pod := p.Pod

	if !discoveryMetricsEnabled(annotations, otelMetricsHints, fmt.Sprint(port)) {
		return nil, nil
	}

	subreceiverKey, found := getHintAnnotation(annotations, otelMetricsHints, scraperHint, fmt.Sprint(port))
	if !found || subreceiverKey == "" {
		// no scraper hint detected
		return nil, nil
	}
	if _, ok := builder.ignoreReceivers[subreceiverKey]; ok {
		// scraper is ignored
		return nil, nil
	}
	builder.logger.Debug("handling added hinted receiver", zap.Any("subreceiverKey", subreceiverKey))

	defaultEndpoint := getStringEnv(env, endpointConfigKey)
	userConfMap := getScraperConfFromAnnotations(annotations, defaultEndpoint, fmt.Sprint(port), builder.logger)

	recTemplate, err := newReceiverTemplate(fmt.Sprintf("%v/%v_%v", subreceiverKey, pod.UID, port), userConfMap)
	recTemplate.signals = receiverSignals{true, false, false}

	return &recTemplate, err
}

func getScraperConfFromAnnotations(
	annotations map[string]string,
	defaultEndpoint, scopeSuffix string,
	logger *zap.Logger) userConfigMap {
	conf := userConfigMap{}
	conf[endpointConfigKey] = defaultEndpoint

	configStr, found := getHintAnnotation(annotations, otelMetricsHints, configHint, scopeSuffix)
	if !found || configStr == "" {
		return conf
	}
	if err := yaml.Unmarshal([]byte(configStr), &conf); err != nil {
		logger.Debug("could not unmarshal configuration from hint", zap.Error(err))
	}

	val := conf[endpointConfigKey]
	confEndpoint, ok := val.(string)
	if !ok {
		logger.Debug("could not extract configured enpoint")
		return userConfigMap{}
	}

	if !strings.Contains(confEndpoint, defaultEndpoint) {
		logger.Debug("configured enpoint should include Pod's endpoint")
		return userConfigMap{}
	}

	return conf
}

func getHintAnnotation(annotations map[string]string, hintBase string, hintKey string, suffix string) (string, bool) {
	// try to scope the hint more on container level by suffixing
	// with .<port> in case of Port event or # TODO: .<container_name> in case of Pod Container event
	containerLevelHint, ok := annotations[fmt.Sprintf("%s.%s/%s", hintBase, suffix, hintKey)]
	if ok {
		return containerLevelHint, ok
	}

	// if there is no container level hint defined try to use the Pod level hint
	podLevelHint, ok := annotations[fmt.Sprintf("%s/%s", hintBase, hintKey)]
	return podLevelHint, ok
}

func discoveryMetricsEnabled(annotations map[string]string, hintBase string, scopeSuffix string) bool {
	enabledHint, found := getHintAnnotation(annotations, hintBase, discoveryEnabledHint, scopeSuffix)
	if !found {
		return false
	}
	return enabledHint == "true"
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
