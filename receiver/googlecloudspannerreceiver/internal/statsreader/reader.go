// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type ReaderConfig struct {
	TopMetricsQueryMaxRows            int
	BackfillEnabled                   bool
	HideTopnLockstatsRowrangestartkey bool
}

type Reader interface {
	Name() string
	Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error)
}

// CompositeReader - this interface is used for the composition of multiple Reader(s).
// Main difference between it and Reader - this interface also has Shutdown method for performing some additional
// cleanup necessary for each Reader instance.
type CompositeReader interface {
	Reader
	// Shutdown Use this method to perform any additional cleanup of underlying components.
	Shutdown()
}
