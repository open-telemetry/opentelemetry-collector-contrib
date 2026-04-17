// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	gnmi "github.com/openconfig/gnmi/proto/gnmi"
)

// gnmiClient manages a dial-in connection to a single gNMI target (Cisco or Arista switch).
type gnmiClient struct {
	target   TargetConfig
	encoding gnmi.Encoding
	consumer consumer.Metrics
	logger   *zap.Logger
	parser   *gnmiParser
}

// start initiates the connection loop with a fixed redial delay on failure.
// It runs until the context is cancelled (i.e. receiver Shutdown is called).
func (c *gnmiClient) start(ctx context.Context, redial time.Duration) {
	for ctx.Err() == nil {
		if err := c.connect(ctx); err != nil {
			c.logger.Warn("gNMI connection failed, retrying",
				zap.String("target", c.target.Address),
				zap.Duration("redial", redial),
				zap.Error(err))

			select {
			case <-ctx.Done():
				return
			case <-time.After(redial):
				// Wait before next connection attempt
			}
		}
	}
}

// connect handles gRPC dialing, authentication, subscription and the receive loop.
// Returns an error when the connection is lost — the caller (start) will redial.
func (c *gnmiClient) connect(ctx context.Context) error {
	// ── 1. Build gRPC dial options ────────────────────────────────────────────
	var opts []grpc.DialOption

	// Configure gRPC keepalive to prevent firewalls from dropping idle connections
	// and to detect dead TCP sessions faster.
	kacp := keepalive.ClientParameters{
		Time:                30 * time.Second, // Send a ping every 30s
		Timeout:             10 * time.Second, // Wait 10s for response before closing
		PermitWithoutStream: true,             // Allow pings even without active streams
	}
	opts = append(opts, grpc.WithKeepaliveParams(kacp))

	if c.target.TLSSetting.Insecure {
		// No TLS — plain-text connection (lab/dev environments)
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Load TLS config from the standard OTel configtls structure
		tlsCfg, err := c.target.TLSSetting.LoadTLSConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to load TLS config for %s: %w", c.target.Address, err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	// ── 2. Dial the network device ────────────────────────────────────────────
	dialTimeout := c.target.Timeout
	if dialTimeout <= 0 {
		dialTimeout = 30 * time.Second // Security Fallback
	}

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	// Use WithBlock() to ensure the connection is fully established before proceeding.
	// This provides more accurate error reporting during the dial phase.
	dialOpts := append(opts, grpc.WithBlock())
	conn, err := grpc.DialContext(dialCtx, c.target.Address, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to dial %s within %v (blocking): %w", c.target.Address, dialTimeout, err)
	}

	defer conn.Close()

	// ── 3. Inject credentials into gRPC metadata ──────────────────────────────
	// Cisco and Arista both accept username/password in the gRPC metadata headers.
	authCtx := ctx
	if c.target.Username != "" {
		authCtx = metadata.AppendToOutgoingContext(ctx,
			"username", c.target.Username,
			"password", c.target.Password)
	}

	// ── 4. Open the gNMI Subscribe stream ────────────────────────────────────
	client := gnmi.NewGNMIClient(conn)
	stream, err := client.Subscribe(authCtx)
	if err != nil {
		return fmt.Errorf("failed to open Subscribe stream to %s: %w", c.target.Address, err)
	}

	// ── 5. Send the SubscribeRequest (all configured paths in one request) ────
	req := c.buildSubscribeRequest()
	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send SubscribeRequest to %s: %w", c.target.Address, err)
	}

	c.logger.Info("gNMI subscription established",
		zap.String("target", c.target.Address),
		zap.Int("subscriptions", len(c.target.Subscriptions)))

	// ── 6. Continuous receive loop ────────────────────────────────────────────
	for ctx.Err() == nil {
		reply, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Device closed the stream gracefully
				c.logger.Info("gNMI stream closed by target",
					zap.String("target", c.target.Address))
				return nil
			}
			if ctx.Err() != nil {
				// Context cancelled — normal shutdown, not an error
				return nil
			}
			return fmt.Errorf("subscription aborted by %s: %w", c.target.Address, err)
		}

		// ── 7. Parse gNMI response → OTLP metrics ────────────────────────────
		metrics, parseErr := c.parser.parse(reply, c.target.Address)
		if parseErr != nil {
			c.logger.Error("Failed to parse gNMI response",
				zap.String("target", c.target.Address),
				zap.Error(parseErr))
			continue
		}

		// Only forward to the pipeline if we got actual metrics
		if metrics.ResourceMetrics().Len() == 0 {
			continue
		}

		if consumeErr := c.consumer.ConsumeMetrics(ctx, metrics); consumeErr != nil {
			c.logger.Error("Failed to consume metrics",
				zap.String("target", c.target.Address),
				zap.Error(consumeErr))
		} else {
			c.logger.Debug("Metrics successfully forwarded to pipeline",
				zap.String("target", c.target.Address),
				zap.Int("count", metrics.DataPointCount()))
		}

	}

	return nil
}

// buildSubscribeRequest creates a gnmi.SubscribeRequest from the target configuration.
// All subscriptions for a target are bundled into a single SubscriptionList.
func (c *gnmiClient) buildSubscribeRequest() *gnmi.SubscribeRequest {
	subscriptions := make([]*gnmi.Subscription, 0, len(c.target.Subscriptions))

	for _, sub := range c.target.Subscriptions {
		path := parsePath(sub.Origin, sub.Path)

		// Map subscription mode string → gNMI enum
		mode := gnmi.SubscriptionMode_SAMPLE
		switch strings.ToLower(sub.SubscriptionMode) {
		case "on_change":
			mode = gnmi.SubscriptionMode_ON_CHANGE
		case "target_defined":
			mode = gnmi.SubscriptionMode_TARGET_DEFINED
		}

		subscriptions = append(subscriptions, &gnmi.Subscription{
			Path:              path,
			Mode:              mode,
			SampleInterval:    uint64(sub.SampleInterval.Nanoseconds()),
			SuppressRedundant: sub.SuppressRedundant,
			// HeartbeatInterval forces an update even if the value hasn't changed.
			// Required by some Cisco on_change subscriptions to confirm liveness.
			HeartbeatInterval: uint64(sub.HeartbeatInterval.Nanoseconds()),
		})
	}

	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Mode:         gnmi.SubscriptionList_STREAM,
				Encoding:     c.encoding,
				Subscription: subscriptions,
			},
		},
	}
}

// parsePath converts a slash-separated string path into a structured gnmi.Path.
// The origin (e.g. "openconfig", "eos_native", "") is set at the Path level.
//
// Examples:
//
//	parsePath("openconfig", "/interfaces/interface/state/counters")
//	parsePath("eos_native", "/Kernel/proc/cpu/utilization/total")
//	parsePath("", "/Cisco-IOS-XE-interfaces-oper:interfaces")
func parsePath(origin, pathStr string) *gnmi.Path {
	parts := strings.Split(strings.Trim(pathStr, "/"), "/")
	elems := make([]*gnmi.PathElem, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			elems = append(elems, &gnmi.PathElem{Name: p})
		}
	}
	return &gnmi.Path{
		Origin: origin,
		Elem:   elems,
	}
}
