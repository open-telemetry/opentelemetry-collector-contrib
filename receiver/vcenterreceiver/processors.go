// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"fmt"
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

type datacenterStats struct {
	ClusterStatusCounts map[types.ManagedEntityStatus]int64
	HostStats           map[string]map[types.ManagedEntityStatus]int64
	VMStats             map[string]map[types.ManagedEntityStatus]int64
	DatastoreCount      int64
	DiskCapacity        int64
	DiskFree            int64
	CPULimit            int64
	MemoryLimit         int64
}

// processDatacenterData creates all of the vCenter metrics from the stored scraped data under a single Datacenter
func (v *vcenterMetricScraper) processDatacenterData(dc *mo.Datacenter, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	now := pcommon.NewTimestampFromTime(time.Now())
	dcStats := &datacenterStats{
		ClusterStatusCounts: make(map[types.ManagedEntityStatus]int64),
		HostStats:           make(map[string]map[types.ManagedEntityStatus]int64),
		VMStats:             make(map[string]map[types.ManagedEntityStatus]int64),
	}
	v.processDatastores(now, dc, dcStats)
	v.processResourcePools(now, dc, errs)
	vmRefToComputeRef := v.processHosts(now, dc, dcStats, errs)
	vmGroupInfoByComputeRef := v.processVMs(now, dc, vmRefToComputeRef, dcStats, errs)
	v.processClusters(now, dc, vmGroupInfoByComputeRef, dcStats, errs)
	v.buildDatacenterMetrics(now, dc, dcStats)
}

// buildDatacenterMetrics builds a resource and metrics for a given scraped vCenter Datacenter
func (v *vcenterMetricScraper) buildDatacenterMetrics(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	dcStats *datacenterStats,
) {
	// Create Datacenter resource builder
	rb := v.createDatacenterResourceBuilder(dc)

	// Record & emit Datacenter metric data points
	v.recordDatacenterStats(ts, dcStats)

	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

// processDatastores creates the vCenter Datastore metrics and resources from the stored scraped data under a single Datacenter
func (v *vcenterMetricScraper) processDatastores(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	dcStats *datacenterStats,
) {
	for _, ds := range v.scrapeData.datastores {
		v.buildDatastoreMetrics(ts, dc, ds)
		dcStats.DatastoreCount++
		dcStats.DiskCapacity += ds.Summary.Capacity
		dcStats.DiskFree += ds.Summary.FreeSpace
	}
}

// buildDatastoreMetrics builds a resource and metrics for a given scraped vCenter Datastore
func (v *vcenterMetricScraper) buildDatastoreMetrics(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	ds *mo.Datastore,
) {
	// Create Datastore resource builder
	rb := v.createDatastoreResourceBuilder(dc, ds)

	// Record & emit Datastore metric data points
	v.recordDatastoreStats(ts, ds)
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

// processResourcePools creates the vCenter Resource Pool metrics and resources from the stored scraped data under a single Datacenter
func (v *vcenterMetricScraper) processResourcePools(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	errs *scrapererror.ScrapeErrors,
) {
	for _, rp := range v.scrapeData.rPoolsByRef {
		// Don't make metrics for vApps
		if rp.Reference().Type == "VirtualApp" {
			continue
		}

		if err := v.buildResourcePoolMetrics(ts, dc, rp); err != nil {
			errs.AddPartial(1, err)
		}
	}
}

// buildResourcePoolMetrics builds a resource and metrics for a given scraped vCenter Resource Pool
func (v *vcenterMetricScraper) buildResourcePoolMetrics(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	rp *mo.ResourcePool,
) error {
	// Get related ResourcePool Compute info
	crRef := rp.Owner
	cr := v.scrapeData.computesByRef[crRef.Value]
	if cr == nil {
		return fmt.Errorf("no collected ComputeResource found for ResourcePool [%s]'s parent ref: %s", rp.Name, crRef.Value)
	}

	// Create ResourcePool resource builder
	rb, err := v.createResourcePoolResourceBuilder(dc, cr, rp)
	if err != nil {
		return err
	}

	// Record & emit Resource Pool metric data points
	v.recordResourcePoolStats(ts, rp)
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return nil
}

// processHosts creates the vCenter HostS metrics and resources from the stored scraped data under a single Datacenter
//
// returns a map containing the ComputeResource info for each VM
func (v *vcenterMetricScraper) processHosts(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	dcStats *datacenterStats,
	errs *scrapererror.ScrapeErrors,
) map[string]*types.ManagedObjectReference {
	vmRefToComputeRef := map[string]*types.ManagedObjectReference{}

	for _, hs := range v.scrapeData.hostsByRef {
		powerState := string(hs.Runtime.PowerState)
		ensureInnerMapInitialized(dcStats.HostStats, powerState)
		dcStats.HostStats[powerState][hs.Summary.OverallStatus]++

		dcStats.CPULimit += int64(hs.Summary.Hardware.CpuMhz * int32(hs.Summary.Hardware.NumCpuCores))
		dcStats.MemoryLimit += hs.Summary.Hardware.MemorySize

		hsVMRefToComputeRef, err := v.buildHostMetrics(ts, dc, hs)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		// Populate master VM to CR relationship map from
		// single Host based version of it
		for vmRef, csRef := range hsVMRefToComputeRef {
			vmRefToComputeRef[vmRef] = csRef
		}
	}

	return vmRefToComputeRef
}

// buildHostMetrics builds a resource and metrics for a given scraped Host
//
// returns a map containing the ComputeResource info for each VM running on the Host
func (v *vcenterMetricScraper) buildHostMetrics(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	hs *mo.HostSystem,
) (vmRefToComputeRef map[string]*types.ManagedObjectReference, err error) {
	vmRefToComputeRef = map[string]*types.ManagedObjectReference{}
	// Get related Host ComputeResource info
	crRef := hs.Parent
	if crRef == nil {
		return vmRefToComputeRef, fmt.Errorf("no parent found for Host: %s", hs.Name)
	}
	cr := v.scrapeData.computesByRef[crRef.Value]
	if cr == nil {
		return vmRefToComputeRef, fmt.Errorf("no collected ComputeResource found for Host [%s]'s parent ref: %s", hs.Name, crRef.Value)
	}

	// Store VM to ComputeResource relationship for all child VMs
	for _, vmRef := range hs.Vm {
		vmRefToComputeRef[vmRef.Value] = crRef
	}

	// Create Host resource builder
	rb := v.createHostResourceBuilder(dc, cr, hs)

	// Record & emit Host metric data points
	v.recordHostSystemStats(ts, hs)
	hostPerfMetrics := v.scrapeData.hostPerfMetricsByRef[hs.Reference().Value]
	if hostPerfMetrics != nil {
		v.recordHostPerformanceMetrics(hostPerfMetrics)
	}

	if hs.Config == nil || hs.Config.VsanHostConfig == nil || hs.Config.VsanHostConfig.ClusterInfo == nil {
		v.logger.Info("couldn't determine UUID necessary for vSAN metrics for host " + hs.Name)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		return vmRefToComputeRef, nil
	}
	vSANMetrics := v.scrapeData.hostVSANMetricsByUUID[hs.Config.VsanHostConfig.ClusterInfo.NodeUuid]
	if vSANMetrics != nil {
		v.recordHostVSANMetrics(vSANMetrics)
	}
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return vmRefToComputeRef, nil
}

// processVMs creates the vCenter VM metrics and resources from the stored scraped data under a single Datacenter
//
// returns a map of all VM State counts for each ComputeResource
func (v *vcenterMetricScraper) processVMs(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	vmRefToComputeRef map[string]*types.ManagedObjectReference,
	dcStats *datacenterStats,
	errs *scrapererror.ScrapeErrors,
) map[string]*vmGroupInfo {
	vmGroupInfoByComputeRef := map[string]*vmGroupInfo{}
	for _, vm := range v.scrapeData.vmsByRef {
		crRef, singleVMGroupInfo, err := v.buildVMMetrics(ts, dc, vm, vmRefToComputeRef)
		if err != nil {
			errs.AddPartial(1, err)
		}

		// Update master ComputeResource VM power state counts with VM power info
		if crRef != nil && singleVMGroupInfo != nil {
			crVMGroupInfo := vmGroupInfoByComputeRef[crRef.Value]
			if crVMGroupInfo == nil {
				crVMGroupInfo = &vmGroupInfo{poweredOff: 0, poweredOn: 0, suspended: 0, templates: 0}
				vmGroupInfoByComputeRef[crRef.Value] = crVMGroupInfo
			}
			overallStatus := vm.Summary.OverallStatus
			if singleVMGroupInfo.poweredOn > 0 {
				crVMGroupInfo.poweredOn++
				ensureInnerMapInitialized(dcStats.VMStats, "poweredOn")
				dcStats.VMStats["poweredOn"][overallStatus]++
			}
			if singleVMGroupInfo.poweredOff > 0 {
				crVMGroupInfo.poweredOff++
				ensureInnerMapInitialized(dcStats.VMStats, "poweredOff")
				dcStats.VMStats["poweredOff"][overallStatus]++
			}
			if singleVMGroupInfo.suspended > 0 {
				crVMGroupInfo.suspended++
				ensureInnerMapInitialized(dcStats.VMStats, "suspended")
				dcStats.VMStats["suspended"][overallStatus]++
			}
			if singleVMGroupInfo.templates > 0 {
				crVMGroupInfo.templates++
			}
		}
	}

	return vmGroupInfoByComputeRef
}

// ensureInnerMapInitialized is a helper function that ensures maps are initialized in order to freely aggregate VM and Host stats
func ensureInnerMapInitialized(stats map[string]map[types.ManagedEntityStatus]int64, key string) {
	if stats[key] == nil {
		stats[key] = make(map[types.ManagedEntityStatus]int64)
	}
}

// buildVMMetrics builds a resource and metrics for a given scraped VM
//
// returns the ComputeResource and power state info associated with this VM
func (v *vcenterMetricScraper) buildVMMetrics(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	vm *mo.VirtualMachine,
	vmRefToComputeRef map[string]*types.ManagedObjectReference,
) (crRef *types.ManagedObjectReference, groupInfo *vmGroupInfo, err error) {
	// Get related VM compute info
	crRef = vmRefToComputeRef[vm.Reference().Value]
	if crRef == nil {
		return crRef, groupInfo, fmt.Errorf("no ComputeResource ref found for VM: %s", vm.Name)
	}
	cr := v.scrapeData.computesByRef[crRef.Value]
	if cr == nil {
		return crRef, groupInfo, fmt.Errorf("no collected ComputeResource for VM [%s]'s ComputeResource ref: %s", vm.Name, crRef)
	}

	// Get related VM host info
	hsRef := vm.Summary.Runtime.Host
	if hsRef == nil {
		return crRef, groupInfo, fmt.Errorf("no Host ref for VM: %s", vm.Name)
	}
	hs := v.scrapeData.hostsByRef[hsRef.Value]
	if hs == nil {
		return crRef, groupInfo, fmt.Errorf("no collected Host for VM [%s]'s Host ref: %s", vm.Name, hsRef.Value)
	}

	// VMs may not have a ResourcePool reported (templates)
	// But grab it if available
	rpRef := vm.ResourcePool
	var rp *mo.ResourcePool
	if rpRef != nil {
		rp = v.scrapeData.rPoolsByRef[rpRef.Value]
	}

	groupInfo = &vmGroupInfo{poweredOff: 0, poweredOn: 0, suspended: 0, templates: 0}
	if vm.Config.Template {
		groupInfo.templates++
	} else {
		switch vm.Runtime.PowerState {
		case "poweredOff":
			groupInfo.poweredOff++
		case "poweredOn":
			groupInfo.poweredOn++
		default:
			groupInfo.suspended++
		}
	}

	// Create VM resource builder
	rb, err := v.createVMResourceBuilder(dc, cr, hs, rp, vm)
	if err != nil {
		return crRef, groupInfo, err
	}

	// Record VM metric data points
	v.recordVMStats(ts, vm, hs)
	perfMetrics := v.scrapeData.vmPerfMetricsByRef[vm.Reference().Value]
	if perfMetrics != nil {
		v.recordVMPerformanceMetrics(perfMetrics)
	}

	vSANMetrics := v.scrapeData.vmVSANMetricsByUUID[vm.Config.InstanceUuid]
	if vSANMetrics != nil {
		v.recordVMVSANMetrics(vSANMetrics)
	}
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return crRef, groupInfo, err
}

// processClusters creates the vCenter Cluster metrics and resources from the stored scraped data under a single Datacenter
func (v *vcenterMetricScraper) processClusters(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	vmStatesByComputeRef map[string]*vmGroupInfo,
	dcStats *datacenterStats,
	errs *scrapererror.ScrapeErrors,
) {
	for crRef, cr := range v.scrapeData.computesByRef {
		// Don't make metrics for anything that's not a Cluster (otherwise it should be the same as a HostSystem)
		if cr.Reference().Type != "ClusterComputeResource" {
			continue
		}

		summary := cr.Summary.GetComputeResourceSummary()
		dcStats.ClusterStatusCounts[summary.OverallStatus]++
		vmGroupInfo := vmStatesByComputeRef[crRef]

		if err := v.buildClusterMetrics(ts, dc, cr, vmGroupInfo); err != nil {
			errs.AddPartial(1, err)
		}
	}
}

// buildClusterMetrics builds a resource and metrics for a given scraped Cluster
func (v *vcenterMetricScraper) buildClusterMetrics(
	ts pcommon.Timestamp,
	dc *mo.Datacenter,
	cr *mo.ComputeResource,
	vmGroupInfo *vmGroupInfo,
) (err error) {
	// Create Cluster resource builder
	rb := v.createClusterResourceBuilder(dc, cr)

	if vmGroupInfo == nil {
		err = fmt.Errorf("no VM power counts found for Cluster: %s", cr.Name)
	}
	// Record and emit Cluster metric data points
	v.recordClusterStats(ts, cr, vmGroupInfo)
	vSANConfig := cr.ConfigurationEx.(*types.ClusterConfigInfoEx).VsanConfigInfo
	if vSANConfig == nil || vSANConfig.Enabled == nil || !*vSANConfig.Enabled || vSANConfig.DefaultConfig == nil {
		v.logger.Info("couldn't determine UUID necessary for vSAN metrics for cluster " + cr.Name)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
		return err
	}

	vSANMetrics := v.scrapeData.clusterVSANMetricsByUUID[vSANConfig.DefaultConfig.Uuid]
	if vSANMetrics != nil {
		v.recordClusterVSANMetrics(vSANMetrics)
	}

	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return err
}
