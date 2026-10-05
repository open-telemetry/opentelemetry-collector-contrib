// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// connect opens a NATS connection using the exporter configuration. name labels
// the connection on the server (connz), and logger records connection lifecycle
// events (disconnect/reconnect/close).
func connect(ctx context.Context, cfg *Config, name string, logger *zap.Logger) (*nats.Conn, error) {
	var errs error
	options := nats.GetDefaultOptions()
	options.Url = cfg.Endpoint
	options.Name = name
	options.Pedantic = cfg.Pedantic
	setConnHandlers(&options, logger)
	errs = multierr.Append(errs, setTLSOption(ctx, &options, &cfg.TLS))
	errs = multierr.Append(errs, setAuthOption(&options, &cfg.Auth))
	if errs != nil {
		return nil, errs
	}

	return options.Connect()
}

// setConnHandlers logs NATS connection lifecycle events so disconnects and
// reconnects are visible in the collector logs.
func setConnHandlers(options *nats.Options, logger *zap.Logger) {
	options.DisconnectedErrCB = func(_ *nats.Conn, err error) {
		// A clean Close() also invokes this callback with a nil error; only an
		// unexpected disconnect carries one.
		if err == nil {
			logger.Info("NATS disconnected")
			return
		}
		logger.Warn("NATS disconnected", zap.Error(err))
	}
	options.ReconnectedCB = func(c *nats.Conn) {
		logger.Info("NATS reconnected", zap.String("url", c.ConnectedUrl()))
	}
	options.ClosedCB = func(_ *nats.Conn) {
		logger.Info("NATS connection closed")
	}
}

func setTLSOption(ctx context.Context, options *nats.Options, cfg *configtls.ClientConfig) error {
	tlsConfig, err := cfg.LoadTLSConfig(ctx)
	if err != nil {
		return err
	}
	options.TLSConfig = tlsConfig
	return nil
}

func setTokenOption(options *nats.Options, cfg *TokenConfig) {
	options.Token = string(cfg.Token)
}

func setUserOption(options *nats.Options, cfg *UserConfig) {
	options.User = cfg.Username
	options.Password = string(cfg.Password)
}

func setNkeyOption(options *nats.Options, cfg *NkeyConfig) error {
	keyPair, err := nkeys.FromSeed(cfg.Seed)
	if err != nil {
		return err
	}

	options.Nkey = cfg.PublicKey
	options.SignatureCB = keyPair.Sign
	return nil
}

func setNkeyJWTOption(options *nats.Options, cfg *NkeyJWTConfig) error {
	keyPair, err := nkeys.FromSeed(cfg.Seed)
	if err != nil {
		return err
	}

	userJWT := string(cfg.JWT)
	options.UserJWT = func() (string, error) {
		return userJWT, nil
	}
	options.SignatureCB = keyPair.Sign
	return nil
}

func setNkeyUserFileOption(options *nats.Options, cfg *NkeyUserFileConfig) error {
	// UserCredentials installs UserJWT/SignatureCB callbacks that re-read the
	// creds file each time they run, so a rotated (short-lived) credential is
	// picked up on reconnect rather than pinned at first connect. It also smoke-
	// tests the file once here, so a missing/unreadable file still errors eagerly.
	return nats.UserCredentials(cfg.UserFilePath)(options)
}

func setAuthOption(options *nats.Options, cfg *AuthConfig) error {
	var errs error
	if cfg.User != nil {
		setUserOption(options, cfg.User)
	}
	if cfg.Token != nil {
		setTokenOption(options, cfg.Token)
	}
	if cfg.Nkey != nil {
		errs = multierr.Append(errs, setNkeyOption(options, cfg.Nkey))
	}
	if cfg.NkeyJWT != nil {
		errs = multierr.Append(errs, setNkeyJWTOption(options, cfg.NkeyJWT))
	}
	if cfg.NkeyUserFile != nil {
		errs = multierr.Append(errs, setNkeyUserFileOption(options, cfg.NkeyUserFile))
	}
	return errs
}
