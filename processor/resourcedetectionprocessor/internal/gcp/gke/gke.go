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

package gke

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
)

// TypeStr is type of detector.
const TypeStr = "gke"

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	log      *zap.Logger
	metadata gcp.Metadata
}

func NewDetector(params component.ProcessorCreateParams, _ internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{log: params.Logger, metadata: &gcp.MetadataImpl{}}, nil
}

// Detect detects associated resources when running in GKE environment.
func (gke *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()

	// Check if on GCP.
	if !gke.metadata.OnGCE() {
		return res, nil
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)

	// Check if running on k8s.
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return res, nil
	}

	attr.InsertString(conventions.AttributeCloudInfrastructureService, conventions.AttributeCloudProviderGCPGKE)

	if clusterName, err := gke.metadata.InstanceAttributeValue("cluster-name"); err != nil {
		gke.log.Warn("Unable to determine GKE cluster name", zap.Error(err))
	} else if clusterName != "" {
		attr.InsertString(conventions.AttributeK8sCluster, clusterName)
	}

	return res, nil
}
