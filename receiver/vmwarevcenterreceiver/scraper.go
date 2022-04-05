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

package vmwarevcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vsan/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver/internal/metadata"
)

// example 2022-03-10 14:15:00
const timeFormat = "2006-01-02 15:04:05"

const instrumentationLibraryName = "otelcol/vcenter"

var _ component.Receiver = (*vcenterMetricScraper)(nil)

type vcenterMetricScraper struct {
	client *VmwareVcenterClient
	mb     *metadata.MetricsBuilder
}

func newVmwareVcenterScraper(
	logger *zap.Logger,
	config *Config,
) *vcenterMetricScraper {
	l := logger.Named("vcenter-client")
	client := newVmwarevcenterClient(config, l)
	return &vcenterMetricScraper{
		client: client,
		mb:     metadata.NewMetricsBuilder(config.MetricsConfig.Metrics),
	}
}

func (v *vcenterMetricScraper) Start(ctx context.Context, _ component.Host) error {
	return v.client.Connect(ctx)
}

func (v *vcenterMetricScraper) Shutdown(ctx context.Context) error {
	return v.client.Disconnect(ctx)
}

func (v *vcenterMetricScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	if v.client == nil {
		return pdata.Metrics{}, errors.New("failed to connect to http client")
	}

	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()
	errs := &scrapererror.ScrapeErrors{}

	err := v.client.ConnectVSAN(ctx)
	if err != nil {
		errs.AddPartial(1, err)
	}

	v.collectClusters(ctx, rms, errs)

	return metrics, errs.Combine()
}

func (v *vcenterMetricScraper) collectClusters(ctx context.Context, rms pdata.ResourceMetricsSlice, errs *scrapererror.ScrapeErrors) error {
	clusters, err := v.client.Clusters(ctx)
	if err != nil {
		return err
	}
	now := pdata.NewTimestampFromTime(time.Now())

	for _, c := range clusters {
		rm := rms.AppendEmpty()
		v.collectCluster(ctx, c, rm, errs)
		v.collectHosts(ctx, now, c, rms, errs)
		v.collectDatastores(ctx, now, c, rms, errs)
		v.collectVMs(ctx, now, c, rms, errs)
		v.collectResourcePools(ctx, now, rms, errs)
	}

	return nil
}

func (v *vcenterMetricScraper) collectCluster(
	ctx context.Context,
	c *object.ClusterComputeResource,
	rm pdata.ResourceMetrics,
	errs *scrapererror.ScrapeErrors,
) {
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.Cluster, c.Name())

	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	v.collectClusterVSAN(ctx, c, errs)
	v.mb.EmitCluster(ilms.Metrics())
}

func (v *vcenterMetricScraper) collectClusterVSAN(
	ctx context.Context,
	cluster *object.ClusterComputeResource,
	errs *scrapererror.ScrapeErrors,
) {
	mor := cluster.Reference()
	csvs, err := v.client.CollectVSANCluster(ctx, &mor, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, r := range *csvs {
		time, err := time.Parse(timeFormat, r.SampleInfo)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		ts := pdata.NewTimestampFromTime(time)

		for _, val := range r.Value {
			values := strings.Split(val.Values, ",")
			for _, value := range values {
				v.recordClusterVsanMetric(ts, val.MetricId.Label, cluster.Name(), value, errs)
			}
		}
	}
}

func (v *vcenterMetricScraper) collectDatastores(
	ctx context.Context,
	colTime pdata.Timestamp,
	_ *object.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	datastores, err := v.client.DataStores(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for _, ds := range datastores {
		v.collectDatastore(ctx, colTime, rms, ds)
	}
}

func (v *vcenterMetricScraper) collectDatastore(
	ctx context.Context,
	now pdata.Timestamp,
	rms pdata.ResourceMetricsSlice,
	ds *object.Datastore,
) {
	rm := rms.AppendEmpty()
	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.Datastore, ds.Name())
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	var moDS mo.Datastore
	ds.Properties(ctx, ds.Reference(), []string{"summary"}, &moDS)
	v.recordDatastoreProperties(now, moDS)
	v.mb.EmitDatastore(ilms.Metrics())
}

func (v *vcenterMetricScraper) collectHosts(
	ctx context.Context,
	colTime pdata.Timestamp,
	cluster *object.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	hosts, err := v.client.Hosts(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	clusterRef := cluster.Reference()
	hostVsanCSVs, err := v.client.CollectVSANHosts(ctx, &clusterRef, time.Now(), time.Now())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, h := range hosts {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString("hostname", h.Name())

		ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

		v.collectHost(ctx, colTime, h, hostVsanCSVs, errs)
		v.mb.EmitHost(ilms.Metrics())
	}

}

func (v *vcenterMetricScraper) collectHost(
	ctx context.Context,
	now pdata.Timestamp,
	host *object.HostSystem,
	vsanCsvs *[]types.VsanPerfEntityMetricCSV,
	errs *scrapererror.ScrapeErrors,
) {
	var hwSum mo.HostSystem
	err := host.Properties(ctx, host.Reference(),
		[]string{
			"summary.hardware",
			"summary.quickStats",
		}, &hwSum)

	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	v.recordHostSystemMemoryUsage(now, hwSum, host.Name())
	if vsanCsvs != nil {
		entityRef := fmt.Sprintf("host-domclient:%v",
			hwSum.Summary.Hardware.Uuid,
		)
		v.addVSAN(*vsanCsvs, entityRef, host.Name(), "", hostType, errs)
	}
}

func (v *vcenterMetricScraper) collectResourcePools(
	ctx context.Context,
	ts pdata.Timestamp,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	rps, err := v.client.ResourcePools(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	for _, rp := range rps {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString("resource_pool", rp.Name())

		ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

		var moRP mo.ResourcePool
		rp.Properties(ctx, rp.Reference(), []string{
			"summary",
			"summary.quickStats",
			"name",
		}, &moRP)
		v.recordResourcePool(ts, moRP)

		v.mb.EmitResourcePool(ilms.Metrics())
	}
}

func (v *vcenterMetricScraper) collectVMs(
	ctx context.Context,
	colTime pdata.Timestamp,
	cluster *object.ClusterComputeResource,
	rms pdata.ResourceMetricsSlice,
	errs *scrapererror.ScrapeErrors,
) {
	vms, err := v.client.VMs(ctx)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	clusterRef := cluster.Reference()
	vsanCsvs, err := v.client.CollectVSANVirtualMachine(ctx, &clusterRef, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	for _, vm := range vms {
		rm := rms.AppendEmpty()
		resourceAttrs := rm.Resource().Attributes()
		resourceAttrs.InsertString(metadata.A.InstanceName, vm.Name())
		resourceAttrs.InsertString(metadata.A.InstanceID, vm.UUID(ctx))

		ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
		ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)
		v.collectVM(ctx, colTime, vm, vsanCsvs, errs)

		v.mb.EmitVM(ilms.Metrics())
	}

}

func (v *vcenterMetricScraper) collectVM(
	ctx context.Context,
	colTime pdata.Timestamp,
	vm *object.VirtualMachine,
	vsanCsvs *[]types.VsanPerfEntityMetricCSV,
	errs *scrapererror.ScrapeErrors,
) {
	var moVM mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{
		"config",
		"runtime",
		"summary",
	}, &moVM)
	vmUUID := moVM.Config.InstanceUuid

	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	ps := string(moVM.Runtime.PowerState)
	v.recordVMUsages(colTime, moVM, vmUUID, vm.Name())
	v.recordVMPerformance(ctx, moVM, vmUUID, ps, time.Now().Add(-15*time.Minute).UTC(), time.Now().Add(-5*time.Minute).UTC(), errs)

	if vsanCsvs != nil {
		entityRef := fmt.Sprintf("virtual-machine:%s", vmUUID)
		v.addVSAN(*vsanCsvs, entityRef, vm.Name(), ps, vmType, errs)
	}
}

type vsanType int

const (
	clusterType vsanType = iota
	hostType
	vmType
)

func (v *vcenterMetricScraper) addVSAN(
	csvs []types.VsanPerfEntityMetricCSV,
	entityID string,
	entityName string,
	// virtual machine specific
	powerState string,
	vsanType vsanType,
	errs *scrapererror.ScrapeErrors,
) {
	for _, r := range csvs {
		if r.EntityRefId != entityID || r.SampleInfo == "" {
			continue
		}

		time, err := time.Parse(timeFormat, r.SampleInfo)
		if err != nil {
			errs.AddPartial(1, err)
			continue
		}

		ts := pdata.NewTimestampFromTime(time)
		for _, val := range r.Value {
			values := strings.Split(val.Values, ",")
			for _, value := range values {
				switch vsanType {
				case clusterType:
					v.recordClusterVsanMetric(ts, val.MetricId.Label, entityName, value, errs)
				case hostType:
					v.recordHostVsanMetric(ts, val.MetricId.Label, entityName, value, errs)
				case vmType:
					instanceUUID := strings.Split(entityID, ":")[1]
					v.recordVMVsanMetric(ts, val.MetricId.Label, entityName, instanceUUID, powerState, value, errs)
				}
			}
		}
	}
}
