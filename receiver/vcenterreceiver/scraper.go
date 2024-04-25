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
}

func newVmwareVcenterScraper(
	logger *zap.Logger,
	config *Config,
	settings receiver.CreateSettings,
) *vcenterMetricScraper {
	client := newVcenterClient(config)
	return &vcenterMetricScraper{
		client:           client,
		config:           config,
		logger:           logger,
		mb:               metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		vmToComputeMap:   make(map[string]string),
		vmToResourcePool: make(map[string]*object.ResourcePool),
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

	v.collectResourcePools(ctx, now, errs)
	for _, c := range computes {
		v.collectHosts(ctx, now, c, errs)
		v.collectDatastores(ctx, now, c, errs)
		poweredOnVMs, poweredOffVMs := v.collectVMs(ctx, now, c, errs)
		if c.Reference().Type == "ClusterComputeResource" {
			v.collectCluster(ctx, now, c, poweredOnVMs, poweredOffVMs, errs)
		}
	}
}

func (v *vcenterMetricScraper) collectCluster(
	ctx context.Context,
	now pcommon.Timestamp,
	c *object.ComputeResource,
	poweredOnVMs, poweredOffVMs int64,
	errs *scrapererror.ScrapeErrors,
) {
	v.mb.RecordVcenterClusterVMCountDataPoint(now, poweredOnVMs, metadata.AttributeVMCountPowerStateOn)
	v.mb.RecordVcenterClusterVMCountDataPoint(now, poweredOffVMs, metadata.AttributeVMCountPowerStateOff)

	var moCluster mo.ClusterComputeResource
	err := c.Properties(ctx, c.Reference(), []string{"summary"}, &moCluster)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	s := moCluster.Summary.GetComputeResourceSummary()
	v.mb.RecordVcenterClusterCPULimitDataPoint(now, int64(s.TotalCpu))
	v.mb.RecordVcenterClusterCPUEffectiveDataPoint(now, int64(s.EffectiveCpu))
	v.mb.RecordVcenterClusterMemoryEffectiveDataPoint(now, s.EffectiveMemory)
	v.mb.RecordVcenterClusterMemoryLimitDataPoint(now, s.TotalMemory)
	v.mb.RecordVcenterClusterHostCountDataPoint(now, int64(s.NumHosts-s.NumEffectiveHosts), false)
	v.mb.RecordVcenterClusterHostCountDataPoint(now, int64(s.NumEffectiveHosts), true)
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterClusterName(c.Name())
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectDatastores(
	ctx context.Context,
	colTime pcommon.Timestamp,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	datastores, err := compute.Datastores(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, ds := range datastores {
		v.collectDatastore(ctx, colTime, ds, compute, errs)
	}
}

func (v *vcenterMetricScraper) collectDatastore(
	ctx context.Context,
	now pcommon.Timestamp,
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
	if compute.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(compute.Name())
	}
	rb.SetVcenterDatastoreName(moDS.Name)
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectHosts(
	ctx context.Context,
	colTime pcommon.Timestamp,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	hosts, err := compute.Hosts(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, h := range hosts {
		v.collectHost(ctx, colTime, h, compute, errs)
	}
}

func (v *vcenterMetricScraper) collectHost(
	ctx context.Context,
	now pcommon.Timestamp,
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
	rb.SetVcenterHostName(host.Name())
	if compute.Reference().Type == "ClusterComputeResource" {
		rb.SetVcenterClusterName(compute.Name())
	}
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectResourcePools(
	ctx context.Context,
	ts pcommon.Timestamp,
	errs *scrapererror.ScrapeErrors,
) {
	rps, err := v.client.ResourcePools(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
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

		v.recordResourcePool(ts, moRP)
		rb := v.mb.NewResourceBuilder()
		rb.SetVcenterResourcePoolName(rp.Name())
		rb.SetVcenterResourcePoolInventoryPath(rp.InventoryPath)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

func (v *vcenterMetricScraper) collectVMs(
	ctx context.Context,
	colTime pcommon.Timestamp,
	compute *object.ComputeResource,
	errs *scrapererror.ScrapeErrors,
) (poweredOnVMs int64, poweredOffVMs int64) {
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

		if string(vm.Runtime.PowerState) == "poweredOff" {
			poweredOffVMs++
		} else {
			poweredOnVMs++
		}

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
		rb := v.createVMResourceBuilder(vm, hwSum, compute, rp)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return poweredOnVMs, poweredOffVMs
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
