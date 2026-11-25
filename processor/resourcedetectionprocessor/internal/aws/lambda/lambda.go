// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_aws_lambda) && !omit_detector_aws_lambda

package lambda // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	lambdadetector "go.opentelemetry.io/contrib/detectors/aws/lambda"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

var _ internal.Detector = (*detector)(nil)

type detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &detector{
		detector:              lambdadetector.NewResourceDetector(),
		logger:                set.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

func (d *detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		return pcommon.NewResource(), "", err
	}

	if res.Attributes().Len() == 0 {
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("lambda metadata unavailable: AWS_LAMBDA_FUNCTION_NAME env var not set")
		}
		d.logger.Debug("Unable to identify AWS Lambda environment")
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
