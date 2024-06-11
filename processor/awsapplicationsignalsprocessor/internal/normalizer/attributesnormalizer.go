// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package normalizer // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor/internal/normalizer"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	deprecatedsemconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	attr "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor/internal/common"
)

const (
	// Length limits from Application Signals SLOs
	maxEnvironmentLength = 259
	maxServiceNameLength = 255

	// Length limits from CloudWatch Metrics
	defaultMetricAttributeLength = 1024
)

type attributesNormalizer struct {
	logger *zap.Logger
}

var attributesRenamingForMetric = map[string]string{
	attr.AWSLocalService:             common.MetricAttributeLocalService,
	attr.AWSLocalOperation:           common.MetricAttributeLocalOperation,
	attr.AWSLocalEnvironment:         common.MetricAttributeEnvironment,
	attr.AWSRemoteService:            common.MetricAttributeRemoteService,
	attr.AWSRemoteOperation:          common.MetricAttributeRemoteOperation,
	attr.AWSRemoteEnvironment:        common.MetricAttributeRemoteEnvironment,
	attr.AWSRemoteTarget:             common.MetricAttributeRemoteResourceIdentifier,
	attr.AWSRemoteResourceIdentifier: common.MetricAttributeRemoteResourceIdentifier,
	attr.AWSRemoteResourceType:       common.MetricAttributeRemoteResourceType,
}

var resourceAttributesRenamingForTrace = map[string]string{
	// these kubernetes resource attributes are set by the openTelemetry operator
	// see the code references from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L245
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L305C43-L305C43
	semconv.AttributeK8SDeploymentName:  "K8s.Workload",
	semconv.AttributeK8SStatefulSetName: "K8s.Workload",
	semconv.AttributeK8SDaemonSetName:   "K8s.Workload",
	semconv.AttributeK8SJobName:         "K8s.Workload",
	semconv.AttributeK8SCronJobName:     "K8s.Workload",
	semconv.AttributeK8SPodName:         "K8s.Pod",
}

var attributesRenamingForTrace = map[string]string{
	attr.AWSRemoteTarget: attr.AWSRemoteResourceIdentifier,
}

var copyMapForMetric = map[string]string{
	// these kubernetes resource attributes are set by the openTelemtry operator
	// see the code referecnes from upstream:
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L245
	// * https://github.com/open-telemetry/opentelemetry-operator/blob/0e39ee77693146e0924da3ca474a0fe14dc30b3a/pkg/instrumentation/sdk.go#L305C43-L305C43
	semconv.AttributeK8SDeploymentName:  "K8s.Workload",
	semconv.AttributeK8SStatefulSetName: "K8s.Workload",
	semconv.AttributeK8SDaemonSetName:   "K8s.Workload",
	semconv.AttributeK8SJobName:         "K8s.Workload",
	semconv.AttributeK8SCronJobName:     "K8s.Workload",
	semconv.AttributeK8SPodName:         "K8s.Pod",
}

const (
	instrumentationModeAuto   = "Auto"
	instrumentationModeManual = "Manual"
)

func NewAttributesNormalizer(logger *zap.Logger) *attributesNormalizer {
	return &attributesNormalizer{
		logger: logger,
	}
}

func (n *attributesNormalizer) Process(attributes, resourceAttributes pcommon.Map, isTrace bool) error {
	n.copyResourceAttributesToAttributes(attributes, resourceAttributes, isTrace)
	truncateAttributesByLength(attributes)
	n.renameAttributes(attributes, resourceAttributes, isTrace)
	n.normalizeTelemetryAttributes(attributes, resourceAttributes, isTrace)
	return nil
}

func (n *attributesNormalizer) renameAttributes(attributes, resourceAttributes pcommon.Map, isTrace bool) {
	if isTrace {
		rename(resourceAttributes, resourceAttributesRenamingForTrace)
		rename(attributes, attributesRenamingForTrace)
	} else {
		rename(attributes, attributesRenamingForMetric)
	}
}

func (n *attributesNormalizer) copyResourceAttributesToAttributes(attributes, resourceAttributes pcommon.Map, isTrace bool) {
	if isTrace {
		return
	}
	for k, v := range copyMapForMetric {
		if resourceAttrValue, ok := resourceAttributes.Get(k); ok {
			// print some debug info when an attribute value is overwritten
			if originalAttrValue, ok := attributes.Get(k); ok {
				n.logger.Debug("attribute value is overwritten", zap.String("attribute", k), zap.String("original", originalAttrValue.AsString()), zap.String("new", resourceAttrValue.AsString()))
			}
			attributes.PutStr(v, resourceAttrValue.AsString())
			if k == semconv.AttributeK8SPodName {
				// only copy "host.id" from resource attributes to "K8s.Node" in attributesif the pod name is set
				if host, ok := resourceAttributes.Get("host.id"); ok {
					attributes.PutStr("K8s.Node", host.AsString())
				}
			}
		}
	}
}

func (n *attributesNormalizer) normalizeTelemetryAttributes(attributes, resourceAttributes pcommon.Map, isTrace bool) {
	if isTrace {
		return
	}

	var (
		sdkName    string
		sdkVersion string
		sdkLang    string
	)
	var (
		sdkAutoName    string
		sdkAutoVersion string
	)
	sdkName, sdkVersion, sdkLang = "-", "-", "-"
	mode := instrumentationModeManual

	resourceAttributes.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case semconv.AttributeTelemetrySDKName:
			sdkName = removeWhitespaces(v.Str())
		case semconv.AttributeTelemetrySDKLanguage:
			sdkLang = removeWhitespaces(v.Str())
		case semconv.AttributeTelemetrySDKVersion:
			sdkVersion = removeWhitespaces(v.Str())
		}
		switch k {
		case semconv.AttributeTelemetryDistroName:
			sdkAutoName = removeWhitespaces(v.Str())
		case deprecatedsemconv.AttributeTelemetryAutoVersion, semconv.AttributeTelemetryDistroVersion:
			sdkAutoVersion = removeWhitespaces(v.Str())
		}
		return true
	})
	if sdkAutoName != "" {
		sdkName = sdkAutoName
		mode = instrumentationModeAuto
	}
	if sdkAutoVersion != "" {
		sdkVersion = sdkAutoVersion
		mode = instrumentationModeAuto
	}
	attributes.PutStr(common.AttributeTelemetrySDK, fmt.Sprintf("%s,%s,%s,%s", sdkName, sdkVersion, sdkLang, mode))
	// TODO: append version to OTelCollector (e.g. OTelCollector/v0.100.0)
	attributes.PutStr(common.AttributeTelemetryAgent, fmt.Sprintf("OTelCollector"))

	var telemetrySource string
	if val, ok := attributes.Get(attr.AWSSpanKind); ok {
		switch val.Str() {
		case "CLIENT":
			telemetrySource = "ClientSpan"
		case "SERVER":
			telemetrySource = "ServerSpan"
		case "PRODUCER":
			telemetrySource = "ProducerSpan"
		case "CONSUMER":
			telemetrySource = "ConsumerSpan"
		case "LOCAL_ROOT":
			telemetrySource = "LocalRootSpan"
		}
		attributes.PutStr(common.AttributeTelemetrySource, telemetrySource)
		attributes.Remove(attr.AWSSpanKind)
	}
}

func rename(attrs pcommon.Map, renameMap map[string]string) {
	for original, replacement := range renameMap {
		if value, ok := attrs.Get(original); ok {
			attrs.PutStr(replacement, value.AsString())
			attrs.Remove(original)
			if original == semconv.AttributeK8SPodName {
				// only rename host.id if the pod name is set
				if host, ok := attrs.Get("host.id"); ok {
					attrs.PutStr("K8s.Node", host.AsString())
				}
			}
		}
	}
}

func truncateAttributesByLength(attributes pcommon.Map) {
	// It's assumed that all attributes are initially inserted as trace attribute, and attributesRenamingForMetric
	// contains all attributes that will be used for CloudWatch metric dimension. Therefore, we iterate the keys
	// for enforcing the limits on length.
	for attrKey := range attributesRenamingForMetric {
		switch attrKey {
		case attr.AWSLocalEnvironment, attr.AWSRemoteEnvironment:
			if val, ok := attributes.Get(attrKey); ok {
				attributes.PutStr(attrKey, truncateStringByLength(val.Str(), maxEnvironmentLength))
			}
		case attr.AWSLocalService, attr.AWSRemoteService:
			if val, ok := attributes.Get(attrKey); ok {
				attributes.PutStr(attrKey, truncateStringByLength(val.Str(), maxServiceNameLength))
			}
		default:
			if val, ok := attributes.Get(attrKey); ok {
				attributes.PutStr(attrKey, truncateStringByLength(val.Str(), defaultMetricAttributeLength))
			}
		}
	}
}

func truncateStringByLength(val string, length int) string {
	if len(val) > length {
		return val[:length]
	}
	return val
}

func removeWhitespaces(val string) string {
	return strings.ReplaceAll(val, " ", "")
}
