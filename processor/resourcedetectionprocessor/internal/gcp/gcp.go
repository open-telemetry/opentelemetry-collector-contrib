// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	gcpdetector "go.opentelemetry.io/contrib/detectors/gcp"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr        = "gcp"
	gceLabelPrefix = "gcp.gce.instance.labels."
)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = gcpdetector.NewDetector

var _ internal.Detector = (*Detector)(nil)

// Detector is a GCP metadata detector. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	labelKeyRegexes       []*regexp.Regexp
	gceClientBuilder      instancesBuilder
	failOnMissingMetadata bool
}

// NewDetector creates a new GCP metadata detector.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	labelKeyRegexes, err := compileLabelRegexes(cfg)
	if err != nil {
		return nil, err
	}

	return &Detector{
		detector:              newResourceDetector(),
		logger:                set.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		labelKeyRegexes:       labelKeyRegexes,
		gceClientBuilder:      &instancesRESTBuilder{},
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect queries the GCP metadata and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, err := d.detector.Detect(ctx)
	if err != nil {
		d.logger.Debug("GCP metadata unavailable", zap.Error(err))
		// A partial result still came from a reachable metadata service, so keep
		// what it did return. fail_on_missing_metadata covers an unusable
		// metadata service, not individual fields absent from its response.
		if !errors.Is(err, sdkresource.ErrPartialResource) {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", fmt.Errorf("gcp metadata unavailable: %w", err)
			}
			return pcommon.NewResource(), "", nil
		}
	}

	// The SDK detector reports an empty resource (or nil) and no error both when the
	// metadata service is unreachable and when not running on a GCP environment.
	if res == nil || res.Len() == 0 {
		d.logger.Debug("GCP detector: metadata unavailable or not running on a GCP environment")
		if d.failOnMissingMetadata {
			if err != nil {
				return pcommon.NewResource(), "", fmt.Errorf("gcp metadata unavailable: %w", err)
			}
			return pcommon.NewResource(), "", errors.New("gcp metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	var isGCE bool
	var projectID, zone, instanceName string
	for _, attr := range res.Attributes() {
		val := attr.Value.AsString()
		switch attr.Key {
		case conventions.CloudProviderKey:
			d.rb.SetCloudProvider(val)
		case conventions.CloudAccountIDKey:
			projectID = val
			d.rb.SetCloudAccountID(val)
		case conventions.CloudPlatformKey:
			if val == conventions.CloudPlatformGCPComputeEngine.Value.AsString() {
				isGCE = true
			}
			d.rb.SetCloudPlatform(val)
		case conventions.CloudAvailabilityZoneKey:
			zone = val
			d.rb.SetCloudAvailabilityZone(val)
		case conventions.CloudRegionKey:
			d.rb.SetCloudRegion(val)
		case conventions.FaaSInstanceKey:
			d.rb.SetFaasInstance(val)
		case conventions.FaaSNameKey:
			d.rb.SetFaasName(val)
		case conventions.FaaSVersionKey:
			d.rb.SetFaasVersion(val)
		case "gcp.cloud_run.job.execution":
			d.rb.SetGcpCloudRunJobExecution(val)
		case "gcp.cloud_run.job.task_index":
			d.rb.SetGcpCloudRunJobTaskIndex(val)
		case "gcp.gce.instance.hostname":
			d.rb.SetGcpGceInstanceHostname(val)
		case "gcp.gce.instance.name":
			instanceName = val
			d.rb.SetGcpGceInstanceName(val)
		case "gcp.gce.instance_group_manager.name":
			d.rb.SetGcpGceInstanceGroupManagerName(val)
		case "gcp.gce.instance_group_manager.region":
			d.rb.SetGcpGceInstanceGroupManagerRegion(val)
		case "gcp.gce.instance_group_manager.zone":
			d.rb.SetGcpGceInstanceGroupManagerZone(val)
		case conventions.HostIDKey:
			d.rb.SetHostID(val)
		case conventions.HostNameKey:
			if instanceName == "" {
				instanceName = val
			}
			d.rb.SetHostName(val)
		case conventions.HostTypeKey:
			d.rb.SetHostType(val)
		case conventions.K8SClusterNameKey:
			d.rb.SetK8sClusterName(val)
		}
	}

	emittedRes := d.rb.Emit()

	if isGCE && len(d.labelKeyRegexes) > 0 {
		if projectID != "" && zone != "" && instanceName != "" {
			instClient, cerr := d.gceClientBuilder.buildClient(ctx)
			if cerr != nil {
				d.logger.Warn("failed to build GCE instances client", zap.Error(cerr))
			} else {
				defer instClient.Close()
				labels, ferr := fetchGCELabels(ctx, instClient, projectID, zone, instanceName, d.labelKeyRegexes)
				if ferr != nil {
					d.logger.Warn("failed fetching GCE labels", zap.Error(ferr))
				} else if len(labels) > 0 {
					attrs := emittedRes.Attributes()
					for k, v := range labels {
						attrs.PutStr(gceLabelPrefix+k, v)
					}
				}
			}
		}
	}

	return emittedRes, conventions.SchemaURL, nil
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
