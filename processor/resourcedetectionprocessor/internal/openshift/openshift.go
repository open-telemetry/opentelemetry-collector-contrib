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
	res := pcommon.NewResource()
	attrs := res.Attributes()

	infra, err := d.provider.Infrastructure(ctx)
	if err != nil {
		d.logger.Error("OpenShift detector metadata retrieval failed", zap.Error(err))
		// return an empty Resource and no error
		return res, "", nil
	}

	var (
		region   string
		platform string
		provider string
	)

	switch strings.ToLower(infra.Status.PlatformStatus.Type) {
	case "aws":
		provider = conventions.AttributeCloudProviderAWS
		platform = conventions.AttributeCloudPlatformAWSOpenshift
		region = strings.ToLower(infra.Status.PlatformStatus.Aws.Region)
	case "azure":
		provider = conventions.AttributeCloudProviderAzure
		platform = conventions.AttributeCloudPlatformAzureOpenshift
		region = strings.ToLower(infra.Status.PlatformStatus.Azure.CloudName)
	case "gcp":
		provider = conventions.AttributeCloudProviderGCP
		platform = conventions.AttributeCloudPlatformGCPOpenshift
		region = strings.ToLower(infra.Status.PlatformStatus.GCP.Region)
	case "ibmcloud":
		provider = conventions.AttributeCloudProviderIbmCloud
		platform = conventions.AttributeCloudPlatformIbmCloudOpenshift
		region = strings.ToLower(infra.Status.PlatformStatus.IBMCloud.Location)
	case "openstack":
		region = strings.ToLower(infra.Status.PlatformStatus.OpenStack.CloudName)
	}

	if d.resourceAttributes.K8sClusterName.Enabled {
		if infra.Status.InfrastructureName != "" {
			attrs.PutStr(conventions.AttributeK8SClusterName, infra.Status.InfrastructureName)
		}
	}
	if d.resourceAttributes.CloudProvider.Enabled {
		if provider != "" {
			attrs.PutStr(conventions.AttributeCloudProvider, provider)
		}
	}
	if d.resourceAttributes.CloudPlatform.Enabled {
		if platform != "" {
			attrs.PutStr(conventions.AttributeCloudPlatform, platform)
		}
	}
	if d.resourceAttributes.CloudRegion.Enabled {
		if region != "" {
			attrs.PutStr(conventions.AttributeCloudRegion, region)
		}
	}

	// TODO(frzifus): support conventions openshift and kubernetes cluster version.
	// SEE: https://github.com/open-telemetry/opentelemetry-specification/issues/2913

	return res, conventions.SchemaURL, nil
}
