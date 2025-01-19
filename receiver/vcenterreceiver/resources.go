// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"fmt"

	"github.com/vmware/govmomi/vim25/mo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

// createDatastoreResourceBuilder returns a ResourceBuilder with
// attributes set for a vSphere Datastore
func (v *vcenterMetricScraper) createDatastoreResourceBuilder(
	dc *mo.Datacenter,
	ds *mo.Datastore,
) *metadata.ResourceBuilder {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dc.Name)
	rb.SetVcenterDatastoreName(ds.Name)

	return rb
}

// createDatacenterResourceBuilder returns a ResourceBuilder with
// attributes set for a vSphere datacenter
func (v *vcenterMetricScraper) createDatacenterResourceBuilder(
	dc *mo.Datacenter,
) *metadata.ResourceBuilder {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dc.Name)
	return rb
}

// createClusterResourceBuilder returns a ResourceBuilder with
// attributes set for a vSphere Cluster
func (v *vcenterMetricScraper) createClusterResourceBuilder(
	dc *mo.Datacenter,
	cr *mo.ComputeResource,
) *metadata.ResourceBuilder {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dc.Name)
	rb.SetVcenterClusterName(cr.Name)

	return rb
}

// createResourcePoolResourceBuilder returns a ResourceBuilder with
// attributes set for a vSphere Resource Pool
func (v *vcenterMetricScraper) createResourcePoolResourceBuilder(
	dc *mo.Datacenter,
	cr *mo.ComputeResource,
	rp *mo.ResourcePool,
) (*metadata.ResourceBuilder, error) {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dc.Name)
	if cr.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(cr.Name)
	}
	if cr.Reference().Type == "ComputeResource" {
		if len(cr.Host) == 0 {
			return nil, fmt.Errorf("no Hosts found for Resource Pool [%s]'s owner ref: %s", rp.Name, cr.Reference().Value)
		}

		hsRef := cr.Host[0]
		hs := v.scrapeData.hostsByRef[hsRef.Value]
		if hs == nil {
			return nil, fmt.Errorf("no Hosts found for Resource Pool [%s]'s owner ref: %s", rp.Name, cr.Reference().Value)
		}

		rb.SetVcenterHostName(hs.Name)
	}
	rb.SetVcenterResourcePoolName(rp.Name)
	iPath := v.scrapeData.rPoolIPathsByRef[rp.Reference().Value]
	if iPath == nil {
		return nil, fmt.Errorf("no inventory path found for collected ResourcePool: %s", rp.Name)
	}
	rb.SetVcenterResourcePoolInventoryPath(*iPath)

	return rb, nil
}

// createHostResourceBuilder returns a ResourceBuilder with
// attributes set for a vSphere Host
func (v *vcenterMetricScraper) createHostResourceBuilder(
	dc *mo.Datacenter,
	cr *mo.ComputeResource,
	hs *mo.HostSystem,
) *metadata.ResourceBuilder {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dc.Name)
	if cr.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(cr.Name)
	}
	rb.SetVcenterHostName(hs.Name)

	return rb
}

// createVMResourceBuilder returns a ResourceBuilder with
// attributes set for a vSphere Virtual Machine
func (v *vcenterMetricScraper) createVMResourceBuilder(
	dc *mo.Datacenter,
	cr *mo.ComputeResource,
	hs *mo.HostSystem,
	rp *mo.ResourcePool,
	vm *mo.VirtualMachine,
) (*metadata.ResourceBuilder, error) {
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dc.Name)
	if cr.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(cr.Name)
	}
	rb.SetVcenterHostName(hs.Name)

	if vm.Config.Template {
		rb.SetVcenterVMTemplateName(vm.Name)
		rb.SetVcenterVMTemplateID(vm.Config.InstanceUuid)

		return rb, nil
	}

	rb.SetVcenterVMName(vm.Name)
	rb.SetVcenterVMID(vm.Config.InstanceUuid)

	if rp == nil {
		return nil, fmt.Errorf("no Resource Pool found for VM: %s", vm.Name)
	}

	if rp.Reference().Type == "VirtualApp" {
		rb.SetVcenterVirtualAppName(rp.Name)
		iPath := v.scrapeData.vAppIPathsByRef[rp.Reference().Value]
		if iPath == nil {
			return nil, fmt.Errorf("no inventory path found for VM [%s]'s collected vApp: %s", vm.Name, rp.Name)
		}
		rb.SetVcenterVirtualAppInventoryPath(*iPath)
	} else {
		rb.SetVcenterResourcePoolName(rp.Name)
		iPath := v.scrapeData.rPoolIPathsByRef[rp.Reference().Value]
		if iPath == nil {
			return nil, fmt.Errorf("no inventory path found for VM [%s]'s collected ResourcePool: %s", vm.Name, rp.Name)
		}
		rb.SetVcenterResourcePoolInventoryPath(*iPath)
	}

	return rb, nil
}
