// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerapps // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	gocontribdetector "go.opentelemetry.io/contrib/detectors/azure/azurecontainerapps"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/containerapps/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

const (
	// TypeStr is type of detector.
	TypeStr = "azurecontainerapps"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	sdkDetector           sdkresource.Detector
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

func NewDetector(_ processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		sdkDetector:           gocontribdetector.NewResourceDetector(),
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect returns resource attributes when running on Azure Container Apps.
// Returns an empty resource when not running on Azure Container Apps, unless
// failOnMissingMetadata is set, in which case it returns an error.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Detection runs unfiltered so that an empty result answers "is this process on Azure
	// Container Apps?"; the configured attributes are applied afterwards.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.sdkDetector)
	if err != nil {
		return pcommon.NewResource(), "", err
	}

	if res.Attributes().Len() == 0 {
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("azure container apps metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
