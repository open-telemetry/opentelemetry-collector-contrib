package prometheusremotewrite

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"time"
)

// DatapointSetter defines an interface for setting and retrieving timestamps on data points.
type TimeDatapointSetter interface {
	SetTimestamp(v pcommon.Timestamp)
	Timestamp() pcommon.Timestamp
}

// SetDataPointTimestamp sets the timestamp on a data point and checks if it's older than a specified threshold.
// It returns true if the data point is older than the threshold.
func SetDataPointTimestamp[T TimeDatapointSetter](dp T, timestamp int64, settings PRWToMetricSettings) bool {
	// Convert the int64 timestamp to a time.Time object.
	convertedTime := time.Unix(0, timestamp*int64(time.Millisecond))

	// Set the timestamp on the data point.
	dp.SetTimestamp(pcommon.NewTimestampFromTime(convertedTime))

	// Check if the data point's timestamp is older than the threshold.
	// The threshold is defined in settings.TimeThreshold.
	if isOlderThanThreshold(dp.Timestamp().AsTime(), settings.TimeThreshold) {
		return true // The data point is older than the threshold.
	}
	return false // The data point is not older than the threshold.
}

func isOlderThanThreshold(timestamp time.Time, threshold int64) bool {
	return timestamp.Before(time.Now().Add(-time.Duration(threshold) * time.Hour))
}
