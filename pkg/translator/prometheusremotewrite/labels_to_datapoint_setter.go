package prometheusremotewrite

import (
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type LabelsToDatapointSetter interface {
	Attributes() pcommon.Map
}

// SetDataPointTimestamp sets the timestamp on a data point and checks if it's older than a specified threshold.
// It returns true if the data point is older than the threshold.
func addLabelsToDataPointV2[T LabelsToDatapointSetter](dp T, labelRefs []uint32, symbols []string) {
	for i := 0; i < len(labelRefs); i += 2 {
		labelName := symbols[labelRefs[i]]
		labelValue := symbols[labelRefs[i+1]]
		if labelName == nameLabel {
			labelName = "key_name"
		}
		dp.Attributes().PutStr(labelName, labelValue)
	}
}

func addLabelsToDataPoint[T LabelsToDatapointSetter](dataPoint T, labels []prompb.Label) {
	for _, label := range labels {
		labelName := label.Name
		if labelName == nameLabel {
			labelName = "key_name"
		}
		dataPoint.Attributes().PutStr(labelName, label.Value)
	}
}
