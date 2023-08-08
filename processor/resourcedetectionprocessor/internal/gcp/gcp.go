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
		logger:   set.Logger,
		detector: gcp.NewDetector(),
		rb:       localMetadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

type detector struct {
	logger   *zap.Logger
	detector gcpDetector
	rb       *localMetadata.ResourceBuilder
}

func (d *detector) Detect(context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if !metadata.OnGCE() {
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.AttributeCloudProviderGCP)
	errs := d.rb.SetFromCallable(d.rb.SetCloudAccountID, d.detector.ProjectID)

	switch d.detector.CloudPlatform() {
	case gcp.GKE:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPKubernetesEngine)
		errs = multierr.Combine(errs,
			d.rb.SetZoneOrRegion(d.detector.GKEAvailabilityZoneOrRegion),
			d.rb.SetFromCallable(d.rb.SetK8sClusterName, d.detector.GKEClusterName),
			d.rb.SetFromCallable(d.rb.SetHostID, d.detector.GKEHostID),
		)
		// GCEHostname is fallible on GKE, since it's not available when using workload identity.
		if v, err := d.detector.GCEHostName(); err == nil {
			d.rb.SetHostName(v)
		} else {
			d.logger.Info("Fallible detector failed. This attribute will not be available.",
				zap.String("key", conventions.AttributeHostName), zap.Error(err))
		}
	case gcp.CloudRun:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPCloudRun)
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.FaaSName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.FaaSVersion),
			d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.FaaSID),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.FaaSCloudRegion),
		)
	case gcp.CloudRunJob:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPCloudRun)
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.FaaSName),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.FaaSCloudRegion),
			d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.FaaSID),
			d.rb.SetFromCallable(d.rb.SetGcpCloudRunJobExecution, d.detector.CloudRunJobExecution),
			d.rb.SetFromCallable(d.rb.SetGcpCloudRunJobTaskIndex, d.detector.CloudRunJobTaskIndex),
		)
	case gcp.CloudFunctions:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPCloudFunctions)
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.FaaSName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.FaaSVersion),
			d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.FaaSID),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.FaaSCloudRegion),
		)
	case gcp.AppEngineFlex:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPAppEngine)
		errs = multierr.Combine(errs,
			d.rb.SetZoneAndRegion(d.detector.AppEngineFlexAvailabilityZoneAndRegion),
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.AppEngineServiceName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.AppEngineServiceVersion),
			d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.AppEngineServiceInstance),
		)
	case gcp.AppEngineStandard:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPAppEngine)
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.AppEngineServiceName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.AppEngineServiceVersion),
			d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.AppEngineServiceInstance),
			d.rb.SetFromCallable(d.rb.SetCloudAvailabilityZone, d.detector.AppEngineStandardAvailabilityZone),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.AppEngineStandardCloudRegion),
		)
	case gcp.GCE:
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPComputeEngine)
		errs = multierr.Combine(errs,
			d.rb.SetZoneAndRegion(d.detector.GCEAvailabilityZoneAndRegion),
			d.rb.SetFromCallable(d.rb.SetHostType, d.detector.GCEHostType),
			d.rb.SetFromCallable(d.rb.SetHostID, d.detector.GCEHostID),
			d.rb.SetFromCallable(d.rb.SetHostName, d.detector.GCEHostName),
			d.rb.SetFromCallable(d.rb.SetGcpGceInstanceHostname, d.detector.GCEInstanceHostname),
			d.rb.SetFromCallable(d.rb.SetGcpGceInstanceName, d.detector.GCEInstanceName),
		)
	default:
		// We don't support this platform yet, so just return with what we have
	}
	return d.rb.Emit(), conventions.SchemaURL, errs
}
