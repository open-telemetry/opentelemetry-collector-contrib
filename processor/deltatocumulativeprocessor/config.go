// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	telemetry "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"
)

var _ xconfmap.Validator = (*Config)(nil)

type Config struct {
	MaxStale   time.Duration `mapstructure:"max_stale"`
	MaxStreams int           `mapstructure:"max_streams"`

	StorageID    *component.ID `mapstructure:"storage"`
	WriteThrough bool          `mapstructure:"write_through"`
}

func (c *Config) Validate() error {
	if c.MaxStale <= 0 {
		return fmt.Errorf("max_stale must be a positive duration (got %s)", c.MaxStale)
	}
	if c.MaxStreams < 0 {
		return fmt.Errorf("max_streams must be a positive number (got %d)", c.MaxStreams)
	}
	if c.WriteThrough && c.StorageID == nil {
		return errors.New("write_through requires storage to be configured")
	}
	return nil
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxStale: 5 * time.Minute,

		// TODO: find good default
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31603
		MaxStreams: math.MaxInt,
	}
}

func (c Config) Metrics(tel telemetry.Metrics) {
	ctx := context.Background()
	tel.DeltatocumulativeStreamsMaxStale.Record(ctx, int64(c.MaxStale.Seconds()))
	tel.DeltatocumulativeStreamsLimit.Record(ctx, int64(c.MaxStreams))
}
