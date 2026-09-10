// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	go_ora "github.com/sijms/go-ora/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

// NewFactory creates a new Oracle receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createReceiverFunc(defaultSQLOpener, newDbClient), metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiverFunc(defaultSQLOpener, newDbClient), metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
		QuerySample: QuerySample{
			MaxRowsPerQuery: 100,
		},
		SessionWaitEvent: SessionWaitEvent{
			MaxRowsPerQuery: 100,
		},
		TopQueryCollection: TopQueryCollection{
			MaxQuerySampleCount: 1000,
			TopQueryCount:       200,
			CollectionInterval:  time.Minute,
		},
	}
}

// sqlOpenerFunc opens a database handle for the given config. It returns an
// optional cleanup function that the scraper must call on shutdown to release
// resources tied to the connection (for Kerberos, the gokrb5 client and its
// background TGT-renewal goroutine); cleanup is nil when there is nothing to
// release.
type sqlOpenerFunc func(cfg *Config) (*sql.DB, func(), error)

// defaultSQLOpener opens a database handle for the given config. For password
// authentication it opens the driver directly from the data source name. For
// Kerberos it builds a connector wired with a gokrb5-backed authenticator so
// go-ora performs the Kerberos handshake during connection, and returns a
// cleanup that destroys the gokrb5 client on shutdown.
func defaultSQLOpener(cfg *Config) (*sql.DB, func(), error) {
	dataSourceName := getDataSource(*cfg)

	if cfg.AuthType != AuthTypeKerberos {
		db, err := sql.Open("oracle", dataSourceName)
		return db, nil, err
	}

	krbClient, err := newKerberosClient(cfg.Kerberos)
	if err != nil {
		return nil, nil, err
	}

	connector, ok := go_ora.NewConnector(dataSourceName).(*go_ora.OracleConnector)
	if !ok {
		return nil, nil, errUnexpectedConnectorType
	}
	auth := &kerberosAuth{cfg: cfg.Kerberos, cl: krbClient}
	connector.WithKerberosAuth(auth)
	return sql.OpenDB(connector), auth.close, nil
}

func createReceiverFunc(sqlOpenerFunc sqlOpenerFunc, clientProviderFunc clientProviderFunc) receiver.CreateMetricsFunc {
	return func(
		_ context.Context,
		settings receiver.Settings,
		cfg component.Config,
		consumer consumer.Metrics,
	) (receiver.Metrics, error) {
		sqlCfg := cfg.(*Config)
		metricsBuilder := metadata.NewMetricsBuilder(sqlCfg.MetricsBuilderConfig, settings)

		instanceName, err := getInstanceName(getDataSource(*sqlCfg))
		if err != nil {
			return nil, err
		}
		hostName, hostNameErr := getHostName(getDataSource(*sqlCfg))
		if hostNameErr != nil {
			return nil, hostNameErr
		}

		// sqlOpenerFunc returns an optional cleanup alongside the DB. It runs at
		// start(); capture the cleanup so shutdown() can release it.
		var dbCleanup func()
		providerFunc := func() (*sql.DB, error) {
			db, cleanup, openErr := sqlOpenerFunc(sqlCfg)
			dbCleanup = cleanup
			return db, openErr
		}

		mp, err := newScraper(metricsBuilder, sqlCfg.MetricsBuilderConfig, sqlCfg.ControllerConfig, settings.Logger, providerFunc, clientProviderFunc, instanceName, hostName, func() {
			if dbCleanup != nil {
				dbCleanup()
			}
		})
		if err != nil {
			return nil, err
		}
		opt := scraperhelper.AddMetricsScraper(metadata.Type, mp)

		return scraperhelper.NewMetricsController(
			&sqlCfg.ControllerConfig,
			settings,
			consumer,
			opt,
		)
	}
}

func createLogsReceiverFunc(sqlOpenerFunc sqlOpenerFunc, clientProviderFunc clientProviderFunc) receiver.CreateLogsFunc {
	return func(
		_ context.Context,
		settings receiver.Settings,
		cfg component.Config,
		logsConsumer consumer.Logs,
	) (receiver.Logs, error) {
		sqlCfg := cfg.(*Config)

		logsBuilder := metadata.NewLogsBuilder(sqlCfg.LogsBuilderConfig, settings)

		instanceName, err := getInstanceName(getDataSource(*sqlCfg))
		if err != nil {
			return nil, err
		}

		hostName, hostNameErr := getHostName(getDataSource(*sqlCfg))
		if hostNameErr != nil {
			return nil, hostNameErr
		}

		// cacheSize is kept at 2 times MaxQuerySampleCount to keep queries of adjacent collections available for delta calculation.
		cacheSize := sqlCfg.TopQueryCollection.MaxQuerySampleCount * 2
		metricCache, err := lru.New[string, map[string]int64](int(cacheSize))
		if err != nil {
			settings.Logger.Error("Failed to create LRU cache, skipping the current scraper", zap.Error(err))
			return nil, err
		}

		// sqlOpenerFunc returns an optional cleanup alongside the DB. It runs at
		// start(); capture the cleanup so shutdown() can release it.
		var dbCleanup func()
		providerFunc := func() (*sql.DB, error) {
			db, cleanup, openErr := sqlOpenerFunc(sqlCfg)
			dbCleanup = cleanup
			return db, openErr
		}

		mp, err := newLogsScraper(logsBuilder, sqlCfg.LogsBuilderConfig, sqlCfg.ControllerConfig, settings.Logger, providerFunc, clientProviderFunc, instanceName, metricCache, sqlCfg.TopQueryCollection, sqlCfg.QuerySample, sqlCfg.SessionWaitEvent, hostName, func() {
			if dbCleanup != nil {
				dbCleanup()
			}
		})
		if err != nil {
			return nil, err
		}

		f := scraper.NewFactory(metadata.Type, nil,
			scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
				return mp, nil
			}, component.StabilityLevelAlpha))
		opt := scraperhelper.AddFactoryWithConfig(f, nil)

		return scraperhelper.NewLogsController(
			&sqlCfg.ControllerConfig,
			settings,
			logsConsumer,
			opt,
		)
	}
}

func getDataSource(cfg Config) string {
	if cfg.DataSource != "" {
		// A user-supplied data source takes precedence over the endpoint fields.
		// When Kerberos is selected, force AUTH TYPE=KERBEROS so go-ora performs
		// the Kerberos handshake with the authenticator wired in defaultSQLOpener;
		// the kerberos block is the source of truth for auth intent, so this
		// overrides any AUTH TYPE already present in the data source.
		if cfg.AuthType == AuthTypeKerberos {
			// Config validation guarantees the data source carries no
			// username/password and no conflicting AUTH TYPE, so we only need to
			// add the marker that makes go-ora perform the Kerberos handshake.
			if u, err := url.Parse(cfg.DataSource); err == nil {
				q := u.Query()
				q.Set("AUTH TYPE", "KERBEROS")
				u.RawQuery = q.Encode()
				return u.String()
			}
			// Validate already parsed the data source, so an error here is
			// unexpected; fall back to the raw value.
		}
		return cfg.DataSource
	}

	// Don't need to worry about errors here as config validation already checked.
	host, portStr, _ := net.SplitHostPort(cfg.Endpoint)
	port, _ := strconv.ParseInt(portStr, 10, 32)

	// Kerberos authenticates via an external ticket, so the data source carries
	// no username/password and instructs go-ora to use Kerberos.
	if cfg.AuthType == AuthTypeKerberos {
		return go_ora.BuildUrl(host, int(port), cfg.Service, "", "", map[string]string{
			"AUTH TYPE": "KERBEROS",
		})
	}

	return go_ora.BuildUrl(host, int(port), cfg.Service, cfg.Username, cfg.Password, nil)
}

func getInstanceName(datasource string) (string, error) {
	datasourceURL, err := url.Parse(datasource)
	if err != nil {
		return "", err
	}

	instanceName := datasourceURL.Host + datasourceURL.Path
	return instanceName, nil
}

func getHostName(datasource string) (string, error) {
	datasourceURL, err := url.Parse(datasource)
	if err != nil {
		return "", err
	}
	return datasourceURL.Host, nil
}
