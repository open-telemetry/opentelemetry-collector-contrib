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

package azure

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is the detector type string
	TypeStr = "azure"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is an Azure metadata detector
type Detector struct {
	provider azureProvider
}

// NewDetector creates a new Azure metadata detector
func NewDetector(component.ProcessorCreateParams, internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{provider: newProvider()}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	attrs := res.Attributes()

	compute, err := d.provider.metadata(ctx)
	if err != nil {
		return res, fmt.Errorf("failed getting metadata: %w", err)
	}

	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	attrs.InsertString(conventions.AttributeHostName, compute.Name)
	attrs.InsertString(conventions.AttributeCloudRegion, compute.Location)
	attrs.InsertString(conventions.AttributeHostID, compute.VMID)
	attrs.InsertString(conventions.AttributeCloudAccount, compute.SubscriptionID)
	attrs.InsertString("azure.vm.size", compute.VMSize)
	attrs.InsertString("azure.resourcegroup.name", compute.ResourceGroupName)

	return res, nil
}
