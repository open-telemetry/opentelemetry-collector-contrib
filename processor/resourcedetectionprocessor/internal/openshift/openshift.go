// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.18.0"
	"go.uber.org/zap"

	ocp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openshift"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "openshift"
)

// NewDetector returns a detector which can detect resource attributes on OpenShift 4.
func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	userCfg := dcfg.(Config)

	if err := userCfg.MergeWithDefaults(); err != nil {
		return nil, err
	}

	tlsCfg, err := userCfg.TLSSettings.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	return &detector{
		logger:   set.Logger,
		provider: ocp.NewProvider(userCfg.Address, userCfg.Token, tlsCfg),
		rb:       metadata.NewResourceBuilder(userCfg.ResourceAttributes),
	}, nil
}

type detector struct {
	logger   *zap.Logger
	provider ocp.Provider
	rb       *metadata.ResourceBuilder
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	infra, err := d.provider.Infrastructure(ctx)
	if err != nil {
		d.logger.Error("OpenShift detector metadata retrieval failed", zap.Error(err))
		// return an empty Resource and no error
		return pcommon.NewResource(), "", nil
	}

	if infra.Status.InfrastructureName != "" {
		d.rb.SetK8sClusterName(infra.Status.InfrastructureName)
	}

	switch strings.ToLower(infra.Status.PlatformStatus.Type) {
	case "aws":
		d.rb.SetCloudProvider(conventions.AttributeCloudProviderAWS)
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformAWSOpenshift)
		d.rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.Aws.Region))
	case "azure":
		d.rb.SetCloudProvider(conventions.AttributeCloudProviderAzure)
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformAzureOpenshift)
		d.rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.Azure.CloudName))
	case "gcp":
		d.rb.SetCloudProvider(conventions.AttributeCloudProviderGCP)
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPOpenshift)
		d.rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.GCP.Region))
	case "ibmcloud":
		d.rb.SetCloudProvider(conventions.AttributeCloudProviderIbmCloud)
		d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformIbmCloudOpenshift)
		d.rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.IBMCloud.Location))
	case "openstack":
		d.rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.OpenStack.CloudName))
	}

	// TODO(frzifus): support conventions openshift and kubernetes cluster version.
	// SEE: https://github.com/open-telemetry/opentelemetry-specification/issues/2913

	return d.rb.Emit(), conventions.SchemaURL, nil
}
