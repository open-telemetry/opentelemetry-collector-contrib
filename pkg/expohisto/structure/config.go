// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package structure // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/structure"

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping/exponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping/logarithm"
)

// DefaultMaxSize is the default maximum number of buckets per
// positive or negative number range.  The value 160 is specified by
// OpenTelemetry--yields a maximum relative error of less than 5% for
// data with contrast 10**5 (e.g., latencies in the range 1ms to 100s).
// See the derivation here:
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/sdk.md#exponential-bucket-histogram-aggregation
const DefaultMaxSize int32 = 160

// MinSize is the smallest reasonable configuration, which is small
// enough to contain the entire normal floating point range at
// MinScale.
const MinSize = 2

// MaximumMaxSize is an arbitrary limit meant to limit accidental use
// of giant histograms.
const MaximumMaxSize = 16384

// MaximumMaxScale is the largest scale supported by the mapping
// functions in this package.  Using this scale means the maximum
// number of buckets that can fit within the range of a signed 32-bit
// integer index could be used.
const MaximumMaxScale int32 = logarithm.MaxScale

// MinimumMaxScale is the smallest scale supported by the mapping
// functions in this package.  At this scale each bucket spans a factor
// of 2**1024, so the whole float64 range falls into a handful of
// buckets.
const MinimumMaxScale int32 = exponent.MinScale

// DefaultMaxScale is the default scale a histogram starts at.  Since
// histograms only ever downscale, starting at the maximum scale
// yields the highest resolution that fits within MaxSize.
const DefaultMaxScale int32 = MaximumMaxScale

// Config contains configuration for exponential histogram creation.
type Config struct {
	maxSize int32

	// maxScale is the scale a histogram starts at.  Because 0 is a
	// valid scale, maxScaleSet records whether it was set
	// explicitly.
	maxScale    int32
	maxScaleSet bool
}

// Option is the interface that applies a configuration option.
type Option interface {
	// apply sets the Option value of a config.
	apply(Config) Config
}

// WithMaxSize sets the maximum size of each range (positive and/or
// negative) in the histogram.
func WithMaxSize(size int32) Option {
	return maxSize(size)
}

// maxSize is an option to set the maximum histogram size.
type maxSize int32

// apply implements Option.
func (ms maxSize) apply(cfg Config) Config {
	cfg.maxSize = int32(ms)
	return cfg
}

// WithMaxScale sets the scale a histogram starts at, which bounds its
// resolution.  Histograms only ever downscale, therefore a value low
// enough that MaxSize can never be exceeded keeps the bucket
// boundaries fixed for the lifetime of the histogram.
func WithMaxScale(scale int32) Option {
	return maxScale(scale)
}

// maxScale is an option to set the maximum histogram scale.
type maxScale int32

// apply implements Option.
func (ms maxScale) apply(cfg Config) Config {
	cfg.maxScale = int32(ms)
	cfg.maxScaleSet = true
	return cfg
}

// NewConfig returns an exponential histogram configuration with
// defaults and limits applied.
func NewConfig(opts ...Option) Config {
	var cfg Config
	for _, opt := range opts {
		cfg = opt.apply(cfg)
	}
	return cfg
}

// Validate returns true for valid configurations.
func (c Config) Valid() bool {
	_, err := c.Validate()
	return err == nil
}

// Validate returns the nearest valid Config object to the input and an
// error describing each invalid field, or nil when the input was
// valid.
func (c Config) Validate() (Config, error) {
	var errs []error

	switch {
	case c.maxSize == 0:
		c.maxSize = DefaultMaxSize
	case c.maxSize >= MinSize && c.maxSize <= MaximumMaxSize:
		// Valid.
	default:
		errs = append(errs, fmt.Errorf("invalid histogram size: %d", c.maxSize))
		switch {
		case c.maxSize < 0:
			c.maxSize = DefaultMaxSize
		case c.maxSize < MinSize:
			c.maxSize = MinSize
		default:
			c.maxSize = MaximumMaxSize
		}
	}

	switch {
	case !c.maxScaleSet:
		c.maxScale = DefaultMaxScale
		c.maxScaleSet = true
	case c.maxScale >= MinimumMaxScale && c.maxScale <= MaximumMaxScale:
		// Valid.
	default:
		errs = append(errs, fmt.Errorf("invalid histogram max scale: %d", c.maxScale))
		if c.maxScale < MinimumMaxScale {
			c.maxScale = MinimumMaxScale
		} else {
			c.maxScale = MaximumMaxScale
		}
	}

	return c, errors.Join(errs...)
}
