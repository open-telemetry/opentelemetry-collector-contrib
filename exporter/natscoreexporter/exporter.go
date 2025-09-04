// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"
)

type natsCoreExporter[T any] struct {
	set       exporter.Settings
	cfg       *Config
	grouper   grouper.Grouper[T]
	marshaler marshaler.Marshaler[T]
	conn      *nats.Conn
}

func newNatsCoreExporter[T any](
	set exporter.Settings,
	cfg *Config,
	grouper grouper.Grouper[T],
	marshaler marshaler.Marshaler[T],
) *natsCoreExporter[T] {
	return &natsCoreExporter[T]{
		set:       set,
		cfg:       cfg,
		grouper:   grouper,
		marshaler: marshaler,
	}
}

func createNatsTokenOption(cfg *TokenConfig) nats.Option {
	return nats.Token(cfg.Token)
}

func createNatsUserInfoOption(cfg *UserInfoConfig) nats.Option {
	return nats.UserInfo(cfg.User, cfg.Password)
}

func createNatsNKeyOption(cfg *NKeyConfig) (nats.Option, error) {
	keyPair, err := nkeys.FromSeed([]byte(cfg.Seed))
	if err != nil {
		return nil, err
	}

	return nats.Nkey(cfg.PublicKey, keyPair.Sign), nil
}

func createNatsUserJWTOption(cfg *UserJWTConfig) (nats.Option, error) {
	keyPair, err := nkeys.FromSeed([]byte(cfg.Seed))
	if err != nil {
		return nil, err
	}

	userCB := func() (string, error) {
		return cfg.JWT, nil
	}
	return nats.UserJWT(userCB, keyPair.Sign), nil
}

func createNatsUserCredentialsOption(cfg *UserCredentialsConfig) nats.Option {
	return nats.UserCredentials(cfg.UserFile)
}

func createNatsAuthOption(cfg *AuthConfig) (nats.Option, error) {
	var authOption nats.Option
	var err error
	if cfg.UserInfo != nil {
		authOption = createNatsUserInfoOption(cfg.UserInfo)
	} else if cfg.Token != nil {
		authOption = createNatsTokenOption(cfg.Token)
	} else if cfg.NKey != nil {
		authOption, err = createNatsNKeyOption(cfg.NKey)
	} else if cfg.UserJWT != nil {
		authOption, err = createNatsUserJWTOption(cfg.UserJWT)
	} else if cfg.UserCredentials != nil {
		authOption = createNatsUserCredentialsOption(cfg.UserCredentials)
	}
	if err != nil {
		return nil, err
	}
	return authOption, nil
}

func createNats(cfg *Config) (*nats.Conn, error) {
	authOption, err := createNatsAuthOption(&cfg.Auth)
	if err != nil {
		return nil, err
	}

	conn, err := nats.Connect(cfg.Endpoint, authOption)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (e *natsCoreExporter[T]) start(_ context.Context, host component.Host) error {
	err := e.marshaler.Resolve(host)
	if err != nil {
		return err
	}

	conn, err := createNats(e.cfg)
	if err != nil {
		return err
	}
	e.conn = conn

	return nil
}

func (e *natsCoreExporter[T]) export(ctx context.Context, data T) error {
	groups, err := e.grouper.Group(ctx, data)
	if err != nil {
		return err
	}

	for _, group := range groups {
		bytes, err := e.marshaler.Marshal(group.Data)
		if err != nil {
			return err
		}

		err = e.conn.Publish(group.Subject, bytes)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *natsCoreExporter[T]) shutdown(_ context.Context) error {
	return e.conn.Drain()
}

func createResolver(cfg *SignalConfig) (marshaler.Resolver, error) {
	if cfg.BuiltinMarshalerName != "" {
		return marshaler.NewBuiltinMarshalerResolver(cfg.BuiltinMarshalerName)
	} else if cfg.EncodingExtensionName != "" {
		return marshaler.NewEncodingExtensionResolver(cfg.EncodingExtensionName)
	} else {
		return nil, errors.New("no built-in marshaler or encoding extension configured")
	}
}

func newNatsCoreLogsExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[plog.Logs], error) {
	grouper, err := grouper.NewLogsGrouper(cfg.Logs.Subject, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	resolver, err := createResolver((*SignalConfig)(&cfg.Logs))
	if err != nil {
		return nil, err
	}
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalLogs)

	return newNatsCoreExporter(set, cfg, grouper, marshaler), nil
}

func newNatsCoreMetricsExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[pmetric.Metrics], error) {
	grouper, err := grouper.NewMetricsGrouper(cfg.Metrics.Subject, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	resolver, err := createResolver((*SignalConfig)(&cfg.Metrics))
	if err != nil {
		return nil, err
	}
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalMetrics)

	return newNatsCoreExporter(set, cfg, grouper, marshaler), nil
}

func newNatsCoreTracesExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[ptrace.Traces], error) {
	grouper, err := grouper.NewTracesGrouper(cfg.Traces.Subject, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	resolver, err := createResolver((*SignalConfig)(&cfg.Traces))
	if err != nil {
		return nil, err
	}
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalTraces)

	return newNatsCoreExporter(set, cfg, grouper, marshaler), nil
}
