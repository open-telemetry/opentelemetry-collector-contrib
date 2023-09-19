// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
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

var emitPerfMetricsWithObjects = featuregate.GlobalRegistry().MustRegister(
	emitPerfMetricsWithObjectsFeatureGateID,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the receiver emits vCenter performance metrics with object metric label dimension."),
)

var _ receiver.Metrics = (*vcenterMetricScraper)(nil)

type vcenterMetricScraper struct {
	client             *vcenterClient
	config             *Config
	mb                 *metadata.MetricsBuilder
	logger             *zap.Logger
	emitPerfWithObject bool
}

func newVmwareVcenterScraper(
	logger *zap.Logger,
	config *Config,
	settings receiver.CreateSettings,
) *vcenterMetricScraper {
	client := newVcenterClient(config)
	return &vcenterMetricScraper{
		client:             client,
		config:             config,
		logger:             logger,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		emitPerfWithObject: emitPerfMetricsWithObjects.IsEnabled(),
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
	clusters, err := v.client.Clusters(ctx, datacenter)
	if err != nil {
		errs.Add(err)
		return
	}
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, c := range clusters {
		v.collectHosts(ctx, now, c, errs)
		v.collectDatastores(ctx, now, c, errs)
		poweredOnVMs, poweredOffVMs := v.collectVMs(ctx, now, c, errs)
		v.collectCluster(ctx, now, c, poweredOnVMs, poweredOffVMs, errs)
	}
	v.collectResourcePools(ctx, now, errs)
}

func (v *vcenterMetricScraper) collectCluster(
	ctx context.Context,
	now pcommon.Timestamp,
	c *object.ClusterComputeResource,
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
	cluster *object.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	datastores, err := cluster.Datastores(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, ds := range datastores {
		v.collectDatastore(ctx, colTime, ds, cluster, errs)
	}
}

func (v *vcenterMetricScraper) collectDatastore(
	ctx context.Context,
	now pcommon.Timestamp,
	ds *object.Datastore,
	cluster *object.ClusterComputeResource,
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
	rb.SetVcenterClusterName(cluster.Name())
	rb.SetVcenterDatastoreName(moDS.Name)
	v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
}

func (v *vcenterMetricScraper) collectHosts(
	ctx context.Context,
	colTime pcommon.Timestamp,
	cluster *object.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	hosts, err := cluster.Hosts(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, h := range hosts {
		v.collectHost(ctx, colTime, h, cluster, errs)
	}
}

func (v *vcenterMetricScraper) collectHost(
	ctx context.Context,
	now pcommon.Timestamp,
	host *object.HostSystem,
	cluster *object.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	var hwSum mo.HostSystem
	err := host.Properties(ctx, host.Reference(),
		[]string{
			"config",
			"summary.hardware",
			"summary.quickStats",
		}, &hwSum)

	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	v.recordHostSystemMemoryUsage(now, hwSum)
	v.recordHostPerformanceMetrics(ctx, hwSum, errs)
	rb := v.mb.NewResourceBuilder()
	rb.SetVcenterHostName(host.Name())
	rb.SetVcenterClusterName(cluster.Name())
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
		}, &moRP)
		if err != nil {
			errs.AddPartial(1, err)
			continue
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
	cluster *object.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) (poweredOnVMs int64, poweredOffVMs int64) {
	vms, err := v.client.VMs(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for _, vm := range vms {
		var moVM mo.VirtualMachine
		err := vm.Properties(ctx, vm.Reference(), []string{
			"config",
			"runtime",
			"summary",
		}, &moVM)

		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		if string(moVM.Runtime.PowerState) == "poweredOff" {
			poweredOffVMs++
		} else {
			poweredOnVMs++
		}

		host, err := vm.HostSystem(ctx)
		if err != nil {
			errs.AddPartial(1, err)
			return
		}
		hostname, err := host.ObjectName(ctx)
		if err != nil {
			errs.AddPartial(1, err)
			return
		}

		var hwSum mo.HostSystem
		err = host.Properties(ctx, host.Reference(),
			[]string{
				"summary.hardware",
			}, &hwSum)

		if err != nil {
			errs.AddPartial(1, err)
			return
		}

		if moVM.Config == nil {
			errs.AddPartial(1, fmt.Errorf("vm config empty for %s", hostname))
			continue
		}
		vmUUID := moVM.Config.InstanceUuid

		v.collectVM(ctx, colTime, moVM, hwSum, errs)
		rb := v.mb.NewResourceBuilder()
		rb.SetVcenterVMName(vm.Name())
		rb.SetVcenterVMID(vmUUID)
		rb.SetVcenterClusterName(cluster.Name())
		rb.SetVcenterHostName(hostname)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
	return poweredOnVMs, poweredOffVMs
}

func (v *vcenterMetricScraper) collectVM(
	ctx context.Context,
	colTime pcommon.Timestamp,
	vm mo.VirtualMachine,
	hs mo.HostSystem,
	errs *scrapererror.ScrapeErrors,
) {
	v.recordVMUsages(colTime, vm, hs)
	v.recordVMPerformance(ctx, vm, errs)
}
