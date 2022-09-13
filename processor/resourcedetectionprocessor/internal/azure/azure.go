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

	attrs.UpsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	attrs.UpsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAzureVM)
	attrs.UpsertString(conventions.AttributeHostName, compute.Name)
	attrs.UpsertString(conventions.AttributeCloudRegion, compute.Location)
	attrs.UpsertString(conventions.AttributeHostID, compute.VMID)
	attrs.UpsertString(conventions.AttributeCloudAccountID, compute.SubscriptionID)
	// Also save compute.Name in "azure.vm.name" as host.id (AttributeHostName) is
	// used by system detector.
	attrs.UpsertString("azure.vm.name", compute.Name)
	attrs.UpsertString("azure.vm.size", compute.VMSize)
	attrs.UpsertString("azure.vm.scaleset.name", compute.VMScaleSetName)
	attrs.UpsertString("azure.resourcegroup.name", compute.ResourceGroupName)

	return res, conventions.SchemaURL, nil
}
