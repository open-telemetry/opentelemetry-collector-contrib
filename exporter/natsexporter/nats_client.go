// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

const defaultJetStreamPublishTimeout = 5 * time.Second

// natsClient is a thin wrapper around a NATS connection. When js is non-nil,
// messages are published via JetStream; otherwise via core NATS.
type natsClient struct {
	conn           *nats.Conn
	js             jetstream.JetStream
	publishTimeout time.Duration
}

func newNatsClient(ctx context.Context, cfg Config, logger *zap.Logger) (*natsClient, error) {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.Timeout(cfg.Connection.Timeout),
		nats.MaxReconnects(cfg.Connection.MaxReconnects),
		nats.ReconnectWait(cfg.Connection.ReconnectWait),
		// Return from Connect without error even if the server is not yet
		// reachable, and let the client reconnect in the background.
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("Disconnected from NATS", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("Reconnected to NATS", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			logger.Error("Asynchronous NATS error", zap.Error(err))
		}),
	}

	authOpt, err := authOption(cfg.Auth)
	if err != nil {
		return nil, err
	}
	if authOpt != nil {
		opts = append(opts, authOpt)
	}

	if cfg.TLS.HasValue() {
		tlsClientCfg := cfg.TLS.Get()

		var tlsCfg *tls.Config
		tlsCfg, err = tlsClientCfg.LoadTLSConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		opts = append(opts, nats.Secure(tlsCfg))
	}

	conn, err := nats.Connect(strings.Join(cfg.URLs, ","), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	client := &natsClient{conn: conn}

	if cfg.JetStream.HasValue() {
		js, err := jetstream.New(conn)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create JetStream context: %w", err)
		}
		client.js = js
		client.publishTimeout = cfg.JetStream.Get().PublishTimeout
		if client.publishTimeout <= 0 {
			client.publishTimeout = defaultJetStreamPublishTimeout
		}
	}

	return client, nil
}

func authOption(auth Authentication) (nats.Option, error) {
	switch {
	case auth.Token.HasValue():
		return nats.Token(string(auth.Token.Get().Token)), nil
	case auth.UserPassword.HasValue():
		up := auth.UserPassword.Get()
		return nats.UserInfo(up.Username, string(up.Password)), nil
	case auth.NKey.HasValue():
		opt, err := nats.NkeyOptionFromSeed(auth.NKey.Get().SeedFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load NKey seed file: %w", err)
		}
		return opt, nil
	case auth.Credentials.HasValue():
		return nats.UserCredentials(auth.Credentials.Get().File), nil
	}
	return nil, nil
}

func (c *natsClient) publish(ctx context.Context, subject string, data []byte) error {
	if c.js != nil {
		pctx := ctx
		if c.publishTimeout > 0 {
			var cancel context.CancelFunc
			pctx, cancel = context.WithTimeout(ctx, c.publishTimeout)
			defer cancel()
		}
		_, err := c.js.Publish(pctx, subject, data)
		return err
	}
	if err := c.conn.Publish(subject, data); err != nil {
		return err
	}
	// Flush so a publish is only reported as successful once the server has
	// acknowledged receipt, giving the exporterhelper queue/retry meaningful
	// semantics for core NATS. FlushWithContext requires the context to carry a
	// deadline (exporterhelper supplies one via its timeout); otherwise fall
	// back to Flush, which uses the connection's own timeout.
	if _, ok := ctx.Deadline(); ok {
		return c.conn.FlushWithContext(ctx)
	}
	return c.conn.Flush()
}

func (c *natsClient) close(context.Context) error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}
