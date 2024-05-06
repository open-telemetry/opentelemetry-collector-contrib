// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

const (
	emitPerfMetricsWithObjectsFeatureGateID = "receiver.vcenter.emitPerfMetricsWithObjects"
)

var _ = featuregate.GlobalRegistry().MustRegister(
	emitPerfMetricsWithObjectsFeatureGateID,
	featuregate.StageStable,
	featuregate.WithRegisterToVersion("v0.97.0"),
)

var _ receiver.Metrics = (*vcenterMetricScraper)(nil)

type vcenterMetricScraper struct {
	client *vcenterClient
	config *Config
	mb     *metadata.MetricsBuilder
	logger *zap.Logger

	// map of vm name => compute name
	vmToComputeMap   map[string]string
	vmToResourcePool map[string]*object.ResourcePool
	vmToVirtualApp   map[string]*object.VirtualApp
}

func newVmwareVcenterScraper(
	logger *zap.Logger,
	config *Config,
	settings receiver.CreateSettings,
) *vcenterMetricScraper {
	client := newVcenterClient(config)
	logger.Warn("[WARNING] `vcenter.cluster.name`: this attribute will be removed from the Datastore resource starting in release v0.101.0")
	return &vcenterMetricScraper{
		client:           client,
		config:           config,
		logger:           logger,
		mb:               metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		vmToComputeMap:   make(map[string]string),
		vmToResourcePool: make(map[string]*object.ResourcePool),
		vmToVirtualApp:   make(map[string]*object.VirtualApp),
	}
}

func (v *vcenterMetricScraper) Start(ctx context.Context, _ component.Host) error {
	connectErr := v.client.EnsureConnection(ctx)
	// don't fail to start if we cannot establish connection, just log an error
	if connectErr != nil {
		v.logger.Error(fmt.Sprintf("unable to establish a connection to the vSphere SDK %s", connectErr.Error()))
	}
	return nil
}

func (v *vcenterMetricScraper) Shutdown(ctx context.Context) error {
	return v.client.Disconnect(ctx)
}

func (v *vcenterMetricScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if v.client == nil {
		v.client = newVcenterClient(v.config)
	}

	// ensure connection before scraping
	if err := v.client.EnsureConnection(ctx); err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("unable to connect to vSphere SDK: %w", err)
	}
	err := v.collectDatacenters(ctx)

	// cleanup so any inventory moves are accounted for
	v.vmToComputeMap = make(map[string]string)
	v.vmToResourcePool = make(map[string]*object.ResourcePool)
	v.vmToVirtualApp = make(map[string]*object.VirtualApp)

	return v.mb.Emit(), err
}

func (v *vcenterMetricScraper) collectDatacenters(ctx context.Context) error {
	datacenters, err := v.client.Datacenters(ctx)
	if err != nil {
		return err
	}
	errs := &scrapererror.ScrapeErrors{}
	for _, dc := range datacenters {
		v.collectClusters(ctx, dc, errs)
	}
	return errs.Combine()
}

func (v *vcenterMetricScraper) collectClusters(ctx context.Context, datacenter *object.Datacenter, errs *scrapererror.ScrapeErrors) {
	computes, err := v.client.Computes(ctx, datacenter)
	if err != nil {
		errs.Add(err)
		return
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	dcName := datacenter.Name()
	v.collectVirtualApps(ctx, errs)
	v.collectResourcePools(ctx, now, dcName, computes, errs)
	for _, c := range computes {
		v.collectHosts(ctx, now, dcName, c, errs)
		v.collectDatastores(ctx, now, dcName, c, errs)
		poweredOnVMs, poweredOffVMs, suspendedVMs, templates := v.collectVMs(ctx, now, dcName, c, errs)
		if c.Reference().Type == "ClusterComputeResource" {
			v.collectCluster(ctx, now, dcName, c, poweredOnVMs, poweredOffVMs, suspendedVMs, templates, errs)
		}
	}
}

func (v *vcenterMetricScraper) collectCluster(
	ctx context.Context,
	now pcommon.Timestamp,
	dcName string,
	c *object.ComputeResource,
	poweredOnVMs, poweredOffVMs, suspendedVMs, templates int64,
	errs *scrapererror.ScrapeErrors,
) {
	v.mb.RecordVcenterClusterVMCountDataPoint(now, poweredOnVMs, metadata.AttributeVMCountPowerStateOn)
	v.mb.RecordVcenterClusterVMCountDataPoint(now, poweredOffVMs, metadata.AttributeVMCountPowerStateOff)
	v.mb.RecordVcenterClusterVMCountDataPoint(now, suspendedVMs, metadata.AttributeVMCountPowerStateSuspended)
	v.mb.RecordVcenterClusterVMTemplateCountDataPoint(now, templates)

	var moCluster mo.ClusterComputeResource
	err := c.Properties(ctx, c.Reference(), []string{"summary"}, &moCluster)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	s := moCluster.Summary.GetComputeResourceSummary()
	v.mb.RecordVcenterClusterCPULimitDataPoint(now, int64(s.TotalCpu))
	v.mb.RecordVcenterClusterCPUEffectiveDataPoint(now, int64(s.EffectiveCpu))
	v.mb.RecordVcenterClusterMemoryEffectiveDataPoint(now, s.EffectiveMemory<<20)
	v.mb.RecordVcenterClusterMemoryLimitDataPoint(now, s.TotalMemory)
	v.mb.RecordVcenterClusterHostCountDataPoint(now, int64(s.NumHosts-s.NumEffectiveHosts), false)
	v.mb.RecordVcenterClusterHostCountDataPoint(now, int64(s.NumEffectiveHosts), true)
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dcName)
	rb.SetVcenterClusterName(c.Name())
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectDatastores(
	ctx context.Context,
	colTime pcommon.Timestamp,
	dcName string,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	datastores, err := compute.Datastores(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, ds := range datastores {
		v.collectDatastore(ctx, colTime, dcName, ds, compute, errs)
	}
}

func (v *vcenterMetricScraper) collectDatastore(
	ctx context.Context,
	now pcommon.Timestamp,
	dcName string,
	ds *object.Datastore,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	var moDS mo.Datastore
	err := ds.Properties(ctx, ds.Reference(), []string{"summary", "name"}, &moDS)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	v.recordDatastoreProperties(now, moDS)
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dcName)
	if compute.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(compute.Name())
	}
	rb.SetVcenterDatastoreName(moDS.Name)
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectHosts(
	ctx context.Context,
	colTime pcommon.Timestamp,
	dcName string,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	hosts, err := compute.Hosts(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, h := range hosts {
		v.collectHost(ctx, colTime, dcName, h, compute, errs)
	}
}

func (v *vcenterMetricScraper) collectHost(
	ctx context.Context,
	now pcommon.Timestamp,
	dcName string,
	host *object.HostSystem,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {

	var hwSum mo.HostSystem
	err := host.Properties(ctx, host.Reference(),
		[]string{
			"name",
			"config",
			"summary.hardware",
			"summary.quickStats",
			"vm",
		}, &hwSum)

	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, vmRef := range hwSum.Vm {
		v.vmToComputeMap[vmRef.Value] = compute.Name()
	}

	v.recordHostSystemMemoryUsage(now, hwSum)
	v.recordHostPerformanceMetrics(ctx, hwSum, errs)
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterDatacenterName(dcName)
	rb.SetVcenterHostName(host.Name())
	if compute.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(compute.Name())
	}
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectResourcePools(
	ctx context.Context,
	ts pcommon.Timestamp,
	dcName string,
	computes []*object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	rps, err := v.client.ResourcePools(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	// Make it easier to find matching Clusters
	computesByRef := map[string]*object.ComputeResource{}
	for _, cr := range computes {
		computesByRef[cr.Reference().Value] = cr
	}

	for _, rp := range rps {
		var moRP mo.ResourcePool
		err = rp.Properties(ctx, rp.Reference(), []string{
			"summary",
			"summary.quickStats",
			"name",
			"vm",
		}, &moRP)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		computeRef, err := rp.Owner(ctx)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}
		for _, vmRef := range moRP.Vm {
			v.vmToComputeMap[vmRef.Value] = computeRef.Reference().Value
			v.vmToResourcePool[vmRef.Value] = rp
		}

		rb := v.mb.NewResourceBuilder()
		rpName := rp.Name()
		rb.SetVcenterResourcePoolName(rpName)
		rb.SetVcenterResourcePoolInventoryPath(rp.InventoryPath)
		rb.SetVcenterDatacenterName(dcName)

		compute := computesByRef[computeRef.Reference().Value]
		if compute == nil {
			errs.AddPartial(1, fmt.Errorf("no collected %s for Resource Pool [%s]'s owner ref: %s", computeRef.Reference().Type, rpName, computeRef.Reference().Value))
			continue
		}

		if computeRef.Reference().Type == "ClusterComputeResource" {
			rb.SetVcenterClusterName(compute.Name())
		}
		if computeRef.Reference().Type == "ComputeResource" {
			hosts, err := compute.Hosts(ctx)
			if err != nil || len(hosts) == 0 || hosts[0] == nil {
				errs.AddPartial(1, fmt.Errorf("no hosts found for Resource Pool [%s]'s owner ref: %s", rpName, computeRef.Reference().Value))
				continue
			}

			host := hosts[0]
			rb.SetVcenterHostName(host.Name())
		}

		v.recordResourcePool(ts, moRP)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (v *vcenterMetricScraper) collectVirtualApps(
	ctx context.Context,
	errs *scrapererror.ScrapeErrors,
) {
	vApps, err := v.client.VirtualApps(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for _, vApp := range vApps {
		var moVApp mo.VirtualApp
		err = vApp.Properties(ctx, vApp.Reference(), []string{
			"name",
			"vm",
		}, &moVApp)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		for _, vmRef := range moVApp.Vm {
			v.vmToVirtualApp[vmRef.Value] = vApp
		}
	}
}

func (v *vcenterMetricScraper) collectVMs(
	ctx context.Context,
	colTime pcommon.Timestamp,
	dcName string,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) (poweredOnVMs int64, poweredOffVMs int64, suspendedVMs int64, templates int64) {
	// Get all VMs with property data
	vms, err := v.client.VMs(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	spec := types.PerfQuerySpec{
		Format: string(types.PerfFormatNormal),
		// Just grabbing real time performance metrics of the current
		// supported metrics by this receiver. If more are added we may need
		// a system of making this user customizable or adapt to use a 5 minute interval per metric
		IntervalId: int32(20),
	}
	vmMoRefs := getVMMoRefs(vms)
	// Get all VM performance metrics
	vmPerfMetrics, err := v.client.perfMetricsQuery(ctx, spec, vmPerfMetricList, vmMoRefs)
	if err != nil {
		errs.AddPartial(1, err)
	}

	for _, vm := range vms {
		computeName, ok := v.vmToComputeMap[vm.Reference().Value]
		if !ok {
			continue
		}

		if computeName != compute.Reference().Value && computeName != compute.Name() {
			continue
		}

		if vm.Config.Template {
			templates++
		} else {
			switch vm.Runtime.PowerState {
			case "poweredOff":
				poweredOffVMs++
			case "poweredOn":
				poweredOnVMs++
			default:
				suspendedVMs++
			}
		}

		// TODO: Remove after v0.100.0 has been released
		// Ignore template resources/metrics for now if not explicitly enabled
		if vm.Config.Template &&
			!v.client.cfg.ResourceAttributes.VcenterVMTemplateID.Enabled &&
			!v.client.cfg.ResourceAttributes.VcenterVMTemplateName.Enabled {
			continue
		}

		// vApp may not exist for a VM
		vApp := v.vmToVirtualApp[vm.Reference().Value]

		// vms are optional without a resource pool
		rpRef := vm.ResourcePool
		var rp *object.ResourcePool
		if rpRef != nil {
			rp = object.NewResourcePool(v.client.vimDriver, *rpRef)
		}

		if rp != nil {
			rpCompute, rpErr := rp.Owner(ctx)
			if rpErr != nil {
				errs.AddPartial(1, err)
				return
			}
			// not part of this cluster
			if rpCompute.Reference().Value != compute.Reference().Value {
				continue
			}
			stored, ok := v.vmToResourcePool[vm.Reference().Value]
			if ok {
				rp = stored
			}
		}

		if vm.Config == nil {
			errs.AddPartial(1, fmt.Errorf("config empty for VM: %s", vm.Name))
			continue
		}

		// Get related VM host info
		hostRef := vm.Summary.Runtime.Host
		if hostRef == nil {
			errs.AddPartial(1, fmt.Errorf("no host for VM: %s", vm.Name))
			continue
		}
		vmHost := object.NewHostSystem(v.client.vimDriver, *hostRef)
		var hwSum mo.HostSystem
		err = vmHost.Properties(ctx, vmHost.Reference(),
			[]string{
				"name",
				"summary.hardware.cpuMhz",
				"vm",
			}, &hwSum)
		if err != nil {
			errs.AddPartial(1, err)
			return
		}

		perfMetrics := vmPerfMetrics.resultsByMoRef[vm.Reference().Value]
		v.buildVMMetrics(colTime, vm, hwSum, perfMetrics)
		rb := v.createVMResourceBuilder(dcName, vm, hwSum, compute, rp, vApp)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return poweredOnVMs, poweredOffVMs, suspendedVMs, templates
}

func (v *vcenterMetricScraper) buildVMMetrics(
	colTime pcommon.Timestamp,
	vm mo.VirtualMachine,
	hs mo.HostSystem,
	em *performance.EntityMetric,
) {
	v.recordVMUsages(colTime, vm, hs)
	if em != nil {
		v.recordVMPerformanceMetrics(em)
	}
}

func getVMMoRefs(
	vms []mo.VirtualMachine,
) []types.ManagedObjectReference {
	moRefs := []types.ManagedObjectReference{}
	for _, vm := range vms {
		moRefs = append(moRefs, vm.Reference())
	}

	return moRefs
}
