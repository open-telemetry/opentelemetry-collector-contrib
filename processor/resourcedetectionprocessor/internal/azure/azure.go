// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"

import (
	"context"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventionsv134 "go.opentelemetry.io/otel/semconv/v1.34.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr   = "azure"
	tagPrefix = "azure.tag."
)

var _ internal.Detector = (*Detector)(nil)

// Detector is an Azure metadata detector
type Detector struct {
	provider      azure.Provider
	tagKeyRegexes []*regexp.Regexp
	logger        *zap.Logger
	rb            *metadata.ResourceBuilder
}

// NewDetector creates a new Azure metadata detector
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	tagKeyRegexes, err := compileRegexes(cfg)
	if err != nil {
		return nil, err
	}

	return &Detector{
		provider:      azure.NewProvider(),
		tagKeyRegexes: tagKeyRegexes,
		logger:        p.Logger,
		rb:            metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	compute, err := d.provider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Azure detector metadata retrieval failed", zap.Error(err))
		// return an empty Resource and no error
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAzure.Value.AsString())
	d.rb.SetCloudPlatform(conventionsv134.CloudPlatformAzureVM.Value.AsString())
	d.rb.SetHostName(compute.OSProfile.ComputerName)
	d.rb.SetCloudRegion(compute.Location)
	d.rb.SetHostID(compute.VMID)
	d.rb.SetCloudAccountID(compute.SubscriptionID)
	d.rb.SetCloudAvailabilityZone(compute.AvailabilityZone)

	// Also save compute.Name in "azure.vm.name" as host.id (AttributeHostName) is
	// used by system detector.
	d.rb.SetAzureVMName(compute.Name)
	d.rb.SetAzureVMSize(compute.VMSize)
	d.rb.SetAzureVMScalesetName(compute.VMScaleSetName)
	d.rb.SetAzureResourcegroupName(compute.ResourceGroupName)
	res := d.rb.Emit()

	if len(d.tagKeyRegexes) != 0 {
		tags := matchAzureTags(compute.TagsList, d.tagKeyRegexes)
		for key, val := range tags {
			res.Attributes().PutStr(tagPrefix+key, val)
		}
	}

	return res, conventions.SchemaURL, nil
}

func matchAzureTags(azureTags []azure.ComputeTagsListMetadata, tagKeyRegexes []*regexp.Regexp) map[string]string {
	tags := make(map[string]string)
	for _, tag := range azureTags {
		matched := regexArrayMatch(tagKeyRegexes, tag.Name)
		if matched {
			tags[tag.Name] = tag.Value
		}
	}
	return tags
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

func regexArrayMatch(arr []*regexp.Regexp, val string) bool {
	for _, elem := range arr {
		matched := elem.MatchString(val)
		if matched {
			return true
		}
	}
	return false
}
