// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	localMetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "gcp"
)

var removeGCPFaasID = featuregate.GlobalRegistry().MustRegister(
	"processor.resourcedetection.removeGCPFaasID",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("Remove faas.id from the GCP detector. Use faas.instance instead."),
	featuregate.WithRegisterFromVersion("v0.87.0"))

// NewDetector returns a detector which can detect resource attributes on:
// * Google Compute Engine (GCE).
// * Google Kubernetes Engine (GKE).
// * Google App Engine (GAE).
// * Cloud Run.
// * Cloud Functions.
// * Bare Metal Solutions (BMS).
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
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
	if d.detector.CloudPlatform() == gcp.BareMetalSolution {
		d.rb.SetCloudProvider(conventions.CloudProviderGCP.Value.AsString())
		errs := d.rb.SetFromCallable(d.rb.SetCloudAccountID, d.detector.BareMetalSolutionProjectID)

		d.rb.SetCloudPlatform("gcp_bare_metal_solution")
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetHostName, d.detector.BareMetalSolutionInstanceID),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.BareMetalSolutionCloudRegion),
		)
		return d.rb.Emit(), conventions.SchemaURL, errs
	}

	if !metadata.OnGCE() {
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderGCP.Value.AsString())
	errs := d.rb.SetFromCallable(d.rb.SetCloudAccountID, d.detector.ProjectID)

	switch d.detector.CloudPlatform() {
	case gcp.GKE:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPKubernetesEngine.Value.AsString())
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
				zap.String("key", string(conventions.HostNameKey)), zap.Error(err))
		}
	case gcp.CloudRun:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPCloudRun.Value.AsString())
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.FaaSName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.FaaSVersion),
			d.rb.SetFromCallable(d.rb.SetFaasInstance, d.detector.FaaSID),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.FaaSCloudRegion),
		)
		if !removeGCPFaasID.IsEnabled() {
			errs = multierr.Combine(errs, d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.FaaSID))
		}
	case gcp.CloudRunJob:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPCloudRun.Value.AsString())
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.FaaSName),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.FaaSCloudRegion),
			d.rb.SetFromCallable(d.rb.SetFaasInstance, d.detector.FaaSID),
			d.rb.SetFromCallable(d.rb.SetGcpCloudRunJobExecution, d.detector.CloudRunJobExecution),
			d.rb.SetFromCallable(d.rb.SetGcpCloudRunJobTaskIndex, d.detector.CloudRunJobTaskIndex),
		)
		if !removeGCPFaasID.IsEnabled() {
			errs = multierr.Combine(errs, d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.FaaSID))
		}
	case gcp.CloudFunctions:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPCloudFunctions.Value.AsString())
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.FaaSName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.FaaSVersion),
			d.rb.SetFromCallable(d.rb.SetFaasInstance, d.detector.FaaSID),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.FaaSCloudRegion),
		)
		if !removeGCPFaasID.IsEnabled() {
			errs = multierr.Combine(errs, d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.FaaSID))
		}
	case gcp.AppEngineFlex:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPAppEngine.Value.AsString())
		errs = multierr.Combine(errs,
			d.rb.SetZoneAndRegion(d.detector.AppEngineFlexAvailabilityZoneAndRegion),
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.AppEngineServiceName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.AppEngineServiceVersion),
			d.rb.SetFromCallable(d.rb.SetFaasInstance, d.detector.AppEngineServiceInstance),
		)
		if !removeGCPFaasID.IsEnabled() {
			errs = multierr.Combine(errs, d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.AppEngineServiceInstance))
		}
	case gcp.AppEngineStandard:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPAppEngine.Value.AsString())
		errs = multierr.Combine(errs,
			d.rb.SetFromCallable(d.rb.SetFaasName, d.detector.AppEngineServiceName),
			d.rb.SetFromCallable(d.rb.SetFaasVersion, d.detector.AppEngineServiceVersion),
			d.rb.SetFromCallable(d.rb.SetFaasInstance, d.detector.AppEngineServiceInstance),
			d.rb.SetFromCallable(d.rb.SetCloudAvailabilityZone, d.detector.AppEngineStandardAvailabilityZone),
			d.rb.SetFromCallable(d.rb.SetCloudRegion, d.detector.AppEngineStandardCloudRegion),
		)
		if !removeGCPFaasID.IsEnabled() {
			errs = multierr.Combine(errs, d.rb.SetFromCallable(d.rb.SetFaasID, d.detector.AppEngineServiceInstance))
		}
	case gcp.GCE:
		d.rb.SetCloudPlatform(conventions.CloudPlatformGCPComputeEngine.Value.AsString())
		errs = multierr.Combine(errs,
			d.rb.SetZoneAndRegion(d.detector.GCEAvailabilityZoneAndRegion),
			d.rb.SetFromCallable(d.rb.SetHostType, d.detector.GCEHostType),
			d.rb.SetFromCallable(d.rb.SetHostID, d.detector.GCEHostID),
			d.rb.SetFromCallable(d.rb.SetHostName, d.detector.GCEHostName),
			d.rb.SetFromCallable(d.rb.SetGcpGceInstanceHostname, d.detector.GCEInstanceHostname),
			d.rb.SetFromCallable(d.rb.SetGcpGceInstanceName, d.detector.GCEInstanceName),
			d.rb.SetManagedInstanceGroup(d.detector.GCEManagedInstanceGroup),
		)
	default:
		// We don't support this platform yet, so just return with what we have
	}
	return d.rb.Emit(), conventions.SchemaURL, errs
}
