// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the Isolation Forest processor
type Config struct {
	// NumTrees specifies the number of isolation trees to build
	NumTrees int `mapstructure:"num_trees"`

	// SubsampleSize is the size of the dataset used to build each tree
	SubsampleSize int `mapstructure:"subsample_size"`

	// WindowSize defines the sliding window size for maintaining data points
	WindowSize int `mapstructure:"window_size"`

	// AnomalyThreshold is the threshold above which a point is considered anomalous
	// Range: [0.0, 1.0] where values closer to 1.0 are more anomalous
	AnomalyThreshold float64 `mapstructure:"anomaly_threshold"`

	// TrainingInterval specifies how often to retrain the model
	TrainingInterval time.Duration `mapstructure:"training_interval"`

	// MetricsToAnalyze specifies which metrics to apply anomaly detection to
	// If empty, all numeric metrics will be analyzed
	MetricsToAnalyze []string `mapstructure:"metrics_to_analyze"`

	// Features specifies which metric attributes to use as features
	// If empty, the metric value itself will be used
	Features []string `mapstructure:"features"`

	// AddAnomalyScore determines whether to add anomaly score as an attribute
	AddAnomalyScore bool `mapstructure:"add_anomaly_score"`

	// DropAnomalousMetrics determines whether to drop metrics identified as anomalous
	DropAnomalousMetrics bool `mapstructure:"drop_anomalous_metrics"`
}

// Validate checks the configuration for errors
func (cfg *Config) Validate() error {
	if cfg.NumTrees <= 0 {
		return errors.New("num_trees must be greater than 0")
	}

	if cfg.SubsampleSize <= 0 {
		return errors.New("subsample_size must be greater than 0")
	}

	if cfg.WindowSize <= 0 {
		return errors.New("window_size must be greater than 0")
	}

	if cfg.AnomalyThreshold < 0.0 || cfg.AnomalyThreshold > 1.0 {
		return errors.New("anomaly_threshold must be between 0.0 and 1.0")
	}

	if cfg.TrainingInterval <= 0 {
		return errors.New("training_interval must be greater than 0")
	}

	return nil
}

var _ component.Config = (*Config)(nil)
