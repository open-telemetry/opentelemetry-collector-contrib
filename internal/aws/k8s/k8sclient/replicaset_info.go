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

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type ReplicaSetInfo struct {
	Name      string
	Namespace string
	Owners    []*ReplicaSetOwner
	Spec      *ReplicaSetSpec
	Status    *ReplicaSetStatus
}

type ReplicaSetOwner struct {
	kind string
	name string
}

type ReplicaSetSpec struct {
	Replicas uint32
}

type ReplicaSetStatus struct {
	Replicas          uint32
	AvailableReplicas uint32
	ReadyReplicas     uint32
}
