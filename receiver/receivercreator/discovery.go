// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"errors"
	"fmt"
	"net/url"
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
	otelLogsHints    = otelHints + ".logs"

	// hints definitions
	discoveryEnabledHint = "enabled"
	scraperHint          = "scraper"
	configHint           = "config"

	logsReceiver          = "filelog"
	defaultLogPathPattern = "/var/log/pods/%s_%s_%s/%s/*.log"
)

// k8sHintsBuilder creates configurations from hints provided as Pod's annotations.
type k8sHintsBuilder struct {
	logger          *zap.Logger
	ignoreReceivers map[string]bool
}

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
// Log receiver configurations are only created for Pod Container Endpoints.
func (builder *k8sHintsBuilder) createReceiverTemplateFromHints(env observer.EndpointEnv) (*receiverTemplate, error) {
	var pod observer.Pod

	endpointType := getStringEnv(env, "type")
	if endpointType == "" {
		return nil, fmt.Errorf("could not get endpoint type: %v", zap.Any("env", env))
	}

	if endpointType != string(observer.PortType) && endpointType != string(observer.PodContainerType) {
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

	switch endpointType {
	case string(observer.PortType):
		return builder.createScraper(pod.Annotations, env)
	case string(observer.PodContainerType):
		return builder.createLogsReceiver(pod.Annotations, env)
	default:
		return nil, nil
	}
}

func (builder *k8sHintsBuilder) createScraper(
	annotations map[string]string,
	env observer.EndpointEnv,
) (*receiverTemplate, error) {
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

	if !discoveryEnabled(annotations, otelMetricsHints, fmt.Sprint(port)) {
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
	userConfMap, err := getScraperConfFromAnnotations(annotations, defaultEndpoint, fmt.Sprint(port), builder.logger)
	if err != nil {
		return nil, fmt.Errorf("could not create receiver configuration: %v", zap.Error(err))
	}

	recTemplate, err := newReceiverTemplate(fmt.Sprintf("%v/%v_%v", subreceiverKey, pod.UID, port), userConfMap)
	recTemplate.signals = receiverSignals{metrics: true, logs: false, traces: false}

	return &recTemplate, err
}

func (builder *k8sHintsBuilder) createLogsReceiver(
	annotations map[string]string,
	env observer.EndpointEnv,
) (*receiverTemplate, error) {
	if _, ok := builder.ignoreReceivers[logsReceiver]; ok {
		// receiver is ignored
		return nil, nil
	}

	var containerName string
	var c observer.PodContainer
	err := mapstructure.Decode(env, &c)
	if err != nil {
		return nil, fmt.Errorf("could not extract pod's container: %v", zap.Any("env", env))
	}
	if c.Name == "" {
		return nil, fmt.Errorf("could not extract container name: %v", zap.Any("container", c))
	}
	containerName = c.Name
	pod := c.Pod

	if !discoveryEnabled(annotations, otelLogsHints, containerName) {
		return nil, nil
	}

	subreceiverKey := logsReceiver
	builder.logger.Debug("handling added hinted receiver", zap.Any("subreceiverKey", subreceiverKey))

	userConfMap := createLogsConfig(
		annotations,
		containerName,
		pod.UID,
		pod.Name,
		pod.Namespace,
		builder.logger)

	recTemplate, err := newReceiverTemplate(fmt.Sprintf("%v/%v_%v", subreceiverKey, pod.UID, containerName), userConfMap)
	recTemplate.signals = receiverSignals{metrics: false, logs: true, traces: false}

	return &recTemplate, err
}

func getScraperConfFromAnnotations(
	annotations map[string]string,
	defaultEndpoint, scopeSuffix string,
	logger *zap.Logger,
) (userConfigMap, error) {
	conf := userConfigMap{}
	conf[endpointConfigKey] = defaultEndpoint

	configStr, found := getHintAnnotation(annotations, otelMetricsHints, configHint, scopeSuffix)
	if !found || configStr == "" {
		return conf, nil
	}
	if err := yaml.Unmarshal([]byte(configStr), &conf); err != nil {
		return userConfigMap{}, fmt.Errorf("could not unmarshal configuration from hint: %v", zap.Error(err))
	}

	val := conf[endpointConfigKey]
	confEndpoint, ok := val.(string)
	if !ok {
		logger.Debug("could not extract configured endpoint")
		return userConfigMap{}, errors.New("could not extract configured endpoint")
	}

	err := validateEndpoint(confEndpoint, defaultEndpoint)
	if err != nil {
		logger.Debug("configured endpoint is not valid", zap.Error(err))
		return userConfigMap{}, fmt.Errorf("configured endpoint is not valid: %v", zap.Error(err))
	}
	return conf, nil
}

func createLogsConfig(
	annotations map[string]string,
	containerName, podUID, podName, namespace string,
	logger *zap.Logger,
) userConfigMap {
	scopeSuffix := containerName
	logPath := fmt.Sprintf(defaultLogPathPattern, namespace, podName, podUID, containerName)
	cont := []any{map[string]any{"id": "container-parser", "type": "container"}}
	defaultConfMap := userConfigMap{
		"include":           []string{logPath},
		"include_file_path": true,
		"include_file_name": false,
		"operators":         cont,
	}

	configStr, found := getHintAnnotation(annotations, otelLogsHints, configHint, scopeSuffix)
	if !found || configStr == "" {
		return defaultConfMap
	}

	userConf := make(map[string]any)
	if err := yaml.Unmarshal([]byte(configStr), &userConf); err != nil {
		logger.Debug("could not unmarshal configuration from hint", zap.Error(err))
	}

	for k, v := range userConf {
		if k == "include" {
			// path cannot be other than the one of the target container
			logger.Warn("include setting cannot be set through annotation's hints")
			continue
		}
		defaultConfMap[k] = v
	}

	return defaultConfMap
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

func discoveryEnabled(annotations map[string]string, hintBase string, scopeSuffix string) bool {
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

func validateEndpoint(endpoint, defaultEndpoint string) error {
	// replace temporarily the dynamic reference to ease the url parsing
	endpoint = strings.ReplaceAll(endpoint, "`endpoint`", defaultEndpoint)

	uri, _ := url.Parse(endpoint)
	// target endpoint can come in form ip:port. In that case we fix the uri
	// temporarily with adding http scheme
	if uri == nil {
		u, err := url.Parse("http://" + endpoint)
		if err != nil {
			return errors.New("could not parse endpoint")
		}
		uri = u
	}

	// configured endpoint should include the target Pod's endpoint
	if uri.Host != defaultEndpoint {
		return errors.New("configured endpoint should include target Pod's endpoint")
	}
	return nil
}
