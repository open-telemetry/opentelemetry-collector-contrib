// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

// Detector is an Azure metadata detector
type Detector struct {
	provider azureProvider
	logger   *zap.Logger
	platform string
	envOK    func() bool
}

// NewDetector creates a new Azure metadata detector. This is effectively a
// default constructor called by the constructors in the `aks` and `azurevm`
// subdirectories.
func NewDetector(
	p component.ProcessorCreateParams,
	_ internal.DetectorConfig,
	platform string,
	envOK func() bool,
) (internal.Detector, error) {
	return &Detector{
		provider: newProvider(),
		logger:   p.Logger,
		platform: platform,
		envOK:    envOK,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()

	if !d.envOK() {
		return res, nil
	}

	metadata, err := d.provider.metadata(ctx)
	if err != nil {
		d.logger.Warn("attempt to get Azure metadata failed", zap.Error(err))
		// don't return an error
		return res, nil
	}

	attrs := res.Attributes()
	attrs.InsertString(conventions.AttributeCloudPlatform, d.platform)
	insertAttrs(metadata, attrs)

	return res, nil
}

func insertAttrs(d *computeMetadata, attrs pdata.AttributeMap) {
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	attrs.InsertString(conventions.AttributeHostName, d.Name)
	attrs.InsertString(conventions.AttributeCloudRegion, d.Location)
	attrs.InsertString(conventions.AttributeHostID, d.VMID)
	attrs.InsertString(conventions.AttributeCloudAccount, d.SubscriptionID)
	attrs.InsertString("azure.vm.size", d.VMSize)
	attrs.InsertString("azure.vm.scaleset.name", d.VMScaleSetName)
	attrs.InsertString("azure.resourcegroup.name", d.ResourceGroupName)
}
