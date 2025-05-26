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
}

type HistogramConfig struct {
	MaxSize int32 `mapstructure:"max_size"`
}

type SummaryConfig struct {
	Percentiles []float64 `mapstructure:"percentiles"`
}
