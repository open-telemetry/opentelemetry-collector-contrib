// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"
	"regexp"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	localMetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
	processormetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr        = "gcp"
	gceLabelPrefix = "gcp.gce.instance.labels."
)

// NewDetector returns a detector which can detect resource attributes on:
// * Google Compute Engine (GCE).
// * Google Kubernetes Engine (GKE).
// * Google App Engine (GAE).
// * Cloud Run.
// * Cloud Functions.
// * Bare Metal Solutions (BMS).
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	labelKeyRegexes, err := compileLabelRegexes(cfg)
	if err != nil {
		return nil, err
	}

	return &detector{
		logger:           set.Logger,
		detector:         gcp.NewDetector(),
		rb:               localMetadata.NewResourceBuilder(cfg.ResourceAttributes),
		labelKeyRegexes:  labelKeyRegexes,
		gceClientBuilder: &instancesRESTBuilder{},
	}, nil
}

type detector struct {
	logger           *zap.Logger
	detector         gcpDetector
	rb               *localMetadata.ResourceBuilder
	labelKeyRegexes  []*regexp.Regexp
	gceClientBuilder instancesBuilder
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
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
		if !processormetadata.ProcessorResourcedetectionRemoveGCPFaasIDFeatureGate.IsEnabled() {
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
		if !processormetadata.ProcessorResourcedetectionRemoveGCPFaasIDFeatureGate.IsEnabled() {
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
		if !processormetadata.ProcessorResourcedetectionRemoveGCPFaasIDFeatureGate.IsEnabled() {
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
		if !processormetadata.ProcessorResourcedetectionRemoveGCPFaasIDFeatureGate.IsEnabled() {
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
		if !processormetadata.ProcessorResourcedetectionRemoveGCPFaasIDFeatureGate.IsEnabled() {
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
		res := d.rb.Emit()

		if len(d.labelKeyRegexes) == 0 {
			return res, conventions.SchemaURL, errs
		}

		projectID, perr := d.detector.ProjectID()
		zone, _, zerr := d.detector.GCEAvailabilityZoneAndRegion()
		name, nerr := d.detector.GCEInstanceName()
		if perr != nil || zerr != nil || nerr != nil {
			d.logger.Warn("failed reading GCE metadata for labels", zap.NamedError("project_id", perr), zap.NamedError("zone", zerr), zap.NamedError("instance_name", nerr))
			return res, conventions.SchemaURL, errs
		}

		instClient, cerr := d.gceClientBuilder.buildClient(ctx)
		if cerr != nil {
			d.logger.Warn("failed to build GCE instances client", zap.Error(cerr))
			return res, conventions.SchemaURL, errs
		}
		defer instClient.Close()

		labels, ferr := fetchGCELabels(ctx, instClient, projectID, zone, name, d.labelKeyRegexes)
		if ferr != nil {
			d.logger.Warn("failed fetching GCE labels", zap.Error(ferr))
			return res, conventions.SchemaURL, errs
		}

		if len(labels) > 0 {
			attrs := res.Attributes()
			for k, v := range labels {
				attrs.PutStr(gceLabelPrefix+k, v)
			}
		}

		return res, conventions.SchemaURL, errs
	default:
		// We don't support this platform yet, so just return with what we have
	}
	return d.rb.Emit(), conventions.SchemaURL, errs
}

type instancesAPI interface {
	Get(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error)
	Close() error
}

type instancesBuilder interface {
	buildClient(ctx context.Context) (instancesAPI, error)
}

type instancesRESTBuilder struct{}

func (*instancesRESTBuilder) buildClient(ctx context.Context) (instancesAPI, error) {
	cli, err := compute.NewInstancesRESTClient(ctx) // picks up GCE metadata creds automatically
	if err != nil {
		return nil, err
	}
	return &instancesRESTClient{inner: cli}, nil
}

type instancesRESTClient struct{ inner *compute.InstancesClient }

func (c *instancesRESTClient) Get(ctx context.Context, req *computepb.GetInstanceRequest) (*computepb.Instance, error) {
	return c.inner.Get(ctx, req)
}
func (c *instancesRESTClient) Close() error { return c.inner.Close() }

func fetchGCELabels(ctx context.Context, svc instancesAPI, project, zone, instance string, labelKeyRegexes []*regexp.Regexp) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	inst, err := svc.Get(ctx, &computepb.GetInstanceRequest{
		Project:  project,
		Zone:     zone,
		Instance: instance,
	})
	if err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for k, v := range inst.GetLabels() {
		if regexArrayMatch(labelKeyRegexes, k) {
			out[k] = v
		}
	}
	return out, nil
}

func compileLabelRegexes(cfg Config) ([]*regexp.Regexp, error) {
	rs := make([]*regexp.Regexp, len(cfg.Labels))
	for i, pat := range cfg.Labels {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		rs[i] = re
	}
	return rs, nil
}

func regexArrayMatch(arr []*regexp.Regexp, val string) bool {
	for _, r := range arr {
		if r.MatchString(val) {
			return true
		}
	}
	return false
}
