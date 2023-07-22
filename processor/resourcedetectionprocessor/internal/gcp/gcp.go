// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	localMetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "gcp"
)

// NewDetector returns a detector which can detect resource attributes on:
// * Google Compute Engine (GCE).
// * Google Kubernetes Engine (GKE).
// * Google App Engine (GAE).
// * Cloud Run.
// * Cloud Functions.
func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &detector{
		logger:             set.Logger,
		detector:           gcp.NewDetector(),
		resourceAttributes: cfg.ResourceAttributes,
	}, nil
}

type detector struct {
	logger             *zap.Logger
	detector           gcpDetector
	resourceAttributes localMetadata.ResourceAttributesConfig
}

func (d *detector) Detect(context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if !metadata.OnGCE() {
		return pcommon.NewResource(), "", nil
	}

	rb := localMetadata.NewResourceBuilder(d.resourceAttributes)

	rb.SetCloudProvider(conventions.AttributeCloudProviderGCP)
	errs := rb.SetFromCallable(rb.SetCloudAccountID, d.detector.ProjectID)

	switch d.detector.CloudPlatform() {
	case gcp.GKE:
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPKubernetesEngine)
		errs = multierr.Combine(errs,
			rb.SetZoneOrRegion(d.detector.GKEAvailabilityZoneOrRegion),
			rb.SetFromCallable(rb.SetK8sClusterName, d.detector.GKEClusterName),
			rb.SetFromCallable(rb.SetHostID, d.detector.GKEHostID),
		)
		// GCEHostname is fallible on GKE, since it's not available when using workload identity.
		if v, err := d.detector.GCEHostName(); err == nil {
			rb.SetHostName(v)
		} else {
			d.logger.Info("Fallible detector failed. This attribute will not be available.",
				zap.String("key", conventions.AttributeHostName), zap.Error(err))
		}
	case gcp.CloudRun:
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPCloudRun)
		errs = multierr.Combine(errs,
			rb.SetFromCallable(rb.SetFaasName, d.detector.FaaSName),
			rb.SetFromCallable(rb.SetFaasVersion, d.detector.FaaSVersion),
			rb.SetFromCallable(rb.SetFaasID, d.detector.FaaSID),
			rb.SetFromCallable(rb.SetCloudRegion, d.detector.FaaSCloudRegion),
		)
	case gcp.CloudFunctions:
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPCloudFunctions)
		errs = multierr.Combine(errs,
			rb.SetFromCallable(rb.SetFaasName, d.detector.FaaSName),
			rb.SetFromCallable(rb.SetFaasVersion, d.detector.FaaSVersion),
			rb.SetFromCallable(rb.SetFaasID, d.detector.FaaSID),
			rb.SetFromCallable(rb.SetCloudRegion, d.detector.FaaSCloudRegion),
		)
	case gcp.AppEngineFlex:
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPAppEngine)
		errs = multierr.Combine(errs,
			rb.SetZoneAndRegion(d.detector.AppEngineFlexAvailabilityZoneAndRegion),
			rb.SetFromCallable(rb.SetFaasName, d.detector.AppEngineServiceName),
			rb.SetFromCallable(rb.SetFaasVersion, d.detector.AppEngineServiceVersion),
			rb.SetFromCallable(rb.SetFaasID, d.detector.AppEngineServiceInstance),
		)
	case gcp.AppEngineStandard:
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPAppEngine)
		errs = multierr.Combine(errs,
			rb.SetFromCallable(rb.SetFaasName, d.detector.AppEngineServiceName),
			rb.SetFromCallable(rb.SetFaasVersion, d.detector.AppEngineServiceVersion),
			rb.SetFromCallable(rb.SetFaasID, d.detector.AppEngineServiceInstance),
			rb.SetFromCallable(rb.SetCloudAvailabilityZone, d.detector.AppEngineStandardAvailabilityZone),
			rb.SetFromCallable(rb.SetCloudRegion, d.detector.AppEngineStandardCloudRegion),
		)
	case gcp.GCE:
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPComputeEngine)
		errs = multierr.Combine(errs,
			rb.SetZoneAndRegion(d.detector.GCEAvailabilityZoneAndRegion),
			rb.SetFromCallable(rb.SetHostType, d.detector.GCEHostType),
			rb.SetFromCallable(rb.SetHostID, d.detector.GCEHostID),
			rb.SetFromCallable(rb.SetHostName, d.detector.GCEHostName),
		)
	default:
		// We don't support this platform yet, so just return with what we have
	}
	return rb.Emit(), conventions.SchemaURL, errs
}
