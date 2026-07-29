// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package mongodbreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongooptions "go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const mongoPort = "27017"

func TestIntegration(t *testing.T) {
	t.Run("4.4", integrationTest("4_4", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("5.0", integrationTest("5_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("6.0", integrationTest("6_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("7.0", integrationTest("7_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("4.4lpu", integrationTest("4_4lpu", []string{"/lpu.sh"}, func(cfg *Config) {
		cfg.Username = "otelu"
		cfg.Password = "otelp"
	}))
}

func integrationTest(name string, script []string, cfgMod func(*Config)) func(*testing.T) {
	dockerFile := fmt.Sprintf("Dockerfile.mongodb.%s", name)
	expectedFile := fmt.Sprintf("expected.%s.yaml", name)
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: dockerFile,
				},
				ExposedPorts: []string{mongoPort},
				WaitingFor:   wait.ForListeningPort(mongoPort).WithStartupTimeout(time.Minute),
				LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
					PostStarts: []testcontainers.ContainerHook{
						scraperinttest.RunScript(script),
					},
				}},
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				cfgMod(rCfg)
				rCfg.CollectionInterval = 2 * time.Second
				rCfg.MetricsBuilderConfig.Metrics.MongodbLockAcquireTime.Enabled = false
				rCfg.Hosts = []confignet.TCPAddrConfig{
					{
						Endpoint: fmt.Sprintf("%s:%s", ci.Host(t), ci.MappedPort(t, mongoPort)),
					},
				}
				rCfg.Insecure = true
			}),
		scraperinttest.WithExpectedFile(filepath.Join("testdata", "integration", expectedFile)),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreResourceAttributeValue("server.address"),
			pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
		),
	).Run
}

func TestTopQueryFromContainer(t *testing.T) {
	for _, version := range []string{"4.4", "5.0", "6.0", "7.0"} {
		t.Run(version, func(t *testing.T) {
			topQueryIntegrationTest(t, version)
		})
	}
}

func topQueryIntegrationTest(t *testing.T, mongoVersion string) {
	ci, err := testcontainers.GenericContainer(
		t.Context(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    filepath.Join("testdata", "integration"),
					Dockerfile: "Dockerfile.mongodb.top_query",
					BuildArgs:  map[string]*string{"MONGO_VERSION": &mongoVersion},
				},
				ExposedPorts: []string{mongoPort},
				WaitingFor:   wait.ForListeningPort(mongoPort).WithStartupTimeout(time.Minute),
				LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{
					PostStarts: []testcontainers.ContainerHook{
						scraperinttest.RunScript([]string{"/setup_top_query.sh"}),
					},
				}},
			},
		},
	)
	assert.NoError(t, err)
	assert.NoError(t, ci.Start(t.Context()))
	defer testcontainers.CleanupContainer(t, ci)

	port, err := ci.MappedPort(t.Context(), mongoPort)
	assert.NoError(t, err)
	endpoint := fmt.Sprintf("localhost:%s", port.Port())

	cfg := createDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddrConfig{{Endpoint: endpoint}}
	cfg.Insecure = true
	cfg.DirectConnection = true
	lbc := metadata.DefaultLogsBuilderConfig()
	lbc.Events.DbServerTopQuery.Enabled = true
	cfg.LogsBuilderConfig = lbc
	cfg.TopQueryCollection.CollectionInterval = time.Millisecond

	s := newMongodbScraper(receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}, cfg)
	assert.NoError(t, s.start(t.Context(), nil))
	defer func() { assert.NoError(t, s.shutdown(t.Context())) }()

	queryCtx, cancelQuery := context.WithCancel(t.Context())
	defer cancelQuery()
	var queriesRun atomic.Int32
	go func() {
		mongoClient, clientErr := mongo.Connect(mongooptions.Client().ApplyURI("mongodb://" + endpoint))
		if clientErr != nil {
			return
		}
		defer mongoClient.Disconnect(t.Context()) //nolint:errcheck
		coll := mongoClient.Database("testdb").Collection("orders")
		for {
			select {
			case <-queryCtx.Done():
				return
			default:
				coll.Find(queryCtx, bson.M{"status": "pending"}) //nolint:errcheck
				queriesRun.Add(1)
			}
		}
	}()

	assert.Eventually(t, func() bool {
		return queriesRun.Load() > 0
	}, 10*time.Second, 100*time.Millisecond, "workload did not start in time")

	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		logs, scrapeErr := s.scrapeTopQueryLogs(t.Context())
		assert.NoError(tt, scrapeErr)

		found := false
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			scopeLogs := logs.ResourceLogs().At(i).ScopeLogs()
			for j := 0; j < scopeLogs.Len(); j++ {
				records := scopeLogs.At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					attrs := records.At(k).Attributes().AsRaw()
					if ns, ok := attrs["db.namespace"]; !ok || ns != "testdb" {
						continue
					}
					if col, ok := attrs["db.collection.name"]; !ok || col != "orders" {
						continue
					}
					assert.NotEmpty(tt, attrs["db.query.text"], "db.query.text should be non-empty")
					assert.NotEmpty(tt, attrs["db.operation.name"], "db.operation.name should be non-empty")
					found = true
				}
			}
		}
		assert.True(tt, found, "expected a top_query event for testdb.orders")
	}, 30*time.Second, 500*time.Millisecond)
}
