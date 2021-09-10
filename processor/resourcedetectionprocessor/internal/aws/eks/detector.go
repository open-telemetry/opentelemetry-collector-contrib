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

package eks

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "eks"

	// Environment variable that is set when running on Kubernetes.
	kubernetesServiceHostEnvVar = "KUBERNETES_SERVICE_HOST"
)

var _ internal.Detector = (*Detector)(nil)

// Detector for EKS
type Detector struct{}

// NewDetector returns a resource detector that will detect AWS EKS resources.
func NewDetector(_ component.ProcessorCreateSettings, _ internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{}, nil
}

// Detect returns a Resource describing the Amazon EKS environment being run in.
func (detector *Detector) Detect(ctx context.Context) (resource pdata.Resource, schemaURL string, err error) {
	res := pdata.NewResource()

	// Check if running on k8s.
	if os.Getenv(kubernetesServiceHostEnvVar) == "" {
		return res, "", nil
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEKS)

	return res, conventions.SchemaURL, nil
}
