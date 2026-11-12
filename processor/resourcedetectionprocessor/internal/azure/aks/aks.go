// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_azure_aks) && !omit_detector_azure_aks

package aks // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks"

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/contrib/detectors/azure/azureaks"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/aks/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

// Environment variable that is set when running on Kubernetes
const kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return azureaks.NewResourceDetector()
}

// Ensure Detector implements internal.Detector.
var _ internal.Detector = (*Detector)(nil)

// Detector is an AKS metadata detector. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector creates a new AKS detector
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect queries the Azure Instance Metadata Service and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Not running on Kubernetes is never a failure, even with fail_on_missing_metadata. The SDK
	// detector reports an empty resource here too, which is indistinguishable from an
	// unreachable metadata service, so this has to be decided before calling it.
	if !onK8s() {
		return pcommon.NewResource(), "", nil
	}

	// Detection runs unfiltered so that an empty result answers "is this process on AKS?"; the
	// configured attributes are applied afterwards. A partial result still came from a reachable
	// metadata service, so the bridge keeps what it did return. fail_on_missing_metadata covers
	// an unusable metadata service, not individual fields absent from its response.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("AKS metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error when the metadata
	// service is unreachable or answers with a client error.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("AKS detector: metadata unavailable or not running on Azure")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("aks metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}

func onK8s() bool {
	return os.Getenv(kubernetesServiceHostEnvVar) != ""
}
