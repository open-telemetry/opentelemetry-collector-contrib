// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"
	"fmt"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "gcp"
	// 'gke' and 'gce' detectors are replaced with the unified 'gcp' detector
	// TODO(#10348): Remove these after the v0.54.0 release.
	DeprecatedGKETypeStr = "gke"
	DeprecatedGCETypeStr = "gce"
)

// NewDetector returns a detector which can detect resource attributes on:
// * Google Compute Engine (GCE).
// * Google Kubernetes Engine (GKE).
// * Google App Engine (GAE).
// * Cloud Run.
// * Cloud Functions.
func NewDetector(set component.ProcessorCreateSettings, _ internal.DetectorConfig) (internal.Detector, error) {
	return &detector{
		logger:   set.Logger,
		detector: gcp.NewDetector(),
	}, nil
}

type detector struct {
	logger   *zap.Logger
	detector gcpDetector
}

func (d *detector) Detect(context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()
	if !metadata.OnGCE() {
		return res, "", nil
	}
	b := &resourceBuilder{logger: d.logger, attrs: res.Attributes()}
	b.attrs.UpsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
	b.add(conventions.AttributeCloudAccountID, d.detector.ProjectID)

	switch d.detector.CloudPlatform() {
	case gcp.GKE:
		b.attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPKubernetesEngine)
		b.addZoneOrRegion(d.detector.GKEAvailabilityZoneOrRegion)
		b.add(conventions.AttributeK8SClusterName, d.detector.GKEClusterName)
		b.add(conventions.AttributeHostID, d.detector.GKEHostID)
		// GCEHostname is fallible on GKE, since it's not available when using workload identity.
		b.addFallible(conventions.AttributeHostName, d.detector.GCEHostName)
	case gcp.CloudRun:
		b.attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPCloudRun)
		b.add(conventions.AttributeFaaSName, d.detector.FaaSName)
		b.add(conventions.AttributeFaaSVersion, d.detector.FaaSVersion)
		b.add(conventions.AttributeFaaSID, d.detector.FaaSID)
		b.add(conventions.AttributeCloudRegion, d.detector.FaaSCloudRegion)
	case gcp.CloudFunctions:
		b.attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPCloudFunctions)
		b.add(conventions.AttributeFaaSName, d.detector.FaaSName)
		b.add(conventions.AttributeFaaSVersion, d.detector.FaaSVersion)
		b.add(conventions.AttributeFaaSID, d.detector.FaaSID)
		b.add(conventions.AttributeCloudRegion, d.detector.FaaSCloudRegion)
	case gcp.AppEngineFlex:
		b.attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPAppEngine)
		b.addZoneAndRegion(d.detector.AppEngineFlexAvailabilityZoneAndRegion)
		b.add(conventions.AttributeFaaSName, d.detector.AppEngineServiceName)
		b.add(conventions.AttributeFaaSVersion, d.detector.AppEngineServiceVersion)
		b.add(conventions.AttributeFaaSID, d.detector.AppEngineServiceInstance)
	case gcp.AppEngineStandard:
		b.attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPAppEngine)
		b.add(conventions.AttributeFaaSName, d.detector.AppEngineServiceName)
		b.add(conventions.AttributeFaaSVersion, d.detector.AppEngineServiceVersion)
		b.add(conventions.AttributeFaaSID, d.detector.AppEngineServiceInstance)
		b.add(conventions.AttributeCloudAvailabilityZone, d.detector.AppEngineStandardAvailabilityZone)
		b.add(conventions.AttributeCloudRegion, d.detector.AppEngineStandardCloudRegion)
	case gcp.GCE:
		b.attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPComputeEngine)
		b.addZoneAndRegion(d.detector.GCEAvailabilityZoneAndRegion)
		b.add(conventions.AttributeHostType, d.detector.GCEHostType)
		b.add(conventions.AttributeHostID, d.detector.GCEHostID)
		b.add(conventions.AttributeHostName, d.detector.GCEHostName)
	default:
		// We don't support this platform yet, so just return with what we have
	}
	return res, conventions.SchemaURL, multierr.Combine(b.errs...)
}

// resourceBuilder simplifies constructing resources using GCP detection
// library functions.
type resourceBuilder struct {
	logger *zap.Logger
	errs   []error
	attrs  pcommon.Map
}

func (r *resourceBuilder) add(key string, detect func() (string, error)) {
	v, err := detect()
	if err != nil {
		r.errs = append(r.errs, err)
		return
	}
	r.attrs.UpsertString(key, v)
}

// addFallible adds a detect function whose failures should be ignored
func (r *resourceBuilder) addFallible(key string, detect func() (string, error)) {
	v, err := detect()
	if err != nil {
		r.logger.Info("Fallible detector failed. This attribute will not be available.", zap.String("key", key), zap.Error(err))
		return
	}
	r.attrs.UpsertString(key, v)
}

// zoneAndRegion functions are expected to return zone, region, err.
func (r *resourceBuilder) addZoneAndRegion(detect func() (string, string, error)) {
	zone, region, err := detect()
	if err != nil {
		r.errs = append(r.errs, err)
		return
	}
	r.attrs.UpsertString(conventions.AttributeCloudAvailabilityZone, zone)
	r.attrs.UpsertString(conventions.AttributeCloudRegion, region)
}

func (r *resourceBuilder) addZoneOrRegion(detect func() (string, gcp.LocationType, error)) {
	v, locType, err := detect()
	if err != nil {
		r.errs = append(r.errs, err)
		return
	}

	switch locType {
	case gcp.Zone:
		r.attrs.UpsertString(conventions.AttributeCloudAvailabilityZone, v)
	case gcp.Region:
		r.attrs.UpsertString(conventions.AttributeCloudRegion, v)
	default:
		r.errs = append(r.errs, fmt.Errorf("location must be zone or region. Got %v", locType))
	}
}

// DeduplicateDetectors ensures only one of ['gcp','gke','gce'] are present in
// the list of detectors. Currently, users configure both GCE and GKE detectors
// when running on GKE. Resource merge would fail in this case if we don't
// deduplicate, which would break users.
// TODO(#10348): Remove this function after the v0.54.0 release.
func DeduplicateDetectors(set component.ProcessorCreateSettings, detectors []string) []string {
	var out []string
	var found bool
	for _, d := range detectors {
		switch d {
		case DeprecatedGKETypeStr:
			set.Logger.Warn("The 'gke' detector is deprecated.  Use the 'gcp' detector instead.")
		case DeprecatedGCETypeStr:
			set.Logger.Warn("The 'gce' detector is deprecated.  Use the 'gcp' detector instead.")
		case TypeStr:
		default:
			out = append(out, d)
			continue
		}
		// ensure we only keep the first GCP detector we find.
		if !found {
			found = true
			out = append(out, d)
		}
	}
	return out
}
