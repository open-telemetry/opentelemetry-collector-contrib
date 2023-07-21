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
		logger:             set.Logger,
		provider:           ocp.NewProvider(userCfg.Address, userCfg.Token, tlsCfg),
		resourceAttributes: userCfg.ResourceAttributes,
	}, nil
}

type detector struct {
	logger             *zap.Logger
	provider           ocp.Provider
	resourceAttributes metadata.ResourceAttributesConfig
}

func (d *detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	infra, err := d.provider.Infrastructure(ctx)
	if err != nil {
		d.logger.Error("OpenShift detector metadata retrieval failed", zap.Error(err))
		// return an empty Resource and no error
		return pcommon.NewResource(), "", nil
	}

	rb := metadata.NewResourceBuilder(d.resourceAttributes)

	if infra.Status.InfrastructureName != "" {
		rb.SetK8sClusterName(infra.Status.InfrastructureName)
	}

	switch strings.ToLower(infra.Status.PlatformStatus.Type) {
	case "aws":
		rb.SetCloudProvider(conventions.AttributeCloudProviderAWS)
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformAWSOpenshift)
		rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.Aws.Region))
	case "azure":
		rb.SetCloudProvider(conventions.AttributeCloudProviderAzure)
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformAzureOpenshift)
		rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.Azure.CloudName))
	case "gcp":
		rb.SetCloudProvider(conventions.AttributeCloudProviderGCP)
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformGCPOpenshift)
		rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.GCP.Region))
	case "ibmcloud":
		rb.SetCloudProvider(conventions.AttributeCloudProviderIbmCloud)
		rb.SetCloudPlatform(conventions.AttributeCloudPlatformIbmCloudOpenshift)
		rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.IBMCloud.Location))
	case "openstack":
		rb.SetCloudRegion(strings.ToLower(infra.Status.PlatformStatus.OpenStack.CloudName))
	}

	// TODO(frzifus): support conventions openshift and kubernetes cluster version.
	// SEE: https://github.com/open-telemetry/opentelemetry-specification/issues/2913

	return rb.Emit(), conventions.SchemaURL, nil
}
