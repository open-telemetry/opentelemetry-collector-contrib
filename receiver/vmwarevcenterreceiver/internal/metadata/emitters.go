// Copyright  The OpenTelemetry Authors
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

package metadata

func (mb *MetricsBuilder) EmitVM(vmName, vmID, powerstate string) {
	mb.EmitForResource(
		WithVcenterVMName(vmName),
		WithVcenterVMID(vmID),
		WithVcenterVMPowerState(powerstate),
	)
}

func (mb *MetricsBuilder) EmitHost(hostname string) {
	mb.EmitForResource(WithVcenterHostName(hostname))
}

func (mb *MetricsBuilder) EmitResourcePool(poolName string) {
	mb.EmitForResource(WithVcenterResourcePoolName(poolName))
}

func (mb *MetricsBuilder) EmitDatastore(datastoreName string) {
	mb.EmitForResource(WithVcenterDatastoreName(datastoreName))
}

func (mb *MetricsBuilder) EmitDatacenter(dcName string) {
	mb.EmitForResource(WithVcenterDatacenterName(dcName))
}

func (mb *MetricsBuilder) EmitCluster(clusterName string) {
	mb.EmitForResource(WithVcenterClusterName(clusterName))
}
