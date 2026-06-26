// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/notify"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/multierr"
)

// Default values surfaced through NewDefaultConfig.
const (
	DefaultQueueSize         = 10000
	DefaultWorkers           = 4
	DefaultMaxRecordsPerPost = 100
	DefaultMaxAttempts       = 3
	DefaultInitialBackoff    = 1 * time.Second
	DefaultMaxBackoff        = 30 * time.Second
	DefaultTimeout           = 10 * time.Second
)

// Config is the user-facing configuration block for HTTP webhook
// notifications emitted after successful S3 uploads.
//
// Transport, TLS, auth, headers, and timeout come from the embedded
// confighttp.ClientConfig. The remaining fields describe the in-memory queue,
// worker pool, size-triggered batching, and per-batch retry policy.
//
// The feature is disabled unless Endpoint is non-empty. An empty Endpoint
// short-circuits Validate so unrelated invariants are not enforced on the
// dormant block.
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`

	// QueueSize is the bounded capacity of the in-memory event channel.
	// Enqueue drops events non-blockingly once this capacity is reached.
	QueueSize int `mapstructure:"queue_size"`

	// Workers is the number of goroutines draining the queue in parallel.
	Workers int `mapstructure:"workers"`

	// MaxRecordsPerPost is the maximum number of records carried in a single
	// HTTP POST body. Batching is size-triggered only; there is no timer.
	MaxRecordsPerPost int `mapstructure:"max_records_per_post"`

	// MaxAttempts is the total number of delivery attempts per batch,
	// including the initial attempt. Must be >= 1.
	MaxAttempts int `mapstructure:"max_attempts"`

	// InitialBackoff is the base backoff between retries of a batch. The
	// effective delay is InitialBackoff * 2^attempt, capped at MaxBackoff,
	// with an independent jitter factor sampled in [0.5, 1.5) per attempt.
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`

	// MaxBackoff caps the base backoff before jitter is applied.
	MaxBackoff time.Duration `mapstructure:"max_backoff"`
}

// NewDefaultConfig returns the baseline Config. Transport defaults come from
// confighttp so behavior matches other HTTP-based collector components;
// feature-specific defaults encode the spec's recommended operating point.
func NewDefaultConfig() Config {
	cc := confighttp.NewDefaultClientConfig()
	cc.Timeout = DefaultTimeout
	return Config{
		ClientConfig:      cc,
		QueueSize:         DefaultQueueSize,
		Workers:           DefaultWorkers,
		MaxRecordsPerPost: DefaultMaxRecordsPerPost,
		MaxAttempts:       DefaultMaxAttempts,
		InitialBackoff:    DefaultInitialBackoff,
		MaxBackoff:        DefaultMaxBackoff,
	}
}

// Validate enforces the configuration invariants.
//
// When Endpoint is empty the block is dormant and Validate returns nil. This
// matches the "feature disabled by default" stance and avoids spurious
// errors on defaults that would never be used.
//
// Otherwise Validate delegates transport checks to confighttp and enforces
// the feature-specific invariants, accumulating errors with multierr so that
// a caller reporting "config invalid" can surface all root causes at once.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return nil
	}

	var errs error
	if err := c.ClientConfig.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}

	// Endpoint must be a well-formed http(s) URL with a host. confighttp
	// does not enforce this, and a scheme-only/host-less value would defer
	// failure to runtime request construction.
	if u, err := url.Parse(c.Endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = multierr.Append(errs, fmt.Errorf("notifications.endpoint must be http(s) URL: %q", c.Endpoint))
	}

	// Reserved request headers. confighttp overwrites colliding headers at
	// the transport layer, so allowing these would silently defeat the
	// notifier's Content-Type contract.
	for _, p := range c.Headers {
		switch strings.ToLower(p.Name) {
		case "content-type":
			errs = multierr.Append(errs, errors.New("notifications.headers must not override Content-Type"))
		case "content-encoding":
			errs = multierr.Append(errs, errors.New("notifications.headers must not override Content-Encoding"))
		}
	}

	// Request compression is not part of the receiver contract (plain JSON
	// POST). Reject any actively compressing value; the explicit "none"
	// value and the empty string both mean uncompressed and are accepted.
	if c.Compression.IsCompressed() {
		errs = multierr.Append(errs, errors.New("notifications.compression is not supported"))
	}

	if c.QueueSize < 1 {
		errs = multierr.Append(errs, errors.New("notifications.queue_size must be >= 1"))
	}
	if c.Workers < 1 {
		errs = multierr.Append(errs, errors.New("notifications.workers must be >= 1"))
	}
	if c.MaxRecordsPerPost < 1 {
		errs = multierr.Append(errs, errors.New("notifications.max_records_per_post must be >= 1"))
	}
	if c.MaxAttempts < 1 {
		errs = multierr.Append(errs, errors.New("notifications.max_attempts must be >= 1"))
	}
	if c.InitialBackoff <= 0 {
		errs = multierr.Append(errs, errors.New("notifications.initial_backoff must be > 0"))
	}
	if c.MaxBackoff < c.InitialBackoff {
		errs = multierr.Append(errs, errors.New("notifications.max_backoff must be >= initial_backoff"))
	}

	return errs
}
