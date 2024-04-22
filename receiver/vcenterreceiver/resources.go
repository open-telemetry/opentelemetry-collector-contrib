// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func (v *vcenterMetricScraper) createVMResourceBuilder(
	vm mo.VirtualMachine,
	hs mo.HostSystem,
	compute *object.ComputeResource,
	rp *object.ResourcePool,
) *metadata.ResourceBuilder {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterVMName(vm.Summary.Config.Name)
	rb.SetVcenterVMID(vm.Config.InstanceUuid)
	if compute.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(compute.Name())
	}
	rb.SetVcenterHostName(hs.Name)
	if rp != nil && rp.Name() != "" {
		rb.SetVcenterResourcePoolName(rp.Name())
		rb.SetVcenterResourcePoolInventoryPath(rp.InventoryPath)
	}
	return rb
}
