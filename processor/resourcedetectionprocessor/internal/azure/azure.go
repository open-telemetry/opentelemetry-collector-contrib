// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_azure) && !omit_detector_azure

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/contrib/detectors/azure/azurevm"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

const tagPrefix = "azure.tag."

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func(tagKeyFilter func(string) bool) sdkresource.Detector {
	return azurevm.NewResourceDetector(azurevm.WithTagKeyFilter(tagKeyFilter))
}

var _ internal.Detector = (*Detector)(nil)

// Detector is an Azure metadata detector. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector creates a new Azure metadata detector
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	tagKeyRegexes, err := compileRegexes(cfg)
	if err != nil {
		return nil, err
	}

	return &Detector{
		detector: newResourceDetector(func(key string) bool {
			return slices.ContainsFunc(tagKeyRegexes, func(re *regexp.Regexp) bool {
				return re.MatchString(key)
			})
		}),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	sdkRes, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("Azure detector metadata retrieval failed", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", fmt.Errorf("azure metadata unavailable: %w", err)
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error both when the
	// metadata service is unreachable and when running outside Azure.
	attrs := sdkRes.Attributes()
	if attrs.Len() == 0 {
		d.logger.Debug("Azure detector: metadata unavailable or not running on an Azure VM")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("azure metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	// The ResourceBuilder, not sdkbridge.RemoveDisabledAttributes, applies the config: the
	// resource group attribute is re-keyed below and the SDK reports attributes that this
	// detector does not declare.
	set := func(key string, setter func(string)) {
		if v, ok := attrs.Get(key); ok {
			setter(v.Str())
		}
	}
	set("cloud.provider", d.rb.SetCloudProvider)
	set("cloud.platform", d.rb.SetCloudPlatform)
	set("host.name", d.rb.SetHostName)
	set("cloud.region", d.rb.SetCloudRegion)
	set("host.id", d.rb.SetHostID)
	set("cloud.account.id", d.rb.SetCloudAccountID)
	set("cloud.availability_zone", d.rb.SetCloudAvailabilityZone)
	set("azure.vm.name", d.rb.SetAzureVMName)
	set("azure.vm.size", d.rb.SetAzureVMSize)
	set("azure.vm.scaleset.name", d.rb.SetAzureVMScalesetName)
	if metadata.ProcessorAzuredetectorRenameResourceGroupAttributeFeatureGate.IsEnabled() {
		set("azure.resource_group.name", d.rb.SetAzureResourceGroupName)
	} else {
		set("azure.resource_group.name", d.rb.SetAzureResourcegroupName)
	}
	res := d.rb.Emit()

	for key, val := range attrs.All() {
		if strings.HasPrefix(key, tagPrefix) {
			res.Attributes().PutStr(key, val.Str())
		}
	}

	return res, schemaURL, nil
}

func compileRegexes(cfg Config) ([]*regexp.Regexp, error) {
	tagRegexes := make([]*regexp.Regexp, len(cfg.Tags))
	for i, elem := range cfg.Tags {
		regex, err := regexp.Compile(elem)
		if err != nil {
			return nil, err
		}
		tagRegexes[i] = regex
	}
	return tagRegexes, nil
}
