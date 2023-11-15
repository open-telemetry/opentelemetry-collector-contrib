// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// cloudNamespaceProcessor adds the `cloud.namespace` resource attribute to logs, metrics and traces.
type cloudNamespaceProcessor struct {
	addCloudNamespace bool
}

const (
	cloudNamespaceAttributeName = "cloud.namespace"
	cloudNamespaceAwsEc2        = "aws/ec2"
	cloudNamespaceAwsEcs        = "ecs"
	cloudNamespaceAwsBeanstalk  = "ElasticBeanstalk"
)

func newCloudNamespaceProcessor(addCloudNamespace bool) *cloudNamespaceProcessor {
	return &cloudNamespaceProcessor{
		addCloudNamespace: addCloudNamespace,
	}
}

func (*cloudNamespaceProcessor) processLogs(logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		addCloudNamespaceAttribute(logs.ResourceLogs().At(i).Resource().Attributes())
	}
	return nil
}

func (*cloudNamespaceProcessor) processMetrics(metrics pmetric.Metrics) error {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		addCloudNamespaceAttribute(metrics.ResourceMetrics().At(i).Resource().Attributes())
	}
	return nil
}

func (*cloudNamespaceProcessor) processTraces(traces ptrace.Traces) error {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		addCloudNamespaceAttribute(traces.ResourceSpans().At(i).Resource().Attributes())
	}
	return nil
}

func (proc *cloudNamespaceProcessor) isEnabled() bool {
	return proc.addCloudNamespace
}

func (*cloudNamespaceProcessor) ConfigPropertyName() string {
	return "add_cloud_namespace"
}

// addCloudNamespaceAttribute adds the `cloud.namespace` attribute
// to a collection of attributes that already contains a `cloud.platform` attribute.
// It does not add the `cloud.namespace` attribute for all `cloud.platform` values,
// but only for a few specific ones - namely AWS EC2, AWS ECS, and AWS Elastic Beanstalk.
func addCloudNamespaceAttribute(attributes pcommon.Map) {
	cloudPlatformAttributeValue, found := attributes.Get(conventions.AttributeCloudPlatform)
	if !found {
		return
	}

	switch cloudPlatformAttributeValue.Str() {
	case conventions.AttributeCloudPlatformAWSEC2:
		attributes.PutStr(cloudNamespaceAttributeName, cloudNamespaceAwsEc2)
	case conventions.AttributeCloudPlatformAWSECS:
		attributes.PutStr(cloudNamespaceAttributeName, cloudNamespaceAwsEcs)
	case conventions.AttributeCloudPlatformAWSElasticBeanstalk:
		attributes.PutStr(cloudNamespaceAttributeName, cloudNamespaceAwsBeanstalk)
	}
}
