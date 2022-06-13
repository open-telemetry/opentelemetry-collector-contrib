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

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/internal/azure"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/provider"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
)

var _ provider.HostnameProvider = (*Provider)(nil)

type Provider struct {
	detector azure.Provider
}

// Hostname returns the Azure cloud integration hostname.
func (p *Provider) Hostname(ctx context.Context) (string, error) {
	metadata, err := p.detector.Metadata(ctx)
	if err != nil {
		return "", err
	}

	return metadata.VMID, nil
}

// NewProvider creates a new Azure hostname provider.
func NewProvider() *Provider {
	return &Provider{detector: azure.NewProvider()}
}
