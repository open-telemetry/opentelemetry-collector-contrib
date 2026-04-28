// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	vpcprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/vpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc/internal/metadata"
)

const (
	// TypeStr is the detector type string.
	TypeStr = "ibmcloud_vpc"
)

var _ internal.Detector = (*Detector)(nil)

// Detector queries the IBM Cloud VPC Instance Metadata Service and emits resource attributes.
type Detector struct {
	provider vpcprovider.Provider
	logger   *zap.Logger
	rb       *metadata.ResourceBuilder
}

// NewDetector creates an IBM Cloud VPC detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	switch cfg.Protocol {
	case "http", "https":
	default:
		return nil, fmt.Errorf("invalid protocol %q: must be \"http\" or \"https\"", cfg.Protocol)
	}

	return &Detector{
		provider: vpcprovider.NewProvider(cfg.Protocol),
		logger:   p.Logger,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects IBM Cloud VPC instance metadata and returns a resource with the available attributes.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	meta, err := d.provider.InstanceMetadata(ctx)
	if err != nil {
		d.logger.Debug("IBM Cloud VPC metadata not available", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderIBMCloud.Value.AsString())
	// TODO: Use semconv constant once CloudPlatformIBMCloudVPC is added.
	d.rb.SetCloudPlatform("ibm_cloud.vpc")
	d.rb.SetCloudRegion(regionFromZone(meta.Zone.Name))
	d.rb.SetCloudAvailabilityZone(meta.Zone.Name)
	d.rb.SetCloudAccountID(accountIDFromCRN(meta.CRN))
	d.rb.SetCloudResourceID(meta.CRN)
	d.rb.SetHostID(meta.ID)
	d.rb.SetHostImageID(meta.Image.ID)
	d.rb.SetHostImageName(meta.Image.Name)
	d.rb.SetHostName(meta.Name)
	d.rb.SetHostType(meta.Profile.Name)

	return d.rb.Emit(), conventions.SchemaURL, nil
}

// regionFromZone extracts the region from a zone name.
// Example: "us-south-1" -> "us-south", "eu-de-2" -> "eu-de"
func regionFromZone(zone string) string {
	idx := strings.LastIndex(zone, "-")
	if idx > 0 {
		return zone[:idx]
	}
	return zone
}

// accountIDFromCRN extracts the account ID from a CRN.
// CRN format: crn:v1:cname:ctype:service-name:region:account-id:service-instance:resource-type:resource-id
// Example: "crn:v1:bluemix:public:is:us-south-1:a/123456::instance:0717_xxx"
// The account segment is at index 6 and may be prefixed with "a/"
func accountIDFromCRN(crn string) string {
	parts := strings.Split(crn, ":")
	if len(parts) >= 7 {
		acct := parts[6]
		acct = strings.TrimPrefix(acct, "a/")
		return acct
	}
	return ""
}
