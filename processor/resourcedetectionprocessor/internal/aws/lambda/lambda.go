// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "lambda"

	// Environment variables that are set when running on AWS Lambda.
	// https://docs.aws.amazon.com/lambda/latest/dg/configuration-envvars.html#configuration-envvars-runtime
	awsRegionEnvVar                   = "AWS_REGION"
	awsLambdaFunctionNameEnvVar       = "AWS_LAMBDA_FUNCTION_NAME"
	awsLambdaFunctionVersionEnvVar    = "AWS_LAMBDA_FUNCTION_VERSION"
	awsLambdaFunctionMemorySizeEnvVar = "AWS_LAMBDA_FUNCTION_MEMORY_SIZE"
	awsLambdaLogGroupNameEnvVar       = "AWS_LAMBDA_LOG_GROUP_NAME"
	awsLambdaLogStreamNameEnvVar      = "AWS_LAMBDA_LOG_STREAM_NAME"
)

var _ internal.Detector = (*detector)(nil)

type detector struct {
	logger             *zap.Logger
	resourceAttributes metadata.ResourceAttributesConfig
}

func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &detector{logger: set.Logger, resourceAttributes: cfg.ResourceAttributes}, nil
}

func (d *detector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	functionName, ok := os.LookupEnv(awsLambdaFunctionNameEnvVar)
	if !ok || functionName == "" {
		d.logger.Debug("Unable to identify AWS Lambda environment", zap.Error(err))
		return pcommon.NewResource(), "", err
	}

	rb := metadata.NewResourceBuilder(d.resourceAttributes)

	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/cloud.md
	rb.SetCloudProvider(conventions.AttributeCloudProviderAWS)
	rb.SetCloudPlatform(conventions.AttributeCloudPlatformAWSLambda)
	if value, ok := os.LookupEnv(awsRegionEnvVar); ok {
		rb.SetCloudRegion(value)
	}

	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/faas.md
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/instrumentation/aws-lambda.md#resource-detector
	rb.SetFaasName(functionName)
	if value, ok := os.LookupEnv(awsLambdaFunctionVersionEnvVar); ok {
		rb.SetFaasVersion(value)
	}

	// Note: The FaaS spec (https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/faas.md)
	//       recommends setting faas.instance to the full log stream name for AWS Lambda.
	if value, ok := os.LookupEnv(awsLambdaLogStreamNameEnvVar); ok {
		rb.SetFaasInstance(value)
	}
	if value, ok := os.LookupEnv(awsLambdaFunctionMemorySizeEnvVar); ok {
		rb.SetFaasMaxMemory(value)
	}

	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/cloud_provider/aws/logs.md
	if value, ok := os.LookupEnv(awsLambdaLogGroupNameEnvVar); ok {
		rb.SetAwsLogGroupNames([]any{value})
	}
	if value, ok := os.LookupEnv(awsLambdaLogStreamNameEnvVar); ok {
		rb.SetAwsLogStreamNames([]any{value})
	}

	return rb.Emit(), conventions.SchemaURL, nil
}
