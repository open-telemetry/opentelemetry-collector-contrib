// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package filesource

import (
	"time"
)

// Options holds configuration for the static sampling strategy store.
type Options struct {
	// StrategiesFile is the path for the sampling strategies file in JSON format
	StrategiesFile string
	// ReloadInterval is the time interval to check and reload sampling strategies file
	ReloadInterval time.Duration
	// Flag for enabling possibly breaking change which includes default operations level
	// strategies when calculating Ratelimiting type service level strategy
	// more information https://github.com/jaegertracing/jaeger/issues/5270
	IncludeDefaultOpStrategies bool
}
