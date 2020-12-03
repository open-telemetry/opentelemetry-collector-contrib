// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package system

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
	TypeStr = "system"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is a system metadata detector
type Detector struct {
	provider systemMetadata
}

// NewDetector creates a new system metadata detector
func NewDetector(component.ProcessorCreateParams) (internal.Detector, error) {
	return &Detector{provider: &systemMetadataImpl{}}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(_ context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	attrs := res.Attributes()

	osType, err := d.provider.OSType()
	if err != nil {
		return res, fmt.Errorf("failed getting OS type: %w", err)
	}

	fqdn, err := d.provider.FQDN()
	if err != nil {
		return res, fmt.Errorf("failed getting FQDN: %w", err)
	}

	attrs.InsertString(conventions.AttributeHostName, fqdn)
	attrs.InsertString(conventions.AttributeOSType, osType)

	return res, nil
}
