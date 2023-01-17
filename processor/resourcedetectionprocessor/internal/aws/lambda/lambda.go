// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lambda // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "lambda"

	// Environment variables that are set when running on AWS Lambda.
	// awsRegionEnvVar                   = "AWS_REGION"
	// awsLambdaFunctionNameEnvVar       = "AWS_LAMBDA_FUNCTION_NAME"
	// awsLambdaFunctionVersionEnvVar    = "AWS_LAMBDA_FUNCTION_VERSION"
	// awsLambdaFunctionMemorySizeEnvVar = "AWS_LAMBDA_FUNCTION_MEMORY_SIZE"
	// awsLambdaLogGroupNameEnvVar       = "AWS_LAMBDA_LOG_GROUP_NAME"
	// awsLambdaLogStreamNameEnvVar      = "AWS_LAMBDA_LOG_STREAM_NAME"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	logger *zap.Logger
}

func NewDetector(set processor.CreateSettings, _ internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{logger: set.Logger}, nil
}

func (detector *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()

	// Check if running on Lambda.
	isLambda, err := isLambda(ctx)
	if !isLambda {
		detector.logger.Debug("Unable to identify AWS Lambda environment", zap.Error(err))
		return res, "", err
	}

	attr := res.Attributes()

	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/cloud.md
	attr.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.PutStr(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSLambda)
	attr.PutStr(conventions.AttributeCloudRegion, "TODO: AWS_REGION")

	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/faas.md
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/instrumentation/aws-lambda.md#resource-detector
	attr.PutStr(conventions.AttributeFaaSName, "TODO: AWS_LAMBDA_FUNCTION_NAME")
	attr.PutStr(conventions.AttributeFaaSVersion, "TODO: AWS_LAMBDA_FUNCTION_VERSION")
	attr.PutStr(conventions.AttributeFaaSInstance, "TODO: AWS_LAMBDA_LOG_STREAM_NAME")
	attr.PutStr(conventions.AttributeFaaSMaxMemory, "TODO: AWS_LAMBDA_FUNCTION_MEMORY_SIZE")

	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/cloud_provider/aws/logs.md
	logGroupNames := attr.PutEmptySlice(conventions.AttributeAWSLogGroupNames)
	logGroupNames.AppendEmpty().SetStr("TODO: AWS_LAMBDA_LOG_GROUP_NAME")

	logStreamNames := attr.PutEmptySlice(conventions.AttributeAWSLogStreamNames)
	logStreamNames.AppendEmpty().SetStr("TODO: AWS_LAMBDA_LOG_STREAM_NAME")

	return res, conventions.SchemaURL, nil
}

func isLambda(ctx context.Context) (bool, error) {
	// TODO
	return true, nil
}
