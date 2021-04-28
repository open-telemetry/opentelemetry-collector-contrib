// Copyright  OpenTelemetry Authors
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

package stores

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type K8sStore interface {
	// Decorate adds metrics and resource attributes to resource metrics
	// and update the kubernetesBlob with info about kubernetes (e.g. pod owner, labels, ...)
	Decorate(metric pdata.ResourceMetrics, kubernetesBlob map[string]interface{}) bool
	// Refresh does the refresh of k8s store to update releveant info about pod/container/service
	RefreshTick(context context.Context)
}
