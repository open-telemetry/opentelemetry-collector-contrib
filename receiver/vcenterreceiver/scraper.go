// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"
import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

var _ receiver.Metrics = (*vcenterMetricScraper)(nil)

type vmGroupInfo struct {
	poweredOn  int64
	poweredOff int64
	suspended  int64
	templates  int64
}

type vcenterScrapeData struct {
	datacenters              []*mo.Datacenter
	datastores               []*mo.Datastore
	clusterRefs              []*types.ManagedObjectReference
	rPoolIPathsByRef         map[string]*string
	vAppIPathsByRef          map[string]*string
	rPoolsByRef              map[string]*mo.ResourcePool
	computesByRef            map[string]*mo.ComputeResource
	hostsByRef               map[string]*mo.HostSystem
	hostPerfMetricsByRef     map[string]*performance.EntityMetric
	vmsByRef                 map[string]*mo.VirtualMachine
	vmPerfMetricsByRef       map[string]*performance.EntityMetric
	vmVSANMetricsByUUID      map[string]*vSANMetricResults
	hostVSANMetricsByUUID    map[string]*vSANMetricResults
	clusterVSANMetricsByUUID map[string]*vSANMetricResults
}

type vcenterMetricScraper struct {
	client     *vcenterClient
	config     *Config
	mb         *metadata.MetricsBuilder
	logger     *zap.Logger
	scrapeData *vcenterScrapeData
}

func newVmwareVcenterScraper(
	logger *zap.Logger,
	config *Config,
	settings receiver.Settings,
) *vcenterMetricScraper {
	client := newVcenterClient(logger, config)
	scrapeData := newVcenterScrapeData()

	return &vcenterMetricScraper{
		client:     client,
		config:     config,
		logger:     logger,
		mb:         metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		scrapeData: scrapeData,
	}
}

func newVcenterScrapeData() *vcenterScrapeData {
	return &vcenterScrapeData{
		datacenters:              make([]*mo.Datacenter, 0),
		datastores:               make([]*mo.Datastore, 0),
		clusterRefs:              make([]*types.ManagedObjectReference, 0),
		rPoolIPathsByRef:         make(map[string]*string),
		vAppIPathsByRef:          make(map[string]*string),
		computesByRef:            make(map[string]*mo.ComputeResource),
		hostsByRef:               make(map[string]*mo.HostSystem),
		hostPerfMetricsByRef:     make(map[string]*performance.EntityMetric),
		rPoolsByRef:              make(map[string]*mo.ResourcePool),
		vmsByRef:                 make(map[string]*mo.VirtualMachine),
		vmPerfMetricsByRef:       make(map[string]*performance.EntityMetric),
		vmVSANMetricsByUUID:      make(map[string]*vSANMetricResults),
		hostVSANMetricsByUUID:    make(map[string]*vSANMetricResults),
		clusterVSANMetricsByUUID: make(map[string]*vSANMetricResults),
	}
}

func (v *vcenterMetricScraper) Start(ctx context.Context, _ component.Host) error {
	connectErr := v.client.EnsureConnection(ctx)
	// don't fail to start if we cannot establish connection, just log an error
	if connectErr != nil {
		v.logger.Error("unable to establish a connection to the vSphere SDK " + connectErr.Error())
	}
	return nil
}

func (v *vcenterMetricScraper) Shutdown(ctx context.Context) error {
	return v.client.Disconnect(ctx)
}

func (v *vcenterMetricScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if v.client == nil {
		v.client = newVcenterClient(v.logger, v.config)
	}
	// ensure connection before scraping
	if err := v.client.EnsureConnection(ctx); err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("unable to connect to vSphere SDK: %w", err)
	}

	errs := &scrapererror.ScrapeErrors{}
	err := v.scrapeAndProcessAllMetrics(ctx, errs)

	return v.mb.Emit(), err
}

// scrapeAndProcessAllMetrics collects & converts all relevant resources managed by vCenter to OTEL resources & metrics
func (v *vcenterMetricScraper) scrapeAndProcessAllMetrics(ctx context.Context, errs *scrapererror.ScrapeErrors) error {
	v.scrapeData = newVcenterScrapeData()
	dcObjects := v.scrapeDatacenterInventoryListObjects(ctx, errs)
	v.scrapeResourcePoolInventoryListObjects(ctx, dcObjects, errs)
	v.scrapeVAppInventoryListObjects(ctx, dcObjects, errs)
	v.scrapeDatacenters(ctx, errs)

	for _, dc := range v.scrapeData.datacenters {
		v.scrapeDatastores(ctx, dc, errs)
		v.scrapeComputes(ctx, dc, errs)
		v.scrapeHosts(ctx, dc, errs)
		v.scrapeResourcePools(ctx, dc, errs)
		v.scrapeVirtualMachines(ctx, dc, errs)

		// Build metrics now that all vCenter data has been scraped for a single datacenter
		v.processDatacenterData(dc, errs)
	}
	// Clear scrape data
	v.scrapeData = nil

	return errs.Combine()
}

// scrapeDatacenterInventoryListObjects scrapes and stores all Datacenter objects with their InventoryLists
func (v *vcenterMetricScraper) scrapeDatacenterInventoryListObjects(
	ctx context.Context,
	errs *scrapererror.ScrapeErrors,
) []*object.Datacenter {
	// Get Datacenters with InventoryLists and store for later retrieval
	dcs, err := v.client.DatacenterInventoryListObjects(ctx)
	if err != nil {
		errs.AddPartial(1, err)
	}
	return dcs
}

// scrapeResourcePoolInventoryListObjects scrapes and stores all ResourcePool objects with their InventoryLists
func (v *vcenterMetricScraper) scrapeResourcePoolInventoryListObjects(
	ctx context.Context,
	dcs []*object.Datacenter,
	errs *scrapererror.ScrapeErrors,
) {
	// Init for current collection
	v.scrapeData.rPoolIPathsByRef = make(map[string]*string)

	// Get ResourcePools with InventoryLists and store for later retrieval
	rPools, err := v.client.ResourcePoolInventoryListObjects(ctx, dcs)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for i := range rPools {
		v.scrapeData.rPoolIPathsByRef[rPools[i].Reference().Value] = &rPools[i].InventoryPath
	}
}

// scrapeVAppInventoryListObjects scrapes and stores all vApp objects with their InventoryLists
func (v *vcenterMetricScraper) scrapeVAppInventoryListObjects(
	ctx context.Context,
	dcs []*object.Datacenter,
	errs *scrapererror.ScrapeErrors,
) {
	// Init for current collection
	v.scrapeData.vAppIPathsByRef = make(map[string]*string)

	// Get vApps with InventoryLists and store for later retrieval
	vApps, err := v.client.VAppInventoryListObjects(ctx, dcs)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for i := range vApps {
		v.scrapeData.vAppIPathsByRef[vApps[i].Reference().Value] = &vApps[i].InventoryPath
	}
}

// scrapeDatacenters scrapes and stores all relevant property data for all Datacenters
func (v *vcenterMetricScraper) scrapeDatacenters(ctx context.Context, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	v.scrapeData.datacenters = make([]*mo.Datacenter, 0)

	// Get Datacenters w/properties and store for later retrieval
	datacenters, err := v.client.Datacenters(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for i := range datacenters {
		v.scrapeData.datacenters = append(v.scrapeData.datacenters, &datacenters[i])
	}
}

// scrapeDatastores scrapes and stores all relevant property data for a Datacenter's Datastores
func (v *vcenterMetricScraper) scrapeDatastores(ctx context.Context, dc *mo.Datacenter, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	v.scrapeData.datastores = make([]*mo.Datastore, 0)

	// Get Datastores w/properties and store for later retrieval
	datastores, err := v.client.Datastores(ctx, dc.Reference())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for i := range datastores {
		v.scrapeData.datastores = append(v.scrapeData.datastores, &datastores[i])
	}
}

// scrapeComputes scrapes and stores all relevant property data for a Datacenter's ComputeResources/ClusterComputeResources
func (v *vcenterMetricScraper) scrapeComputes(ctx context.Context, dc *mo.Datacenter, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	v.scrapeData.computesByRef = make(map[string]*mo.ComputeResource)
	v.scrapeData.clusterRefs = []*types.ManagedObjectReference{}

	// Get ComputeResources/ClusterComputeResources w/properties and store for later retrieval
	computes, err := v.client.ComputeResources(ctx, dc.Reference())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for i := range computes {
		computeRef := computes[i].Reference()
		v.scrapeData.computesByRef[computeRef.Value] = &computes[i]
		if computeRef.Type == "ClusterComputeResource" {
			v.scrapeData.clusterRefs = append(v.scrapeData.clusterRefs, &computeRef)
		}
	}

	// Get all Cluster vSAN metrics and store for later retrieval
	vSANMetrics, err := v.client.VSANClusters(ctx, v.scrapeData.clusterRefs)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to retrieve vSAN metrics for Clusters: %w", err))
		return
	}
	v.scrapeData.clusterVSANMetricsByUUID = vSANMetrics.MetricResultsByUUID
}

// scrapeHosts scrapes and stores all relevant metric/property data for a Datacenter's HostSystems
func (v *vcenterMetricScraper) scrapeHosts(ctx context.Context, dc *mo.Datacenter, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	v.scrapeData.hostsByRef = make(map[string]*mo.HostSystem)
	// Get HostSystems w/properties and store for later retrieval
	hosts, err := v.client.HostSystems(ctx, dc.Reference())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	hsRefs := []types.ManagedObjectReference{}
	for i := range hosts {
		hsRefs = append(hsRefs, hosts[i].Reference())
		v.scrapeData.hostsByRef[hosts[i].Reference().Value] = &hosts[i]
	}

	spec := types.PerfQuerySpec{
		MaxSample: 1,
		Format:    string(types.PerfFormatNormal),
		// Just grabbing real time performance metrics of the current
		// supported metrics by this receiver. If more are added we may need
		// a system of making this user customizable or adapt to use a 5 minute interval per metric
		IntervalId: int32(20),
	}
	// Get all HostSystem performance metrics and store for later retrieval
	results, err := v.client.PerfMetricsQuery(ctx, spec, hostPerfMetricList, hsRefs)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to retrieve perf metrics for HostSystems: %w", err))
	} else {
		v.scrapeData.hostPerfMetricsByRef = results.resultsByRef
	}

	vSANMetrics, err := v.client.VSANHosts(ctx, v.scrapeData.clusterRefs)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to retrieve vSAN metrics for Hosts: %w", err))
		return
	}
	v.scrapeData.hostVSANMetricsByUUID = vSANMetrics.MetricResultsByUUID
}

// scrapeResourcePools scrapes and stores all relevant property data for a Datacenter's ResourcePools/vApps
func (v *vcenterMetricScraper) scrapeResourcePools(ctx context.Context, dc *mo.Datacenter, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	v.scrapeData.rPoolsByRef = make(map[string]*mo.ResourcePool)

	// Get ResourcePools/vApps w/properties and store for later retrieval
	rPools, err := v.client.ResourcePools(ctx, dc.Reference())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for i := range rPools {
		v.scrapeData.rPoolsByRef[rPools[i].Reference().Value] = &rPools[i]
	}
}

// scrapeVirtualMachines scrapes and stores all relevant metric/property data for a Datacenter's VirtualMachines
func (v *vcenterMetricScraper) scrapeVirtualMachines(ctx context.Context, dc *mo.Datacenter, errs *scrapererror.ScrapeErrors) {
	// Init for current collection
	v.scrapeData.vmsByRef = make(map[string]*mo.VirtualMachine)

	// Get VirtualMachines w/properties and store for later retrieval
	vms, err := v.client.VMs(ctx, dc.Reference())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	vmRefs := []types.ManagedObjectReference{}
	for i := range vms {
		vmRefs = append(vmRefs, vms[i].Reference())
		v.scrapeData.vmsByRef[vms[i].Reference().Value] = &vms[i]
	}

	spec := types.PerfQuerySpec{
		Format: string(types.PerfFormatNormal),
		// Just grabbing real time performance metrics of the current
		// supported metrics by this receiver. If more are added we may need
		// a system of making this user customizable or adapt to use a 5 minute interval per metric
		IntervalId: int32(20),
	}
	// Get all VirtualMachine performance metrics and store for later retrieval
	results, err := v.client.PerfMetricsQuery(ctx, spec, vmPerfMetricList, vmRefs)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to retrieve perf metrics for VirtualMachines: %w", err))
	} else {
		v.scrapeData.vmPerfMetricsByRef = results.resultsByRef
	}

	// Get all VirtualMachine vSAN metrics and store for later retrieval
	vSANMetrics, err := v.client.VSANVirtualMachines(ctx, v.scrapeData.clusterRefs)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to retrieve vSAN metrics for VirtualMachines: %w", err))
		return
	}

	v.scrapeData.vmVSANMetricsByUUID = vSANMetrics.MetricResultsByUUID
}
