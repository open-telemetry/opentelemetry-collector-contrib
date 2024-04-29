// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func (v *vcenterMetricScraper) createVMResourceBuilder(
	dcName string,
	vm mo.VirtualMachine,
	hs mo.HostSystem,
	compute *object.ComputeResource,
	rp *object.ResourcePool,
	vApp *object.VirtualApp,
) *metadata.ResourceBuilder {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dcName)
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
	if vApp != nil && vApp.Name() != "" {
		rb.SetVcenterVirtualAppName(vApp.Name())
		rb.SetVcenterVirtualAppInventoryPath(vApp.InventoryPath)
	}
	return rb
}
