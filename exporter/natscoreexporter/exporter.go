// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"context"
	"errors"
	"os"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

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

func setNatsSecureOption(options *nats.Options, ctx context.Context, cfg *configtls.ClientConfig) error {
	tlsConfig, err := cfg.LoadTLSConfig(ctx)
	if err != nil {
		return err
	}
	options.TLSConfig = tlsConfig
	return nil
}

func setNatsTokenOption(options *nats.Options, cfg *TokenConfig) {
	options.Token = cfg.Token
}

func setNatsUserInfoOption(options *nats.Options, cfg *UserInfoConfig) {
	options.User = cfg.User
	options.Password = cfg.Password
}

func setNatsNKeyOption(options *nats.Options, cfg *NkeyConfig) error {
	keyPair, err := nkeys.FromSeed(cfg.Seed)
	if err != nil {
		return err
	}

	options.Nkey = cfg.PublicKey
	options.SignatureCB = keyPair.Sign
	return nil
}

func setNatsUserJWTOption(options *nats.Options, cfg *UserJWTConfig) error {
	keyPair, err := nkeys.FromSeed(cfg.Seed)
	if err != nil {
		return err
	}

	options.UserJWT = func() (string, error) {
		return cfg.JWT, nil
	}
	options.SignatureCB = keyPair.Sign
	return nil
}

func setNatsUserCredentialsOption(options *nats.Options, cfg *UserCredentialsConfig) error {
	var errs error
	userConfig, err := os.ReadFile(cfg.UserFilePath)
	errs = multierr.Append(errs, err)
	userJwt, err := jwt.ParseDecoratedJWT(userConfig)
	errs = multierr.Append(errs, err)
	keyPair, err := jwt.ParseDecoratedNKey(userConfig)
	errs = multierr.Append(errs, err)
	if errs != nil {
		return errs
	}

	options.UserJWT = func() (string, error) {
		return userJwt, nil
	}
	options.SignatureCB = keyPair.Sign
	return nil
}

func setNatsAuthOption(options *nats.Options, cfg *AuthConfig) error {
	var errs error
	if cfg.UserInfo != nil {
		setNatsUserInfoOption(options, cfg.UserInfo)
	}
	if cfg.Token != nil {
		setNatsTokenOption(options, cfg.Token)
	}
	if cfg.Nkey != nil {
		errs = multierr.Append(errs, setNatsNKeyOption(options, cfg.Nkey))
	}
	if cfg.UserJWT != nil {
		errs = multierr.Append(errs, setNatsUserJWTOption(options, cfg.UserJWT))
	}
	if cfg.UserCredentials != nil {
		errs = multierr.Append(errs, setNatsUserCredentialsOption(options, cfg.UserCredentials))
	}
	return errs
}

func createNats(ctx context.Context, cfg *Config) (*nats.Conn, error) {
	var errs error
	options := nats.GetDefaultOptions()
	options.Url = cfg.Endpoint
	options.Pedantic = cfg.Pedantic
	errs = multierr.Append(errs, setNatsSecureOption(&options, ctx, &cfg.TLS))
	errs = multierr.Append(errs, setNatsAuthOption(&options, &cfg.Auth))
	if errs != nil {
		return nil, errs
	}

	conn, err := options.Connect()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (e *natsCoreExporter[T]) start(ctx context.Context, host component.Host) error {
	var errs error

	errs = multierr.Append(errs, e.marshaler.Resolve(host))

	conn, err := createNats(ctx, e.cfg)
	errs = multierr.Append(errs, err)
	e.conn = conn

	return errs
}

func (e *natsCoreExporter[T]) export(ctx context.Context, data T) error {
	var errs error

	groups, err := e.grouper.Group(ctx, data)
	errs = multierr.Append(errs, err)

	for _, group := range groups {
		bytes, err := e.marshaler.Marshal(group.Data)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		err = e.conn.Publish(group.Subject, bytes)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
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
	var errs error

	grouper, err := grouper.NewLogsGrouper(cfg.Logs.Subject, set.TelemetrySettings)
	errs = multierr.Append(errs, err)

	resolver, err := createResolver((*SignalConfig)(&cfg.Logs))
	errs = multierr.Append(errs, err)
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalLogs)

	return newNatsCoreExporter(set, cfg, grouper, marshaler), errs
}

func newNatsCoreMetricsExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[pmetric.Metrics], error) {
	var errs error

	grouper, err := grouper.NewMetricsGrouper(cfg.Metrics.Subject, set.TelemetrySettings)
	errs = multierr.Append(errs, err)

	resolver, err := createResolver((*SignalConfig)(&cfg.Metrics))
	errs = multierr.Append(errs, err)
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalMetrics)

	return newNatsCoreExporter(set, cfg, grouper, marshaler), errs
}

func newNatsCoreTracesExporter(set exporter.Settings, cfg *Config) (*natsCoreExporter[ptrace.Traces], error) {
	var errs error

	grouper, err := grouper.NewTracesGrouper(cfg.Traces.Subject, set.TelemetrySettings)
	errs = multierr.Append(errs, err)

	resolver, err := createResolver((*SignalConfig)(&cfg.Traces))
	errs = multierr.Append(errs, err)
	marshaler := marshaler.NewMarshaler(resolver, marshaler.PickMarshalTraces)

	return newNatsCoreExporter(set, cfg, grouper, marshaler), errs
}
