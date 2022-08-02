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

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "azure"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is an Azure metadata detector
type Detector struct {
	provider azure.Provider
	logger   *zap.Logger
}

// NewDetector creates a new Azure metadata detector
func NewDetector(p component.ProcessorCreateSettings, cfg internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{
		provider: azure.NewProvider(),
		logger:   p.Logger,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()
	attrs := res.Attributes()

	compute, err := d.provider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Azure detector metadata retrieval failed", zap.Error(err))
		// return an empty Resource and no error
		return res, "", nil
	}

	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	attrs.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
	// Note: At the time of writing, the value of the "name" field from the metadata
	// API (compute.Name) may differ from the actual hostname (e.g. if you update a
	// VM's hostname, the metadata API will still return the original name).
	attrs.InsertString(conventions.AttributeHostName, compute.Name)
	attrs.InsertString(conventions.AttributeCloudRegion, compute.Location)
	attrs.InsertString(conventions.AttributeHostID, compute.VMID)
	attrs.InsertString(conventions.AttributeCloudAccountID, compute.SubscriptionID)
	// Some components (e.g. signalfx exporter) require the compute.Name value (the
	// "name" returned from the metadata API), which historically has been written
	// to the host name attribute (AttributeHostName, inserted above), but another
	// detector may have precedence in writing to this attribute, in which case the
	// original name value from the metadata API will be lost by the time a
	// downstream component runs. To make sure this value is avaliable, we also save
	// the compute.Name in "azure.vm.name" here for use by downstream components.
	attrs.InsertString("azure.vm.name", compute.Name)
	attrs.InsertString("azure.vm.size", compute.VMSize)
	attrs.InsertString("azure.vm.scaleset.name", compute.VMScaleSetName)
	attrs.InsertString("azure.resourcegroup.name", compute.ResourceGroupName)

	return res, conventions.SchemaURL, nil
}
