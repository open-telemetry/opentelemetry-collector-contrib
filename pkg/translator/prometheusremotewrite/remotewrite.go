package prometheusremotewrite

import "go.uber.org/zap"

type Settings struct {
	Namespace         string
	ExternalLabels    map[string]string
	DisableTargetInfo bool
	TimeThreshold     int64
	Logger            zap.Logger
}
