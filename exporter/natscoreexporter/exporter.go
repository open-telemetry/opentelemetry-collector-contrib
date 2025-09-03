// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

func createNatsTokenOption(cfg *Config) nats.Option {
	return nats.Token(cfg.Auth.Token.Token)
}

func createNatsUserInfoOption(cfg *Config) nats.Option {
	return nats.UserInfo(cfg.Auth.UserInfo.User, cfg.Auth.UserInfo.Password)
}

func createNatsNKeyOption(cfg *Config) (nats.Option, error) {
	keyPair, err := nkeys.FromSeed([]byte(cfg.Auth.UserJWT.Seed))
	if err != nil {
		return nil, err
	}

	return nats.Nkey(cfg.Auth.NKey.PublicKey, keyPair.Sign), nil
}

func createNatsUserJWTOption(cfg *Config) (nats.Option, error) {
	keyPair, err := nkeys.FromSeed([]byte(cfg.Auth.UserJWT.Seed))
	if err != nil {
		return nil, err
	}

	userCB := func() (string, error) {
		return cfg.Auth.UserJWT.JWT, nil
	}
	return nats.UserJWT(userCB, keyPair.Sign), nil
}

func createNatsUserCredentialsOption(cfg *Config) nats.Option {
	return nats.UserCredentials(cfg.Auth.UserCredentials.UserFile)
}

func createNatsOptions(cfg *Config) ([]nats.Option, error) {
	var authOption nats.Option
	var err error
	if cfg.Auth.UserInfo != nil {
		authOption = createNatsUserInfoOption(cfg)
	} else if cfg.Auth.Token != nil {
		authOption = createNatsTokenOption(cfg)
	} else if cfg.Auth.NKey != nil {
		authOption, err = createNatsNKeyOption(cfg)
	} else if cfg.Auth.UserJWT != nil {
		authOption, err = createNatsUserJWTOption(cfg)
	} else if cfg.Auth.UserCredentials != nil {
		authOption = createNatsUserCredentialsOption(cfg)
	} else {
		return nil, errors.New("no auth method configured")
	}
	if err != nil {
		return nil, err
	}
	return []nats.Option{authOption}, nil
}

func createNats(cfg *Config) (*nats.Conn, error) {
	natsOptions, err := createNatsOptions(cfg)
	if err != nil {
		return nil, err
	}

	conn, err := nats.Connect(cfg.Endpoint, natsOptions...)
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

func createResolver(builtinMarshalerName marshaler.BuiltinMarshalerName, encodingExtensionName string) (marshaler.Resolver, error) {
	if builtinMarshalerName != "" {
		return marshaler.NewBuiltinMarshalerResolver(builtinMarshalerName)
	} else if encodingExtensionName != "" {
		return marshaler.NewEncodingExtensionResolver(encodingExtensionName)
	} else {
		return nil, errors.New("no built-in marshaler or encoding extension configured")
	}
}

func newNatsCoreLogsExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[plog.Logs], error) {
	grouper, err := grouper.NewLogsGrouper(cfg.Logs.Subject, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	resolver, err := createResolver(cfg.Logs.BuiltinMarshalerName, cfg.Logs.EncodingExtensionName)
	if err != nil {
		return nil, err
	}
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalLogs)

	exporter := newNatsCoreExporter(set, cfg, grouper, marshaler)
	return exporter, nil
}

func newNatsCoreMetricsExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[pmetric.Metrics], error) {
	grouper, err := grouper.NewMetricsGrouper(cfg.Metrics.Subject, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	resolver, err := createResolver(cfg.Metrics.BuiltinMarshalerName, cfg.Metrics.EncodingExtensionName)
	if err != nil {
		return nil, err
	}
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalMetrics)

	exporter := newNatsCoreExporter(set, cfg, grouper, marshaler)
	return exporter, nil
}

func newNatsCoreTracesExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[ptrace.Traces], error) {
	grouper, err := grouper.NewTracesGrouper(cfg.Traces.Subject, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	resolver, err := createResolver(cfg.Traces.BuiltinMarshalerName, cfg.Traces.EncodingExtensionName)
	if err != nil {
		return nil, err
	}
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalTraces)

	exporter := newNatsCoreExporter(set, cfg, grouper, marshaler)
	return exporter, nil
}
