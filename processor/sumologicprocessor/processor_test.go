// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

func TestAddCloudNamespaceForLogs(t *testing.T) {
	testCases := []struct {
		name              string
		addCloudNamespace bool
		createLogs        func() plog.Logs
		test              func(plog.Logs)
	}{
		{
			name:              "adds cloud.namespace attribute for EC2",
			addCloudNamespace: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				cloudNamespaceAttribute, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds cloud.namespace attribute for ECS",
			addCloudNamespace: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ecs")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				cloudNamespaceAttribute, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ecs", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds cloud.namespace attribute for Beanstalk",
			addCloudNamespace: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_elastic_beanstalk")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				cloudNamespaceAttribute, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ElasticBeanstalk", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "does not add cloud.namespace attribute for unknown cloud.platform attribute values",
			addCloudNamespace: false,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_eks")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_lambda")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "azure_vm")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "gcp_app_engine")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				for i := 0; i < outputLogs.ResourceLogs().Len(); i++ {
					_, found := outputLogs.ResourceLogs().At(i).Resource().Attributes().Get("cloud.namespace")
					assert.False(t, found)
				}
			},
		},
		{
			name:              "does not add cloud.namespce attribute when disabled",
			addCloudNamespace: false,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				cloudNamespaceAttribute, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds different cloud.namespace attributes to different resources",
			addCloudNamespace: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_lambda")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ecs")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_elastic_beanstalk")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				ec2ResourceAttribute, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", ec2ResourceAttribute.Str())

				_, found = outputLogs.ResourceLogs().At(1).Resource().Attributes().Get("cloud.namespace")
				assert.False(t, found)

				ecsResourceAttribute, found := outputLogs.ResourceLogs().At(2).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ecs", ecsResourceAttribute.Str())

				beanstalkResourceAttribute, found := outputLogs.ResourceLogs().At(3).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ElasticBeanstalk", beanstalkResourceAttribute.Str())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newCloudNamespaceConfig(testCase.addCloudNamespace))

			// Act
			outputLogs, err := processor.processLogs(context.Background(), testCase.createLogs())
			require.NoError(t, err)

			// Assert
			testCase.test(outputLogs)
		})
	}
}

func TestAddCloudNamespaceForMetrics(t *testing.T) {
	testCases := []struct {
		name              string
		addCloudNamespace bool
		createMetrics     func() pmetric.Metrics
		test              func(pmetric.Metrics)
	}{
		{
			name:              "adds cloud.namespace attribute for EC2",
			addCloudNamespace: true,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				cloudNamespaceAttribute, found := outputMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds cloud.namespace attribute for ECS",
			addCloudNamespace: true,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ecs")
				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				cloudNamespaceAttribute, found := outputMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ecs", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds cloud.namespace attribute for Beanstalk",
			addCloudNamespace: true,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_elastic_beanstalk")
				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				cloudNamespaceAttribute, found := outputMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ElasticBeanstalk", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "does not add cloud.namespace attribute for unknown cloud.platform attribute values",
			addCloudNamespace: false,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_eks")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_lambda")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "azure_vm")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "gcp_app_engine")
				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				for i := 0; i < outputMetrics.ResourceMetrics().Len(); i++ {
					_, found := outputMetrics.ResourceMetrics().At(i).Resource().Attributes().Get("cloud.namespace")
					assert.False(t, found)
				}
			},
		},
		{
			name:              "does not add cloud.namespce attribute when disabled",
			addCloudNamespace: false,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				cloudNamespaceAttribute, found := outputMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds different cloud.namespace attributes to different resources",
			addCloudNamespace: true,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_lambda")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ecs")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_elastic_beanstalk")
				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				ec2ResourceAttribute, found := outputMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", ec2ResourceAttribute.Str())

				_, found = outputMetrics.ResourceMetrics().At(1).Resource().Attributes().Get("cloud.namespace")
				assert.False(t, found)

				ecsResourceAttribute, found := outputMetrics.ResourceMetrics().At(2).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ecs", ecsResourceAttribute.Str())

				beanstalkResourceAttribute, found := outputMetrics.ResourceMetrics().At(3).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ElasticBeanstalk", beanstalkResourceAttribute.Str())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newCloudNamespaceConfig(testCase.addCloudNamespace))

			// Act
			outputMetrics, err := processor.processMetrics(context.Background(), testCase.createMetrics())
			require.NoError(t, err)

			// Assert
			testCase.test(outputMetrics)
		})
	}
}

func TestAddCloudNamespaceForTraces(t *testing.T) {
	testCases := []struct {
		name              string
		addCloudNamespace bool
		createTraces      func() ptrace.Traces
		test              func(ptrace.Traces)
	}{
		{
			name:              "adds cloud.namespace attribute for EC2",
			addCloudNamespace: true,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				cloudNamespaceAttribute, found := outputTraces.ResourceSpans().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds cloud.namespace attribute for ECS",
			addCloudNamespace: true,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ecs")
				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				cloudNamespaceAttribute, found := outputTraces.ResourceSpans().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ecs", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds cloud.namespace attribute for Beanstalk",
			addCloudNamespace: true,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_elastic_beanstalk")
				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				cloudNamespaceAttribute, found := outputTraces.ResourceSpans().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ElasticBeanstalk", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "does not add cloud.namespace attribute for unknown cloud.platform attribute values",
			addCloudNamespace: false,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_eks")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_lambda")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "azure_vm")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "gcp_app_engine")
				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				for i := 0; i < outputTraces.ResourceSpans().Len(); i++ {
					_, found := outputTraces.ResourceSpans().At(i).Resource().Attributes().Get("cloud.namespace")
					assert.False(t, found)
				}
			},
		},
		{
			name:              "does not add cloud.namespce attribute when disabled",
			addCloudNamespace: false,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				cloudNamespaceAttribute, found := outputTraces.ResourceSpans().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", cloudNamespaceAttribute.Str())
			},
		},
		{
			name:              "adds different cloud.namespace attributes to different resources",
			addCloudNamespace: true,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ec2")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_lambda")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_ecs")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.platform", "aws_elastic_beanstalk")
				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				ec2ResourceAttribute, found := outputTraces.ResourceSpans().At(0).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "aws/ec2", ec2ResourceAttribute.Str())

				_, found = outputTraces.ResourceSpans().At(1).Resource().Attributes().Get("cloud.namespace")
				assert.False(t, found)

				ecsResourceAttribute, found := outputTraces.ResourceSpans().At(2).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ecs", ecsResourceAttribute.Str())

				beanstalkResourceAttribute, found := outputTraces.ResourceSpans().At(3).Resource().Attributes().Get("cloud.namespace")
				assert.True(t, found)
				assert.Equal(t, "ElasticBeanstalk", beanstalkResourceAttribute.Str())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newCloudNamespaceConfig(testCase.addCloudNamespace))

			// Act
			outputTraces, err := processor.processTraces(context.Background(), testCase.createTraces())
			require.NoError(t, err)

			// Assert
			testCase.test(outputTraces)
		})
	}
}

func TestTranslateAttributesForLogs(t *testing.T) {
	testCases := []struct {
		name                string
		translateAttributes bool
		createLogs          func() plog.Logs
		test                func(plog.Logs)
	}{
		{
			name:                "translates one attribute",
			translateAttributes: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId1")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId2")

				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				attribute1, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("AccountId")
				assert.True(t, found)
				assert.Equal(t, "MyId1", attribute1.Str())

				attribute2, found := outputLogs.ResourceLogs().At(1).Resource().Attributes().Get("AccountId")
				assert.True(t, found)
				assert.Equal(t, "MyId2", attribute2.Str())
			},
		},
		{
			name:                "does not translate",
			translateAttributes: false,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId1")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId2")

				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				attribute1, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("cloud.account.id")
				assert.True(t, found)
				assert.Equal(t, "MyId1", attribute1.Str())

				attribute2, found := outputLogs.ResourceLogs().At(1).Resource().Attributes().Get("cloud.account.id")
				assert.True(t, found)
				assert.Equal(t, "MyId2", attribute2.Str())
			},
		},
		{
			name:                "translates no attributes",
			translateAttributes: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("not.actual.attr", "a1")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("maybe.an.attr", "a2")
				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				attribute1, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("not.actual.attr")
				assert.True(t, found)
				assert.Equal(t, "a1", attribute1.Str())

				attribute2, found := outputLogs.ResourceLogs().At(1).Resource().Attributes().Get("maybe.an.attr")
				assert.True(t, found)
				assert.Equal(t, "a2", attribute2.Str())
			},
		},
		{
			name:                "translates many attributes, but not all",
			translateAttributes: true,
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("maybe.an.attr", "a2")
				inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("k8s.cluster.name", "A cool cluster")

				return inputLogs
			},
			test: func(outputLogs plog.Logs) {
				attribute1, found := outputLogs.ResourceLogs().At(0).Resource().Attributes().Get("AccountId")
				assert.True(t, found)
				assert.Equal(t, "MyId", attribute1.Str())

				attribute2, found := outputLogs.ResourceLogs().At(1).Resource().Attributes().Get("maybe.an.attr")
				assert.True(t, found)
				assert.Equal(t, "a2", attribute2.Str())

				attribute3, found := outputLogs.ResourceLogs().At(2).Resource().Attributes().Get("Cluster")
				assert.True(t, found)
				assert.Equal(t, "A cool cluster", attribute3.Str())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newTranslateAttributesConfig(testCase.translateAttributes))

			// Act
			outputLogs, err := processor.processLogs(context.Background(), testCase.createLogs())
			require.NoError(t, err)

			// Assert
			testCase.test(outputLogs)
		})
	}
}

func TestTranslateAttributesForMetrics(t *testing.T) {
	testCases := []struct {
		name                string
		translateAttributes bool
		createMetrics       func() pmetric.Metrics
		test                func(pmetric.Metrics)
	}{
		{
			name:                "translates many attributes, but not all",
			translateAttributes: true,
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("maybe.an.attr", "a2")
				inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("k8s.cluster.name", "A cool cluster")

				return inputMetrics
			},
			test: func(outputMetrics pmetric.Metrics) {
				attribute1, found := outputMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("AccountId")
				assert.True(t, found)
				assert.Equal(t, "MyId", attribute1.Str())

				attribute2, found := outputMetrics.ResourceMetrics().At(1).Resource().Attributes().Get("maybe.an.attr")
				assert.True(t, found)
				assert.Equal(t, "a2", attribute2.Str())

				attribute3, found := outputMetrics.ResourceMetrics().At(2).Resource().Attributes().Get("Cluster")
				assert.True(t, found)
				assert.Equal(t, "A cool cluster", attribute3.Str())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newTranslateAttributesConfig(testCase.translateAttributes))

			// Act
			outputMetrics, err := processor.processMetrics(context.Background(), testCase.createMetrics())
			require.NoError(t, err)

			// Assert
			testCase.test(outputMetrics)
		})
	}
}

func TestTranslateAttributesForTraces(t *testing.T) {
	// Traces are NOT translated.
	testCases := []struct {
		name                string
		translateAttributes bool
		createTraces        func() ptrace.Traces
		test                func(ptrace.Traces)
	}{
		{
			name:                "does not translate even translatable attributes",
			translateAttributes: true,
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("cloud.account.id", "MyId")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("maybe.an.attr", "a2")
				inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr("k8s.cluster.name", "A cool cluster")

				return inputTraces
			},
			test: func(outputTraces ptrace.Traces) {
				attribute1, found := outputTraces.ResourceSpans().At(0).Resource().Attributes().Get("cloud.account.id")
				assert.True(t, found)
				assert.Equal(t, "MyId", attribute1.Str())

				attribute2, found := outputTraces.ResourceSpans().At(1).Resource().Attributes().Get("maybe.an.attr")
				assert.True(t, found)
				assert.Equal(t, "a2", attribute2.Str())

				attribute3, found := outputTraces.ResourceSpans().At(2).Resource().Attributes().Get("k8s.cluster.name")
				assert.True(t, found)
				assert.Equal(t, "A cool cluster", attribute3.Str())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newTranslateAttributesConfig(testCase.translateAttributes))

			// Act
			outputTraces, err := processor.processTraces(context.Background(), testCase.createTraces())
			require.NoError(t, err)

			// Assert
			testCase.test(outputTraces)
		})
	}
}

func TestTranslateTelegrafMetrics(t *testing.T) {
	testCases := []struct {
		testName        string
		originalNames   []string
		translatedNames []string
		shouldTranslate bool
	}{
		{
			testName:        "translates two names",
			originalNames:   []string{"cpu_usage_irq", "system_load1"},
			translatedNames: []string{"CPU_Irq", "CPU_LoadAvg_1min"},
			shouldTranslate: true,
		},
		{
			testName:        "does not translate",
			originalNames:   []string{"cpu_usage_irq", "system_load1"},
			translatedNames: []string{"cpu_usage_irq", "system_load1"},
			shouldTranslate: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newTranslateTelegrafAttributesConfig(testCase.shouldTranslate))

			// Prepare metrics
			metrics := pmetric.NewMetrics()
			for _, name := range testCase.originalNames {
				metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(name)
			}

			// Act
			resultMetrics, err := processor.processMetrics(context.Background(), metrics)
			require.NoError(t, err)

			// Assert
			for index, name := range testCase.translatedNames {
				assert.Equal(t, name, resultMetrics.ResourceMetrics().At(index).ScopeMetrics().At(0).Metrics().At(0).Name())
			}
		})
	}
}

func TestTranslateDockerMetrics(t *testing.T) {
	testCases := []struct {
		testName                     string
		originalNames                []string
		translatedNames              []string
		originalResourceAttributes   map[string]string
		translatedResourceAttributes map[string]any
		shouldTranslate              bool
	}{
		{
			testName:        "translates two names",
			originalNames:   []string{"container.cpu.usage.percpu", "container.blockio.io_serviced_recursive"},
			translatedNames: []string{"cpu_usage.percpu_usage", "io_serviced_recursive"},
			originalResourceAttributes: map[string]string{
				"container.id":         "a",
				"container.image.name": "a",
				"container.name":       "a",
			},
			translatedResourceAttributes: map[string]any{
				"container.FullID":    "a",
				"container.ImageName": "a",
				"container.Name":      "a",
			},
			shouldTranslate: true,
		},
		{
			testName:        "does not translate",
			originalNames:   []string{"container.cpu.usage.percpu", "container.blockio.io_serviced_recursive"},
			translatedNames: []string{"container.cpu.usage.percpu", "container.blockio.io_serviced_recursive"},
			originalResourceAttributes: map[string]string{
				"container.id":         "a",
				"container.image.name": "a",
				"container.name":       "a",
			},
			translatedResourceAttributes: map[string]any{
				"container.id":         "a",
				"container.image.name": "a",
				"container.name":       "a",
			},
			shouldTranslate: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newTranslateDockerMetricsConfig(testCase.shouldTranslate))

			// Prepare metrics
			metrics := pmetric.NewMetrics()
			for _, name := range testCase.originalNames {
				metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(name)
			}

			// Prepare resource attributes
			ra := metrics.ResourceMetrics().At(0).Resource().Attributes()
			for k, v := range testCase.originalResourceAttributes {
				ra.PutStr(k, v)
			}

			// Act
			resultMetrics, err := processor.processMetrics(context.Background(), metrics)
			require.NoError(t, err)

			// Assert
			for index, name := range testCase.translatedNames {
				assert.Equal(t, name, resultMetrics.ResourceMetrics().At(index).ScopeMetrics().At(0).Metrics().At(0).Name())
			}
			assert.Equal(t, testCase.translatedResourceAttributes, metrics.ResourceMetrics().At(0).Resource().Attributes().AsRaw())
		})
	}
}

func TestNestingAttributesForLogs(t *testing.T) {
	testCases := []struct {
		name       string
		createLogs func() plog.Logs
		test       func(*testing.T, plog.Logs)
	}{
		{
			name: "sample nesting",
			createLogs: func() plog.Logs {
				attrs := mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.container_name": pcommon.NewValueStr("xyz"),
					"kubernetes.host.name":      pcommon.NewValueStr("the host"),
					"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
					"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
					"another_attr":              pcommon.NewValueStr("42"),
				})

				resourceAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"a": mapToPcommonValue(map[string]pcommon.Value{
						"b": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("d"),
						}),
					}),
					"a.b.c": pcommon.NewValueStr("d"),
					"d.g.e": pcommon.NewValueStr("l"),
					"b.g.c": pcommon.NewValueStr("bonus"),
				})

				logs := plog.NewLogs()
				resourceAttrs.CopyTo(logs.ResourceLogs().AppendEmpty().Resource().Attributes())
				attrs.CopyTo(logs.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes())

				return logs
			},
			test: func(t *testing.T, l plog.Logs) {
				expectedAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
						"container_name": pcommon.NewValueStr("xyz"),
						"namespace_name": pcommon.NewValueStr("sumologic"),
						"host": mapToPcommonValue(map[string]pcommon.Value{
							"name":    pcommon.NewValueStr("the host"),
							"address": pcommon.NewValueStr("127.0.0.1"),
						}),
					}),
					"another_attr": pcommon.NewValueStr("42"),
				})

				expectedResourceAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"a": mapToPcommonValue(map[string]pcommon.Value{
						"b": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("d"),
						}),
					}),
					"d": mapToPcommonValue(map[string]pcommon.Value{
						"g": mapToPcommonValue(map[string]pcommon.Value{
							"e": pcommon.NewValueStr("l"),
						}),
					}),
					"b": mapToPcommonValue(map[string]pcommon.Value{
						"g": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("bonus"),
						}),
					}),
				})

				require.Equal(t, 1, l.ResourceLogs().Len())
				require.Equal(t, expectedResourceAttrs.AsRaw(), l.ResourceLogs().At(0).Resource().Attributes().AsRaw())

				require.Equal(t, 1, l.ResourceLogs().At(0).ScopeLogs().Len())
				require.Equal(t, 1, l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
				require.Equal(t, expectedAttrs.AsRaw(), l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			processor := newsumologicProcessor(newProcessorCreateSettings(), newNestAttributesConfig(".", true))

			logs := testCase.createLogs()

			resultLogs, err := processor.processLogs(context.Background(), logs)
			require.NoError(t, err)

			testCase.test(t, resultLogs)
		})
	}
}

func TestNestingAttributesForMetrics(t *testing.T) {
	testCases := []struct {
		name          string
		createMetrics func() pmetric.Metrics
		test          func(*testing.T, pmetric.Metrics)
	}{
		{
			name: "sample nesting",
			createMetrics: func() pmetric.Metrics {
				attrs := mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.container_name": pcommon.NewValueStr("xyz"),
					"kubernetes.host.name":      pcommon.NewValueStr("the host"),
					"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
					"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
					"another_attr":              pcommon.NewValueStr("42"),
				})

				resourceAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"a": mapToPcommonValue(map[string]pcommon.Value{
						"b": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("d"),
						}),
					}),
					"a.b.c": pcommon.NewValueStr("d"),
					"d.g.e": pcommon.NewValueStr("l"),
					"b.g.c": pcommon.NewValueStr("bonus"),
				})

				metrics := pmetric.NewMetrics()
				resourceAttrs.CopyTo(metrics.ResourceMetrics().AppendEmpty().Resource().Attributes())
				metric := metrics.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				attrs.CopyTo(metric.SetEmptySum().DataPoints().AppendEmpty().Attributes())

				return metrics
			},
			test: func(t *testing.T, m pmetric.Metrics) {
				expectedAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
						"container_name": pcommon.NewValueStr("xyz"),
						"namespace_name": pcommon.NewValueStr("sumologic"),
						"host": mapToPcommonValue(map[string]pcommon.Value{
							"name":    pcommon.NewValueStr("the host"),
							"address": pcommon.NewValueStr("127.0.0.1"),
						}),
					}),
					"another_attr": pcommon.NewValueStr("42"),
				})

				expectedResourceAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"a": mapToPcommonValue(map[string]pcommon.Value{
						"b": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("d"),
						}),
					}),
					"d": mapToPcommonValue(map[string]pcommon.Value{
						"g": mapToPcommonValue(map[string]pcommon.Value{
							"e": pcommon.NewValueStr("l"),
						}),
					}),
					"b": mapToPcommonValue(map[string]pcommon.Value{
						"g": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("bonus"),
						}),
					}),
				})

				require.Equal(t, 1, m.ResourceMetrics().Len())
				require.Equal(t, expectedResourceAttrs.AsRaw(), m.ResourceMetrics().At(0).Resource().Attributes().AsRaw())

				require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().Len())
				require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
				require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().Len())

				sum := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				require.Equal(t, expectedAttrs.AsRaw(), sum.DataPoints().At(0).Attributes().AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			processor := newsumologicProcessor(newProcessorCreateSettings(), newNestAttributesConfig(".", true))

			metrics := testCase.createMetrics()

			resultMetrics, err := processor.processMetrics(context.Background(), metrics)
			require.NoError(t, err)

			testCase.test(t, resultMetrics)
		})
	}
}

func TestNestingAttributesForTraces(t *testing.T) {
	testCases := []struct {
		name         string
		createTraces func() ptrace.Traces
		test         func(*testing.T, ptrace.Traces)
	}{
		{
			name: "sample nesting",
			createTraces: func() ptrace.Traces {
				attrs := mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.container_name": pcommon.NewValueStr("xyz"),
					"kubernetes.host.name":      pcommon.NewValueStr("the host"),
					"kubernetes.host.address":   pcommon.NewValueStr("127.0.0.1"),
					"kubernetes.namespace_name": pcommon.NewValueStr("sumologic"),
					"another_attr":              pcommon.NewValueStr("42"),
				})

				resourceAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"a": mapToPcommonValue(map[string]pcommon.Value{
						"b": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("d"),
						}),
					}),
					"a.b.c": pcommon.NewValueStr("d"),
					"d.g.e": pcommon.NewValueStr("l"),
					"b.g.c": pcommon.NewValueStr("bonus"),
				})

				traces := ptrace.NewTraces()
				resourceAttrs.CopyTo(traces.ResourceSpans().AppendEmpty().Resource().Attributes())
				attrs.CopyTo(traces.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes())

				return traces
			},
			test: func(t *testing.T, l ptrace.Traces) {
				expectedAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes": mapToPcommonValue(map[string]pcommon.Value{
						"container_name": pcommon.NewValueStr("xyz"),
						"namespace_name": pcommon.NewValueStr("sumologic"),
						"host": mapToPcommonValue(map[string]pcommon.Value{
							"name":    pcommon.NewValueStr("the host"),
							"address": pcommon.NewValueStr("127.0.0.1"),
						}),
					}),
					"another_attr": pcommon.NewValueStr("42"),
				})

				expectedResourceAttrs := mapToPcommonMap(map[string]pcommon.Value{
					"a": mapToPcommonValue(map[string]pcommon.Value{
						"b": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("d"),
						}),
					}),
					"d": mapToPcommonValue(map[string]pcommon.Value{
						"g": mapToPcommonValue(map[string]pcommon.Value{
							"e": pcommon.NewValueStr("l"),
						}),
					}),
					"b": mapToPcommonValue(map[string]pcommon.Value{
						"g": mapToPcommonValue(map[string]pcommon.Value{
							"c": pcommon.NewValueStr("bonus"),
						}),
					}),
				})

				require.Equal(t, 1, l.ResourceSpans().Len())
				require.Equal(t, expectedResourceAttrs.AsRaw(), l.ResourceSpans().At(0).Resource().Attributes().AsRaw())

				require.Equal(t, 1, l.ResourceSpans().At(0).ScopeSpans().Len())
				require.Equal(t, 1, l.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
				require.Equal(t, expectedAttrs.AsRaw(), l.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			processor := newsumologicProcessor(newProcessorCreateSettings(), newNestAttributesConfig(".", true))

			traces := testCase.createTraces()

			resultTraces, err := processor.processTraces(context.Background(), traces)
			require.NoError(t, err)

			testCase.test(t, resultTraces)
		})
	}
}

// Tests for aggregating attributes.
// Testing different edge cases should be done in translate_attributes_processor_test.go,
// here are only e2e tests that mainly check if both resource and normal attributes are aggregated correctly.
func TestAggregateAttributesForLogs(t *testing.T) {
	testCases := []struct {
		name       string
		config     []aggregationPair
		createLogs func() plog.Logs
		test       func(plog.Logs)
	}{
		{
			name: "simple aggregation",
			config: []aggregationPair{
				{
					Attribute: "kubernetes.labels",
					Prefixes:  []string{"pod_labels_"},
				},
			},
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				resourceAttrs := inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes()
				resourceAttrs.PutStr("pod_labels_app", "demo")
				resourceAttrs.PutStr("pod_labels_pod_template_hash", "123456#&")
				resourceAttrs.PutStr("pod_labels_foo", "bar")

				logAttrs := inputLogs.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes()
				logAttrs.PutStr("pod_labels_log_id", "42")

				return inputLogs
			},
			test: func(l plog.Logs) {
				resourceAttrs := l.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, resourceAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.labels": mapToPcommonValue(map[string]pcommon.Value{
						"app":               pcommon.NewValueStr("demo"),
						"pod_template_hash": pcommon.NewValueStr("123456#&"),
						"foo":               pcommon.NewValueStr("bar"),
					}),
				}).AsRaw())

				logAttrs := l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
				require.Equal(t, logAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.labels": mapToPcommonValue(map[string]pcommon.Value{
						"log_id": pcommon.NewValueStr("42"),
					}),
				}).AsRaw())
			},
		},
		{
			name: "no-op",
			config: []aggregationPair{
				{
					Attribute: "kubernetes.labels",
					Prefixes:  []string{},
				},
			},
			createLogs: func() plog.Logs {
				inputLogs := plog.NewLogs()
				resourceAttrs := inputLogs.ResourceLogs().AppendEmpty().Resource().Attributes()
				resourceAttrs.PutStr("pod_labels_app", "demo")
				resourceAttrs.PutStr("pod_labels_pod_template_hash", "123456#&")
				resourceAttrs.PutStr("pod_labels_foo", "bar")

				logAttrs := inputLogs.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes()
				logAttrs.PutStr("pod_labels_log_id", "42")

				return inputLogs
			},
			test: func(l plog.Logs) {
				resourceAttrs := l.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, resourceAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"pod_labels_app":               pcommon.NewValueStr("demo"),
					"pod_labels_pod_template_hash": pcommon.NewValueStr("123456#&"),
					"pod_labels_foo":               pcommon.NewValueStr("bar"),
				}).AsRaw())

				logAttrs := l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
				require.Equal(t, logAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"pod_labels_log_id": pcommon.NewValueStr("42"),
				}).AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newAggregateAttributesConfig(testCase.config))

			// Act
			outputLogs, err := processor.processLogs(context.Background(), testCase.createLogs())
			require.NoError(t, err)

			// Assert
			testCase.test(outputLogs)
		})
	}
}

func TestAggregateAttributesForMetrics(t *testing.T) {
	testCases := []struct {
		name          string
		config        []aggregationPair
		createMetrics func() pmetric.Metrics
		test          func(pmetric.Metrics)
	}{
		{
			name: "simple aggregation",
			config: []aggregationPair{
				{
					Attribute: "kubernetes.labels",
					Prefixes:  []string{"pod_labels_"},
				},
			},
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				resourceAttrs := inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes()
				resourceAttrs.PutStr("pod_labels_app", "demo")
				resourceAttrs.PutStr("pod_labels_pod_template_hash", "123456#&")
				resourceAttrs.PutStr("pod_labels_foo", "bar")

				metric := inputMetrics.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics()
				// Test only for sum: tests for different metric types are in aggregate_attributes_processor_test.go
				metricAttrs := metric.AppendEmpty().SetEmptySum().DataPoints().AppendEmpty().Attributes()
				metricAttrs.PutStr("pod_labels_metric_id", "42")

				return inputMetrics
			},
			test: func(l pmetric.Metrics) {
				resourceAttrs := l.ResourceMetrics().At(0).Resource().Attributes()
				require.Equal(t, resourceAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.labels": mapToPcommonValue(map[string]pcommon.Value{
						"app":               pcommon.NewValueStr("demo"),
						"pod_template_hash": pcommon.NewValueStr("123456#&"),
						"foo":               pcommon.NewValueStr("bar"),
					}),
				}).AsRaw())

				metricAttrs := l.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
				require.Equal(t, metricAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.labels": mapToPcommonValue(map[string]pcommon.Value{
						"metric_id": pcommon.NewValueStr("42"),
					}),
				}).AsRaw())
			},
		},
		{
			name: "no-op",
			config: []aggregationPair{
				{
					Attribute: "kubernetes.labels",
					Prefixes:  []string{},
				},
			},
			createMetrics: func() pmetric.Metrics {
				inputMetrics := pmetric.NewMetrics()
				resourceAttrs := inputMetrics.ResourceMetrics().AppendEmpty().Resource().Attributes()
				resourceAttrs.PutStr("pod_labels_app", "demo")
				resourceAttrs.PutStr("pod_labels_pod_template_hash", "123456#&")
				resourceAttrs.PutStr("pod_labels_foo", "bar")

				metric := inputMetrics.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics()
				// Test only for sum: tests for different metric types are in aggregate_attributes_processor_test.go
				metricAttrs := metric.AppendEmpty().SetEmptySum().DataPoints().AppendEmpty().Attributes()
				metricAttrs.PutStr("pod_labels_metric_id", "42")

				return inputMetrics
			},
			test: func(l pmetric.Metrics) {
				resourceAttrs := l.ResourceMetrics().At(0).Resource().Attributes()
				require.Equal(t, resourceAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"pod_labels_app":               pcommon.NewValueStr("demo"),
					"pod_labels_pod_template_hash": pcommon.NewValueStr("123456#&"),
					"pod_labels_foo":               pcommon.NewValueStr("bar"),
				}).AsRaw())

				metricAttrs := l.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes()
				require.Equal(t, metricAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"pod_labels_metric_id": pcommon.NewValueStr("42"),
				}).AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newAggregateAttributesConfig(testCase.config))

			// Act
			outputMetrics, err := processor.processMetrics(context.Background(), testCase.createMetrics())
			require.NoError(t, err)

			// Assert
			testCase.test(outputMetrics)
		})
	}
}

func TestAggregateAttributesForTraces(t *testing.T) {
	testCases := []struct {
		name         string
		config       []aggregationPair
		createTraces func() ptrace.Traces
		test         func(ptrace.Traces)
	}{
		{
			name: "simple aggregation",
			config: []aggregationPair{
				{
					Attribute: "kubernetes.labels",
					Prefixes:  []string{"pod_labels_"},
				},
			},
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				resourceAttrs := inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes()
				resourceAttrs.PutStr("pod_labels_app", "demo")
				resourceAttrs.PutStr("pod_labels_pod_template_hash", "123456#&")
				resourceAttrs.PutStr("pod_labels_foo", "bar")

				logAttrs := inputTraces.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes()
				logAttrs.PutStr("pod_labels_span_id", "42")

				return inputTraces
			},
			test: func(l ptrace.Traces) {
				resourceAttrs := l.ResourceSpans().At(0).Resource().Attributes()
				require.Equal(t, resourceAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.labels": mapToPcommonValue(map[string]pcommon.Value{
						"app":               pcommon.NewValueStr("demo"),
						"pod_template_hash": pcommon.NewValueStr("123456#&"),
						"foo":               pcommon.NewValueStr("bar"),
					}),
				}).AsRaw())

				logAttrs := l.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
				require.Equal(t, logAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"kubernetes.labels": mapToPcommonValue(map[string]pcommon.Value{
						"span_id": pcommon.NewValueStr("42"),
					}),
				}).AsRaw())
			},
		},
		{
			name: "no-op",
			config: []aggregationPair{
				{
					Attribute: "kubernetes.labels",
					Prefixes:  []string{},
				},
			},
			createTraces: func() ptrace.Traces {
				inputTraces := ptrace.NewTraces()
				resourceAttrs := inputTraces.ResourceSpans().AppendEmpty().Resource().Attributes()
				resourceAttrs.PutStr("pod_labels_app", "demo")
				resourceAttrs.PutStr("pod_labels_pod_template_hash", "123456#&")
				resourceAttrs.PutStr("pod_labels_foo", "bar")

				logAttrs := inputTraces.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes()
				logAttrs.PutStr("pod_labels_span_id", "42")

				return inputTraces
			},
			test: func(l ptrace.Traces) {
				resourceAttrs := l.ResourceSpans().At(0).Resource().Attributes()
				require.Equal(t, resourceAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"pod_labels_app":               pcommon.NewValueStr("demo"),
					"pod_labels_pod_template_hash": pcommon.NewValueStr("123456#&"),
					"pod_labels_foo":               pcommon.NewValueStr("bar"),
				}).AsRaw())

				logAttrs := l.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
				require.Equal(t, logAttrs.AsRaw(), mapToPcommonMap(map[string]pcommon.Value{
					"pod_labels_span_id": pcommon.NewValueStr("42"),
				}).AsRaw())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newAggregateAttributesConfig(testCase.config))

			// Act
			outputTraces, err := processor.processTraces(context.Background(), testCase.createTraces())
			require.NoError(t, err)

			// Assert
			testCase.test(outputTraces)
		})
	}
}

func TestLogFieldsConversionLogs(t *testing.T) {
	testCases := []struct {
		name       string
		createLogs func() plog.Logs
		test       func(plog.Logs)
	}{
		{
			name: "logFieldsConversion",
			createLogs: func() plog.Logs {
				logs := plog.NewLogs()
				log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				log.SetSeverityNumber(plog.SeverityNumberInfo)
				log.SetSeverityText("severity")
				var spanIDBytes = [8]byte{1, 1, 1, 1, 1, 1, 1, 1}
				log.SetSpanID(spanIDBytes)
				var traceIDBytes = [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
				log.SetTraceID(traceIDBytes)
				return logs
			},
			test: func(outputLogs plog.Logs) {
				attribute1, found := outputLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("definitely_not_default_name")
				assert.True(t, found)
				assert.Equal(t, "INFO", attribute1.Str())
				attribute2, found := outputLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("severitytext")
				assert.True(t, found)
				assert.Equal(t, "severity", attribute2.Str())
				attribute3, found := outputLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("spanid")
				assert.True(t, found)
				assert.Equal(t, "0101010101010101", attribute3.Str())
				attribute4, found := outputLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("traceid")
				assert.True(t, found)
				assert.Equal(t, "01010101010101010101010101010101", attribute4.Str())

			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Arrange
			processor := newsumologicProcessor(newProcessorCreateSettings(), newLogFieldsConversionConfig())

			// Act
			outputLogs, err := processor.processLogs(context.Background(), testCase.createLogs())
			require.NoError(t, err)

			// Assert
			testCase.test(outputLogs)
		})
	}
}

func newProcessorCreateSettings() processor.CreateSettings {
	return processor.CreateSettings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}
}

func newCloudNamespaceConfig(addCloudNamespace bool) *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = addCloudNamespace
	config.TranslateAttributes = false
	config.TranslateTelegrafAttributes = false
	config.TranslateDockerMetrics = false
	config.NestAttributes = &NestingProcessorConfig{
		Enabled: false,
	}
	return config
}

func newTranslateAttributesConfig(translateAttributes bool) *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = false
	config.TranslateAttributes = translateAttributes
	config.TranslateTelegrafAttributes = false
	config.TranslateDockerMetrics = false
	config.NestAttributes = &NestingProcessorConfig{
		Enabled: false,
	}
	return config
}

func newTranslateTelegrafAttributesConfig(translateTelegrafAttributes bool) *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = false
	config.TranslateAttributes = false
	config.TranslateTelegrafAttributes = translateTelegrafAttributes
	config.TranslateDockerMetrics = false
	config.NestAttributes = &NestingProcessorConfig{
		Enabled: false,
	}
	return config
}

func newTranslateDockerMetricsConfig(translateDockerMetrics bool) *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = false
	config.TranslateAttributes = false
	config.TranslateTelegrafAttributes = false
	config.TranslateDockerMetrics = translateDockerMetrics
	config.NestAttributes = &NestingProcessorConfig{
		Enabled: false,
	}
	return config
}

func newNestAttributesConfig(separator string, enabled bool) *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = false
	config.TranslateAttributes = false
	config.TranslateTelegrafAttributes = false
	config.TranslateDockerMetrics = false
	config.NestAttributes = &NestingProcessorConfig{
		Separator: separator,
		Enabled:   enabled,
	}
	return config
}

func newAggregateAttributesConfig(aggregations []aggregationPair) *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = false
	config.TranslateAttributes = false
	config.TranslateTelegrafAttributes = false
	config.TranslateDockerMetrics = false
	config.AggregateAttributes = aggregations
	return config
}

func newLogFieldsConversionConfig() *Config {
	config := createDefaultConfig().(*Config)
	config.AddCloudNamespace = false
	config.TranslateAttributes = false
	config.TranslateTelegrafAttributes = false
	config.TranslateDockerMetrics = false
	config.NestAttributes = &NestingProcessorConfig{
		Enabled: false,
	}
	config.LogFieldsAttributes = &logFieldAttributesConfig{
		&logFieldAttribute{Enabled: true, Name: "definitely_not_default_name"},
		&logFieldAttribute{Enabled: true, Name: SeverityTextAttributeName},
		&logFieldAttribute{Enabled: true, Name: SpanIDAttributeName},
		&logFieldAttribute{Enabled: true, Name: TraceIDAttributeName},
	}
	return config
}
