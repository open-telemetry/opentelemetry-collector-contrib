// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package mongodbreceiver

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tj/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	t.Run("4.0", integrationTest("4_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("5.0", integrationTest("5_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("6.0", integrationTest("5_0", []string{"/setup.sh"}, func(*Config) {}))
	t.Run("7.0", integrationTest("5_0", []string{"/setup.sh"}, func(*Config) {}))
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
		),
	).Run
}

func TestScrapeLogsFromContainer(t *testing.T) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "mongo:noble",
				Env: map[string]string{
					"MONGO_INITDB_ROOT_USERNAME": "admin",
					"MONGO_INITDB_ROOT_PASSWORD": "admin",
				},
				Files: []testcontainers.ContainerFile{
					{
						HostFilePath:      filepath.Join("testdata", "integration", "scripts", "init-mongo.js"),
						ContainerFilePath: "/docker-entrypoint-initdb.d/init-mongo.js",
						FileMode:          700,
					},
				},
				ExposedPorts: []string{mongoPort},
				WaitingFor: wait.ForListeningPort(mongoPort).
					WithStartupTimeout(2 * time.Minute),
			},
			Started: true,
		})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx))
	})

	p, err := container.MappedPort(ctx, mongoPort)
	require.NoError(t, err)

	connStr := fmt.Sprintf("mongodb://app_user:app_password@localhost:%s", p.Port())
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connStr))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Disconnect(ctx))
	})

	cfg := createDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddrConfig{
		{
			Endpoint: net.JoinHostPort("localhost", p.Port()),
		},
	}
	cfg.Username = "otel-user"
	cfg.Password = "otel-password"
	cfg.Insecure = true
	cfg.InsecureSkipVerify = true
	cfg.CollectionInterval = 100 * time.Millisecond
	cfg.MetricsBuilderConfig = metadata.DefaultMetricsBuilderConfig()

	settings := receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}
	scraper := newMongodbScraper(settings, cfg)
	require.NoError(t, err)

	// Start a goroutine to send queries periodically.
	queryCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := atomic.Bool{}
	finished.Store(false)
	go func() {
		coll := client.Database("sample_db").Collection("users")
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, err := coll.Find(queryCtx, bson.M{"$where": "sleep(500); return true;"})
				if !finished.Load() {
					assert.NoError(t, err)
				}
			case <-queryCtx.Done():
				return
			}
		}
	}()
	assert.NoError(t, scraper.start(context.Background(), nil))
	defer func() {
		assert.NoError(t, scraper.shutdown(context.Background()))
	}()

	// Assert that we eventually get the logs we expect.
	require.Eventually(t, func() bool {
		logs, err := scraper.scrapeLogs(context.Background())
		assert.NoError(t, err)
		if logs.ResourceLogs().Len() == 0 || logs.ResourceLogs().At(0).ScopeLogs().Len() == 0 {
			return false
		}
		logRecords := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		if logRecords.Len() == 0 {
			return false
		}

		for i := 0; i < logRecords.Len(); i++ {
			record := logRecords.At(i)
			attributes := record.Attributes().AsRaw()
			queryAttribute, ok := attributes["db.query.text"]
			if !ok {
				continue
			}

			if attributes["db.collection.name"].(string) == "users" && strings.Contains(queryAttribute.(string), `{"find":"?"`) && attributes["mongodb.query.plan"].(string) != "" {
				return true
			}
		}

		return false
	}, 60*time.Second, 1*time.Millisecond, "failed to find expected log record")
	finished.Store(true)
}
