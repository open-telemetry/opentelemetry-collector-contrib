package delta

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

func Aggregator() streams.Aggregator {
	return NewLock(NewSequencer(NewAccumulator()))
}
