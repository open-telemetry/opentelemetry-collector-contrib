// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package aerospikereceiver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

var aerospikePort = "3000"

func TestIntegration(t *testing.T) {
	t.Run("6.2", integrationTest(func(*Config) {}))
	t.Run("6.2-cluster", integrationTest(func(cfg *Config) {
		cfg.CollectClusterMetrics = true
	}))
}

func integrationTest(cfgMod func(*Config)) func(*testing.T) {
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image:        "aerospike:ce-6.2.0.7_1",
				ExposedPorts: []string{aerospikePort},
				WaitingFor:   waitStrategy{},
				LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
					PostReadies: []testcontainers.ContainerHook{
						func(ctx context.Context, container testcontainers.Container) error {
							host, err := aerospikeHost(ctx, container)
							if err != nil {
								return err
							}
							return populateMetrics(host)
						},
					},
				}},
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.Endpoint = fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, aerospikePort))
				rCfg.ControllerConfig.CollectionInterval = 100 * time.Millisecond
				cfgMod(rCfg)
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreResourceAttributeValue("aerospike.node.name"),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run
}

type waitStrategy struct{}

func (ws waitStrategy) WaitUntilReady(ctx context.Context, st wait.StrategyTarget) error {
	if err := wait.ForAll(
		wait.ForListeningPort(nat.Port(aerospikePort)),
		wait.ForLog("service ready: soon there will be cake!"),
		wait.ForLog("NODE-ID"),
	).
		WithDeadline(time.Minute).
		WaitUntilReady(ctx, st); err != nil {
		return err
	}
	host, err := aerospikeHost(ctx, st)
	if err != nil {
		return err
	}
	var clientErr error
	for {
		select {
		case <-ctx.Done():
			return clientErr
		default:
			_, clientErr = as.NewClientWithPolicyAndHost(clientPolicy(), host)
			if clientErr == nil {
				return nil
			}
		}
	}
}

func aerospikeHost(ctx context.Context, st wait.StrategyTarget) (*as.Host, error) {
	host, err := st.Host(ctx)
	if err != nil {
		return nil, err
	}
	port, err := st.MappedPort(ctx, nat.Port(aerospikePort))
	if err != nil {
		return nil, err
	}
	return as.NewHost(host, port.Int()), nil
}

type doneCheckable interface {
	IsDone() (bool, as.Error)
}

type recordsCheckable interface {
	Results() <-chan *as.Result
}

type (
	aeroDoneFunc    func() (doneCheckable, as.Error)
	aeroRecordsFunc func() (recordsCheckable, as.Error)
)

func doneWaitAndCheck(f aeroDoneFunc) error {
	chk, err := f()
	if err != nil {
		return err
	}

	for res := false; !res; res, err = chk.IsDone() {
		if err != nil {
			return err
		}
		time.Sleep(time.Second / 3)
	}
	return nil
}

func recordsWaitAndCheck(f aeroRecordsFunc) error {
	chk, err := f()
	if err != nil {
		return err
	}

	// consume all records
	chk.Results()
	return nil
}

func clientPolicy() *as.ClientPolicy {
	clientPolicy := as.NewClientPolicy()
	clientPolicy.Timeout = 60 * time.Second
	// minconns is used to populate the client connections metric
	clientPolicy.MinConnectionsPerNode = 50
	return clientPolicy
}

func populateMetrics(host *as.Host) error {
	errSetFilter := errors.New("failed to set filter")
	errCreateSindex := errors.New("failed to create sindex")
	errRunningCreateSindex := errors.New("failed running create index")

	c, err := as.NewClientWithPolicyAndHost(clientPolicy(), host)
	if err != nil {
		return err
	}

	ns := "test"
	set := "integration"
	pibin := "bin1"
	sibin := "bin2"

	// write 100 records to get some memory usage
	for i := 0; i < 100; i++ {
		var key *as.Key
		key, err = as.NewKey(ns, set, i)
		if err != nil {
			return errors.New("failed to create key")
		}
		err = c.Put(nil, key, as.BinMap{pibin: i, sibin: i})
		if err != nil {
			return errors.New("failed to write record")
		}
	}

	// register UDFs for aggregation queries
	cwd, wderr := os.Getwd()
	if wderr != nil {
		return errors.New("can't get working directory")
	}

	udfFile := "udf"
	udfFunc := "sum_single_bin"
	luaPath := filepath.Join(cwd, "testdata", "integration/")
	as.SetLuaPath(luaPath)

	task, err := c.RegisterUDFFromFile(nil, filepath.Join(luaPath, udfFile+".lua"), udfFile+".lua", as.LUA)
	if err != nil {
		return errors.New("failed registering udf file")
	}
	if nil != <-task.OnComplete() {
		return errors.New("failed while registering udf file")
	}

	queryPolicy := as.NewQueryPolicy()
	queryPolicyShort := as.NewQueryPolicy()
	queryPolicyShort.ShortQuery = true

	var writePolicy *as.WritePolicy

	// *** Primary Index Queries *** //

	// perform a basic primary index query
	s1 := as.NewStatement(ns, set)
	if err := recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.Query(queryPolicy, s1)
	}); err != nil {
		return err
	}

	// aggregation query on primary index
	s2 := as.NewStatement(ns, set)
	if err := recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.QueryAggregate(queryPolicy, s2, "/"+udfFile, udfFunc, as.StringValue(pibin))
	}); err != nil {
		return err
	}

	// background udf query on primary index
	s3 := as.NewStatement(ns, set)
	if err := doneWaitAndCheck(func() (doneCheckable, as.Error) {
		return c.ExecuteUDF(queryPolicy, s3, "/"+udfFile, udfFunc, as.StringValue(pibin))
	}); err != nil {
		return err
	}

	// ops query on primary index
	s4 := as.NewStatement(ns, set)
	wbin := as.NewBin(pibin, 200)
	ops := as.PutOp(wbin)
	if err := doneWaitAndCheck(func() (doneCheckable, as.Error) {
		return c.QueryExecute(queryPolicy, writePolicy, s4, ops)
	}); err != nil {
		return err
	}

	// perform a basic short primary index query
	s5 := as.NewStatement(ns, set)
	if err := recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.Query(queryPolicyShort, s5)
	}); err != nil {
		return err
	}

	// *** Secondary Index Queries *** //

	// create secondary index for SI queries
	itask, err := c.CreateIndex(writePolicy, ns, set, "sitest", "bin2", as.NUMERIC)
	if err != nil {
		return errCreateSindex
	}
	if err = <-itask.OnComplete(); err != nil {
		return errRunningCreateSindex
	}

	// SI filter
	filt := as.NewRangeFilter(sibin, 0, 100)

	// perform a basic secondary index query
	s6 := as.NewStatement(ns, set)
	if sferr := s6.SetFilter(filt); sferr != nil {
		return errSetFilter
	}
	if err := recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.Query(queryPolicy, s6)
	}); err != nil {
		return err
	}

	// aggregation query on secondary index
	s7 := as.NewStatement(ns, set)
	if sferr := s7.SetFilter(filt); sferr != nil {
		return errSetFilter
	}
	if err := recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.QueryAggregate(queryPolicy, s7, "/"+udfFile, udfFunc, as.StringValue(sibin))
	}); err != nil {
		return err
	}

	// background udf query on secondary index
	s8 := as.NewStatement(ns, set)
	if sferr := s8.SetFilter(filt); sferr != nil {
		return errSetFilter
	}
	if err := doneWaitAndCheck(func() (doneCheckable, as.Error) {
		return c.ExecuteUDF(queryPolicy, s8, "/"+udfFile, udfFunc, as.StringValue(sibin))
	}); err != nil {
		return err
	}

	// ops query on secondary index
	s9 := as.NewStatement(ns, set)
	if sferr := s9.SetFilter(filt); sferr != nil {
		return errSetFilter
	}
	siwbin := as.NewBin("bin4", 400)
	siops := as.PutOp(siwbin)
	if err := doneWaitAndCheck(func() (doneCheckable, as.Error) {
		return c.QueryExecute(queryPolicy, writePolicy, s9, siops)
	}); err != nil {
		return err
	}

	// perform a basic short secondary index query
	s10 := as.NewStatement(ns, set)
	if sferr := s10.SetFilter(filt); sferr != nil {
		return errSetFilter
	}
	if err := recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.Query(queryPolicyShort, s10)
	}); err != nil {
		return err
	}

	// *** GeoJSON *** //

	bins := []as.BinMap{
		{
			"name":     "Bike Shop",
			"demand":   17923,
			"capacity": 17,
			"coord":    as.GeoJSONValue(`{"type" : "Point", "coordinates": [13.009318762,80.003157854]}`),
		},
		{
			"name":     "Residential Block",
			"demand":   2429,
			"capacity": 2974,
			"coord":    as.GeoJSONValue(`{"type" : "Point", "coordinates": [13.00961276, 80.003422154]}`),
		},
		{
			"name":     "Restaurant",
			"demand":   49589,
			"capacity": 4231,
			"coord":    as.GeoJSONValue(`{"type" : "Point", "coordinates": [13.009318762,80.003157854]}`),
		},
		{
			"name":     "Cafe",
			"demand":   247859,
			"capacity": 26,
			"coord":    as.GeoJSONValue(`{"type" : "Point", "coordinates": [13.00961276, 80.003422154]}`),
		},
		{
			"name":     "Park",
			"demand":   247859,
			"capacity": 26,
			"coord":    as.GeoJSONValue(`{"type" : "AeroCircle", "coordinates": [[0.0, 10.0], 10]}`),
		},
	}

	geoSet := "geoset"
	for i, b := range bins {
		key, _ := as.NewKey(ns, geoSet, i)
		err = c.Put(nil, key, b)
		if err != nil {
			return errors.New("failed to write geojson record")
		}
	}

	// create secondary index for geo queries
	itask, err = c.CreateIndex(writePolicy, ns, geoSet, "testset_geo_index", "coord", as.GEO2DSPHERE)
	if err != nil {
		return errCreateSindex
	}
	if err := <-itask.OnComplete(); err != nil {
		return errRunningCreateSindex
	}

	// run geoJSON query
	geoStm1 := as.NewStatement(ns, geoSet)
	geoFilt1 := as.NewGeoWithinRadiusFilter("coord", float64(13.009318762), float64(80.003157854), float64(50000))
	if sferr := geoStm1.SetFilter(geoFilt1); sferr != nil {
		return errSetFilter
	}
	return recordsWaitAndCheck(func() (recordsCheckable, as.Error) {
		return c.Query(queryPolicy, geoStm1)
	})
}
