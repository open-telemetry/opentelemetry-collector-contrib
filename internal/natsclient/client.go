// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// Conn is a NATS connection that reports its health as component status.
type Conn struct {
	*nats.Conn
	// closing is set by Close so the lifecycle callbacks can tell an
	// intentional close from a connection that was lost.
	closing atomic.Bool
}

// Close closes the connection without reporting it as a failure.
func (c *Conn) Close() {
	c.closing.Store(true)
	c.Conn.Close()
}

// Connect opens a NATS connection using cfg. name labels the connection on the
// server (connz). Connection lifecycle events are logged with logger and
// reported as component status through host: recoverable while reconnecting,
// OK once reconnected, and permanent if the connection is closed for good.
func Connect(ctx context.Context, cfg *ClientConfig, name string, host component.Host, logger *zap.Logger) (*Conn, error) {
	var errs error
	options := nats.GetDefaultOptions()
	options.Url = cfg.Endpoint
	options.Name = name
	options.Pedantic = cfg.Pedantic
	// Reconnect indefinitely: the nats.go default gives up after 60 attempts
	// (~2 minutes), after which the connection stays closed until a restart.
	options.MaxReconnect = -1
	c := &Conn{}
	setConnHandlers(&options, c, host, logger)
	errs = multierr.Append(errs, setTLSOption(ctx, &options, &cfg.TLS))
	errs = multierr.Append(errs, setAuthOption(&options, &cfg.Auth))
	if errs != nil {
		return nil, errs
	}

	nc, err := options.Connect()
	if err != nil {
		return nil, err
	}
	c.Conn = nc
	return c, nil
}

// setConnHandlers logs NATS connection lifecycle events and reports them as
// component status, so a lost connection is visible to health checks.
func setConnHandlers(options *nats.Options, c *Conn, host component.Host, logger *zap.Logger) {
	options.DisconnectedErrCB = func(_ *nats.Conn, err error) {
		// A clean Close() also invokes this callback with a nil error; only an
		// unexpected disconnect carries one.
		if err == nil {
			logger.Info("NATS disconnected")
			return
		}
		logger.Warn("NATS disconnected", zap.Error(err))
		if !c.closing.Load() {
			componentstatus.ReportStatus(host, componentstatus.NewRecoverableErrorEvent(
				fmt.Errorf("NATS disconnected, reconnecting: %w", err),
			))
		}
	}
	options.ReconnectedCB = func(nc *nats.Conn) {
		logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		if !c.closing.Load() {
			componentstatus.ReportStatus(host, componentstatus.NewEvent(componentstatus.StatusOK))
		}
	}
	options.ClosedCB = func(nc *nats.Conn) {
		if c.closing.Load() {
			logger.Info("NATS connection closed")
			return
		}
		// Not closed by us: nats.go gave up, e.g. after the server rejected the
		// same credentials twice in a row. Retrying would not help.
		err := nc.LastError()
		if err == nil {
			err = errors.New("closed by the client library")
		}
		logger.Error("NATS connection closed and will not reconnect", zap.Error(err))
		componentstatus.ReportStatus(host, componentstatus.NewPermanentErrorEvent(
			fmt.Errorf("NATS connection closed and will not reconnect: %w", err),
		))
	}
	// Async errors include slow consumers: a subscription whose pending buffer
	// is full drops messages until the handler catches up.
	options.AsyncErrorCB = func(_ *nats.Conn, sub *nats.Subscription, err error) {
		if sub == nil {
			logger.Warn("NATS async error", zap.Error(err))
			return
		}
		logger.Warn("NATS async error", zap.String("subject", sub.Subject), zap.Error(err))
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
