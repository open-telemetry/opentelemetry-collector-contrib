// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"

type (
	TypeName     string // How humans describe the MetricTypes ("counter", "gauge")
	ObserverType string // How the server will aggregate histogram and timings ("gauge", "summary")
)

const (
	CounterTypeName      TypeName = "counter"
	GaugeTypeName        TypeName = "gauge"
	HistogramTypeName    TypeName = "histogram"
	TimingTypeName       TypeName = "timing"
	TimingAltTypeName    TypeName = "timer"
	DistributionTypeName TypeName = "distribution"

	GaugeObserver     ObserverType = "gauge"
	SummaryObserver   ObserverType = "summary"
	HistogramObserver ObserverType = "histogram"
	DisableObserver   ObserverType = "disabled"

	DefaultObserverType = DisableObserver
)

type TimerHistogramMapping struct {
	StatsdType   TypeName        `mapstructure:"statsd_type"`
	ObserverType ObserverType    `mapstructure:"observer_type"`
	Histogram    HistogramConfig `mapstructure:"histogram"`
	Summary      SummaryConfig   `mapstructure:"summary"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type HistogramConfig struct {
	MaxSize         int32            `mapstructure:"max_size"`
	ExplicitBuckets []ExplicitBucket `mapstructure:"explicit_buckets"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type ExplicitBucket struct {
	_              struct{}
	MatcherPattern string    `mapstructure:"matcher_pattern"`
	Buckets        []float64 `mapstructure:"buckets"`
}

type SummaryConfig struct {
	Percentiles []float64 `mapstructure:"percentiles"`
	// prevent unkeyed literal initialization
	_ struct{}
}
