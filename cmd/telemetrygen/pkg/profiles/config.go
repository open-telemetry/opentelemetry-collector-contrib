// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/validate"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

// Config describes the test scenario.
type Config struct {
	config.Config
	NumProfiles     int
	SampleCount     int
	StackDepth      int
	UniqueFunctions int
	ProfileDuration time.Duration
	TraceID         string
	SpanID          string
}

func NewConfig() *Config {
	cfg := &Config{}
	cfg.SetDefaults()
	return cfg
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", c.HTTPPath, "Which URL path to write to")

	fs.IntVar(&c.NumProfiles, "profiles", c.NumProfiles, "Number of profiles to generate in each worker (ignored if duration is provided)")
	fs.IntVar(&c.SampleCount, "sample-count", c.SampleCount, "Number of samples per profile")
	fs.IntVar(&c.StackDepth, "stack-depth", c.StackDepth, "Maximum number of frames per stack")
	fs.IntVar(&c.UniqueFunctions, "unique-functions", c.UniqueFunctions, "Number of distinct functions in the profile dictionary")
	fs.DurationVar(&c.ProfileDuration, "profile-duration", c.ProfileDuration, "Duration represented by each profile (DurationNano)")
	fs.StringVar(&c.TraceID, "trace-id", c.TraceID, "TraceID for profile-to-trace correlation")
	fs.StringVar(&c.SpanID, "span-id", c.SpanID, "SpanID for profile-to-trace correlation")
}

// SetDefaults sets the default values for the configuration.
func (c *Config) SetDefaults() {
	c.Config.SetDefaults()
	c.HTTPPath = "/v1development/profiles"
	c.Rate = 1
	c.TotalDuration = types.DurationWithInf(0)
	c.SampleCount = 10
	c.StackDepth = 5
	c.UniqueFunctions = 20
	c.ProfileDuration = 10 * time.Second
	c.TraceID = ""
	c.SpanID = ""
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration.Duration() <= 0 && c.NumProfiles <= 0 && !c.TotalDuration.IsInf() {
		return errors.New("either `profiles` or `duration` must be greater than 0")
	}

	if c.SampleCount <= 0 {
		return errors.New("sample-count must be greater than 0")
	}

	if c.StackDepth <= 0 {
		return errors.New("stack-depth must be greater than 0")
	}

	if c.UniqueFunctions <= 0 {
		return errors.New("unique-functions must be greater than 0")
	}

	if c.ProfileDuration < 0 {
		return errors.New("profile-duration must be non-negative")
	}

	if c.LoadSize < 0 {
		return fmt.Errorf("load size must be non-negative, found %d", c.LoadSize)
	}

	if c.TraceID != "" {
		if err := validate.TraceID(c.TraceID); err != nil {
			return err
		}
	}

	if c.SpanID != "" {
		if err := validate.SpanID(c.SpanID); err != nil {
			return err
		}
	}

	return nil
}
